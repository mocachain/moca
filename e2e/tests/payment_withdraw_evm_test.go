package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mocachain/moca/v2/precompiles/payment"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

// paymentCalldata packs a payment-precompile call taking an address string
// and an amount: deposit(to, amount), or withdraw(from, amount) where from is
// the stream account to withdraw from, or "" to claim a matured delayed
// withdrawal.
func paymentCalldata(t *testing.T, methodName, account string, amount *big.Int) []byte {
	t.Helper()
	method, err := payment.GetMethod(methodName)
	require.NoError(t, err)
	args, err := method.Inputs.Pack(account, amount)
	require.NoError(t, err)
	return append(append([]byte{}, method.ID...), args...)
}

// TestPaymentWithdrawImmediateEvmFlow drives x/payment's withdraw flow
// through the payment precompile's withdraw method for an amount under the
// module's WithdrawTimeLockThreshold (100 MOCA by default), the retired
// suite's TestWithdraw: more than the user holds is refused and debits
// nothing, and a withdrawal within the stream settles at once -- StaticBalance
// returns to zero and the bank balance grows by exactly the amount, less the
// tx's own gas.
func TestPaymentWithdrawImmediateEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	paymentClient := paymenttypes.NewQueryClient(conn)
	precompile := payment.Precompile{}

	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	fundMoca(ctx, t, client, chainID, userAddr, fundingAmountMOCA)

	depositAmount := mustBigInt(t, oneMocaInAmoca) // 1 MOCA, well under the 100 MOCA lock threshold

	sendPrecompileTx(ctx, t, client, chainID, userKey, precompile.Address(),
		paymentCalldata(t, payment.DepositMethodName, userAddr.String(), depositAmount))
	require.Equal(t, depositAmount, getStreamRecord(ctx, t, paymentClient, userAddr.String()).StaticBalance.BigInt())

	// More than the user holds in total is refused, and refused means untouched.
	// An overdraw the bank balance could cover would instead be topped up from
	// it: x/payment self-deposits the shortfall for a stream account that has a
	// bank account, so only an amount beyond everything the user has is refused.
	overdraw := new(big.Int).Add(depositAmount, new(big.Int).Mul(nativeBalance(ctx, t, client, userAddr), big.NewInt(2)))
	reason := sendPrecompileTxExpectRevert(ctx, t, client, chainID, userKey, precompile.Address(),
		paymentCalldata(t, payment.WithdrawMethodName, userAddr.String(), overdraw))
	require.Contains(t, reason, "balance not enough")
	require.Equal(t, depositAmount, getStreamRecord(ctx, t, paymentClient, userAddr.String()).StaticBalance.BigInt())

	bankBefore := nativeBalance(ctx, t, client, userAddr)
	receipt := sendPrecompileTxReceipt(ctx, t, client, chainID, userKey, precompile.Address(),
		paymentCalldata(t, payment.WithdrawMethodName, userAddr.String(), depositAmount))

	require.True(t, getStreamRecord(ctx, t, paymentClient, userAddr.String()).StaticBalance.IsZero(),
		"a withdrawal under the time-lock threshold must settle immediately")
	expected := new(big.Int).Sub(new(big.Int).Add(bankBefore, depositAmount), txGasCost(t, receipt))
	require.Equal(t, expected.String(), nativeBalance(ctx, t, client, userAddr).String(),
		"the withdrawn amount must land in the bank balance")
}

