package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/payment"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

// TestPaymentAccountEvmFlow drives x/payment's payment-account lifecycle
// through the payment precompile's createPaymentAccount and disableRefund
// methods, exercising the same functional path as the retired suite's
// TestCreatePaymentAccount over a real signed EVM transaction against a
// live node.
func TestPaymentAccountEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	paymentClient := paymenttypes.NewQueryClient(conn)
	precompile := payment.Precompile{}

	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	fundMoca(t, ctx, client, chainID, userAddr, fundingAmountMOCA)

	createMethod, err := payment.GetMethod(payment.CreatePaymentAccountMethodName)
	require.NoError(t, err)
	createCalldata := append(append([]byte{}, createMethod.ID...), []byte{}...)
	sendPrecompileTx(t, ctx, client, chainID, userKey, precompile.Address(), createCalldata)

	accountsResp, err := paymentClient.PaymentAccountsByOwner(ctx, &paymenttypes.QueryPaymentAccountsByOwnerRequest{
		Owner: userAddr.String(),
	})
	require.NoError(t, err)
	require.Len(t, accountsResp.PaymentAccounts, 1, "owner should have exactly one payment account")
	paymentAddr := accountsResp.PaymentAccounts[0]

	before, err := paymentClient.PaymentAccount(ctx, &paymenttypes.QueryPaymentAccountRequest{Addr: paymentAddr})
	require.NoError(t, err)
	require.Equal(t, userAddr.String(), before.PaymentAccount.Owner)
	require.True(t, before.PaymentAccount.Refundable, "new payment account should default to refundable")

	disableMethod, err := payment.GetMethod(payment.DisableRefundMethodName)
	require.NoError(t, err)
	disableArgs, err := disableMethod.Inputs.Pack(paymentAddr)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, userKey, precompile.Address(),
		append(append([]byte{}, disableMethod.ID...), disableArgs...))

	after, err := paymentClient.PaymentAccount(ctx, &paymenttypes.QueryPaymentAccountRequest{Addr: paymentAddr})
	require.NoError(t, err)
	require.False(t, after.PaymentAccount.Refundable, "disableRefund should persist")
}

// TestPaymentDepositEvmFlow drives x/payment's deposit flow through the
// payment precompile's deposit method: a funded user deposits straight into
// their own account address (no payment sub-account involved), and the
// deposited amount lands exactly in StaticBalance since nothing is
// streaming against it yet.
func TestPaymentDepositEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	paymentClient := paymenttypes.NewQueryClient(conn)
	precompile := payment.Precompile{}

	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	fundMoca(t, ctx, client, chainID, userAddr, fundingAmountMOCA)

	depositAmount := mustBigInt(t, oneMocaInAmoca) // 1 MOCA

	depositMethod, err := payment.GetMethod(payment.DepositMethodName)
	require.NoError(t, err)
	depositArgs, err := depositMethod.Inputs.Pack(userAddr.String(), depositAmount)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, userKey, precompile.Address(),
		append(append([]byte{}, depositMethod.ID...), depositArgs...))

	streamResp, err := paymentClient.StreamRecord(ctx, &paymenttypes.QueryGetStreamRecordRequest{
		Account: userAddr.String(),
	})
	require.NoError(t, err)
	require.True(t, streamResp.StreamRecord.NetflowRate.IsZero(), "no bucket/quota is attached, so nothing should be streaming")
	require.Equal(t, depositAmount, streamResp.StreamRecord.StaticBalance.BigInt())
}
