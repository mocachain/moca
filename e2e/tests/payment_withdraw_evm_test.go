package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/payment"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

// TestPaymentWithdrawImmediateEvmFlow drives x/payment's withdraw flow
// through the payment precompile's withdraw method for an amount under the
// module's WithdrawTimeLockThreshold (100 MOCA by default): the withdrawal
// settles as an immediate bank transfer and StaticBalance returns to zero
// straight away, exercising the same functional path as the retired suite's
// withdraw-under-threshold scenario.
func TestPaymentWithdrawImmediateEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	paymentClient := paymenttypes.NewQueryClient(conn)
	precompile := payment.Precompile{}

	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	fundMoca(t, ctx, client, chainID, userAddr, fundingAmountMOCA)

	depositAmount := mustBigInt(t, oneMocaInAmoca) // 1 MOCA, well under the 100 MOCA lock threshold

	depositMethod, err := payment.GetMethod(payment.DepositMethodName)
	require.NoError(t, err)
	depositArgs, err := depositMethod.Inputs.Pack(userAddr.String(), depositAmount)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, userKey, precompile.Address(),
		append(append([]byte{}, depositMethod.ID...), depositArgs...))

	require.Equal(t, depositAmount, getStreamRecord(t, ctx, paymentClient, userAddr.String()).StaticBalance.BigInt())

	withdrawMethod, err := payment.GetMethod(payment.WithdrawMethodName)
	require.NoError(t, err)
	withdrawArgs, err := withdrawMethod.Inputs.Pack(userAddr.String(), depositAmount)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, userKey, precompile.Address(),
		append(append([]byte{}, withdrawMethod.ID...), withdrawArgs...))

	require.True(t, getStreamRecord(t, ctx, paymentClient, userAddr.String()).StaticBalance.IsZero(),
		"a withdrawal under the time-lock threshold must settle immediately")
}

// TestPaymentWithdrawDelayedEvmFlow drives x/payment's withdraw flow for an
// amount at or above WithdrawTimeLockThreshold: StaticBalance still drops
// immediately, but the funds don't move -- a DelayedWithdrawalRecord is
// created instead, unlocking a day later, and claiming it early is rejected.
func TestPaymentWithdrawDelayedEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	paymentClient := paymenttypes.NewQueryClient(conn)
	precompile := payment.Precompile{}
	precompileAddr := precompile.Address()

	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	// 155 MOCA funded, 150 deposited: comfortably above the 100 MOCA lock
	// threshold, with headroom left over for this test's 3 transactions' gas.
	fundMoca(t, ctx, client, chainID, userAddr, 155)

	lockedAmount := new(big.Int).Mul(mustBigInt(t, oneMocaInAmoca), big.NewInt(150))

	depositMethod, err := payment.GetMethod(payment.DepositMethodName)
	require.NoError(t, err)
	depositArgs, err := depositMethod.Inputs.Pack(userAddr.String(), lockedAmount)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, userKey, precompileAddr,
		append(append([]byte{}, depositMethod.ID...), depositArgs...))

	withdrawMethod, err := payment.GetMethod(payment.WithdrawMethodName)
	require.NoError(t, err)
	withdrawArgs, err := withdrawMethod.Inputs.Pack(userAddr.String(), lockedAmount)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, userKey, precompileAddr,
		append(append([]byte{}, withdrawMethod.ID...), withdrawArgs...))

	require.True(t, getStreamRecord(t, ctx, paymentClient, userAddr.String()).StaticBalance.IsZero(),
		"StaticBalance is debited immediately even though the transfer itself is delayed")

	delayedResp, err := paymentClient.DelayedWithdrawal(ctx, &paymenttypes.QueryDelayedWithdrawalRequest{Account: userAddr.String()})
	require.NoError(t, err)
	require.Equal(t, lockedAmount, delayedResp.DelayedWithdrawal.Amount.BigInt())
	require.Equal(t, userAddr.String(), delayedResp.DelayedWithdrawal.From)

	// Claiming it before UnlockTimestamp must be rejected.
	claimArgs, err := withdrawMethod.Inputs.Pack("", lockedAmount)
	require.NoError(t, err)
	claimCalldata := append(append([]byte{}, withdrawMethod.ID...), claimArgs...)
	_, callErr := client.CallContract(ctx, ethereum.CallMsg{
		From: userAddr, To: &precompileAddr, Data: claimCalldata,
	}, nil)
	require.Error(t, callErr, "claiming a delayed withdrawal before its unlock timestamp must be rejected")
	require.Contains(t, callErr.Error(), "does not reach to the delayed duration")
}