// TestPaymentWithdrawDelayedEvmFlow drives x/payment's withdraw flow for an
// amount at or above WithdrawTimeLockThreshold, the retired suite's
// TestWithdrawDelayed end to end: StaticBalance drops at once but the bank
// balance only loses the gas, a DelayedWithdrawalRecord unlocking
// WithdrawTimeLockDuration later is created instead, claiming it early and
// queueing a second large withdrawal behind it are both refused, and once
// block time passes the unlock the claim pays out exactly the locked amount
// (less gas) without touching the stream. localup pins the lock to 10s so
// the unlock can be waited out.
func TestPaymentWithdrawDelayedEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	paymentClient := paymenttypes.NewQueryClient(conn)
	precompile := payment.Precompile{}
	precompileAddr := precompile.Address()

	paramsResp, err := paymentClient.Params(ctx, &paymenttypes.QueryParamsRequest{})
	require.NoError(t, err)
	params := paramsResp.Params
	require.NotNil(t, params.WithdrawTimeLockThreshold)
	require.LessOrEqual(t, params.WithdrawTimeLockDuration, uint64(60),
		"this test waits the lock out; localup pins WithdrawTimeLockDuration to 10s")

	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	// 1.5x the threshold gets locked, and deposited a second time to back the
	// refused duplicate, plus headroom for this test's gas.
	thresholdMoca := new(big.Int).Div(params.WithdrawTimeLockThreshold.BigInt(), mustBigInt(t, oneMocaInAmoca)).Int64()
	lockedMoca := thresholdMoca * 3 / 2
	fundMoca(ctx, t, client, chainID, userAddr, 2*lockedMoca+fundingAmountMOCA)
	lockedAmount := new(big.Int).Mul(mustBigInt(t, oneMocaInAmoca), big.NewInt(lockedMoca))

	streamBalance := func() *big.Int {
		return getStreamRecord(ctx, t, paymentClient, userAddr.String()).StaticBalance.BigInt()
	}
	deposit := paymentCalldata(t, payment.DepositMethodName, userAddr.String(), lockedAmount)
	withdraw := paymentCalldata(t, payment.WithdrawMethodName, userAddr.String(), lockedAmount)
	claim := paymentCalldata(t, payment.WithdrawMethodName, "", lockedAmount)

	sendPrecompileTx(ctx, t, client, chainID, userKey, precompileAddr, deposit)
	require.Equal(t, lockedAmount, streamBalance())

	bankBefore := nativeBalance(ctx, t, client, userAddr)
	withdrawReceipt := sendPrecompileTxReceipt(ctx, t, client, chainID, userKey, precompileAddr, withdraw)
	require.True(t, streamBalance().Sign() == 0,
		"StaticBalance is debited immediately even though the transfer itself is delayed")
	require.Equal(t, new(big.Int).Sub(bankBefore, txGasCost(t, withdrawReceipt)).String(), nativeBalance(ctx, t, client, userAddr).String(),
		"nothing but gas may leave the bank balance while the withdrawal is locked")

	delayedResp, err := paymentClient.DelayedWithdrawal(ctx, &paymenttypes.QueryDelayedWithdrawalRequest{Account: userAddr.String()})
	require.NoError(t, err)
	delayed := delayedResp.DelayedWithdrawal
	require.Equal(t, lockedAmount, delayed.Amount.BigInt())
	require.Equal(t, userAddr.String(), delayed.From)
	withdrawHeader, err := client.HeaderByNumber(ctx, withdrawReceipt.BlockNumber)
	require.NoError(t, err)
	require.Equal(t, int64(withdrawHeader.Time)+int64(params.WithdrawTimeLockDuration), delayed.UnlockTimestamp, //nolint:gosec // block time and a 10s lock are far below MaxInt64
		"the record must unlock WithdrawTimeLockDuration after the withdrawing block")

	// Claiming before the unlock is refused.
	reason := sendPrecompileTxExpectRevert(ctx, t, client, chainID, userKey, precompileAddr, claim)
	require.Contains(t, reason, "does not reach to the delayed duration")

	// So is queueing another large withdrawal behind it, even when the stream
	// can cover it -- and a refused withdrawal debits nothing.
	sendPrecompileTx(ctx, t, client, chainID, userKey, precompileAddr, deposit)
	require.Equal(t, lockedAmount, streamBalance())
	reason = sendPrecompileTxExpectRevert(ctx, t, client, chainID, userKey, precompileAddr, withdraw)
	require.Contains(t, reason, "delayed withdrawal already exists")
	require.Equal(t, lockedAmount, streamBalance())

	// Once block time passes the unlock, the claim pays the locked amount out
	// of the module, leaving the stream alone.
	waitForBlockTime(ctx, t, client, delayed.UnlockTimestamp)
	bankBefore = nativeBalance(ctx, t, client, userAddr)
	claimReceipt := sendPrecompileTxReceipt(ctx, t, client, chainID, userKey, precompileAddr, claim)
	expected := new(big.Int).Sub(new(big.Int).Add(bankBefore, lockedAmount), txGasCost(t, claimReceipt))
	require.Equal(t, expected.String(), nativeBalance(ctx, t, client, userAddr).String(),
		"the unlocked funds must be paid out in full")
	require.Equal(t, lockedAmount, streamBalance(), "claiming the delayed withdrawal must not touch the stream")
	_, err = paymentClient.DelayedWithdrawal(ctx, &paymenttypes.QueryDelayedWithdrawalRequest{Account: userAddr.String()})
	require.Equal(t, codes.NotFound, status.Code(err), "the claimed record must be gone")
}
