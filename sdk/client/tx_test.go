package client

import (
	"context"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mocachain/moca/v2/sdk/client/test"
	"github.com/mocachain/moca/v2/sdk/keys"
	"github.com/mocachain/moca/v2/sdk/types"
)

// testFeeAmount is the fee for a tx that carries explicit gas info: DefaultGasLimit at DefaultGasPrice.
func testFeeAmount() sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(types.Denom, sdkmath.NewInt(types.DefaultGasLimit*types.DefaultGasPrice)))
}

// waitForTx blocks until the tx is in a block, so the next tx signs with an up-to-date account sequence.
func waitForTx(t *testing.T, cli *MocaClient, txHash string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := cli.GetTx(context.Background(), &tx.GetTxRequest{Hash: txHash})
		return err == nil
	}, 30*time.Second, 200*time.Millisecond, "tx %s not included in a block", txHash)
}

func TestSendTokenSucceedWithSimulatedGas(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	require.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	require.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 12)))
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, nil)
	require.NoError(t, err)
	require.Equal(t, uint32(0), response.TxResponse.Code, "raw log: %s", response.TxResponse.RawLog)
	t.Log(response.TxResponse.String())
	waitForTx(t, gnfdCli, response.TxResponse.TxHash)
}

func TestSendTokenWithTxOptionSucceed(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	require.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	require.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	require.NoError(t, err)
	mode := tx.BroadcastMode_BROADCAST_MODE_SYNC

	txOpt := &types.TxOption{
		Mode:       &mode,
		NoSimulate: true,
		GasLimit:   types.DefaultGasLimit,
		Memo:       "test",
		FeePayer:   payerAddr,
		FeeAmount:  testFeeAmount(),
	}
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	require.NoError(t, err)
	require.Equal(t, uint32(0), response.TxResponse.Code, "raw log: %s", response.TxResponse.RawLog)
	t.Log(response.TxResponse.String())
	waitForTx(t, gnfdCli, response.TxResponse.TxHash)
}

func TestErrorOutWhenGasInfoNotFullProvided(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	require.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	require.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	require.NoError(t, err)
	mode := tx.BroadcastMode_BROADCAST_MODE_SYNC
	txOpt := &types.TxOption{
		Mode:       &mode,
		NoSimulate: true,
		Memo:       "test",
		FeePayer:   payerAddr,
	}
	_, err = gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	assert.Equal(t, err, types.ErrGasInfoNotProvided)
}

func TestSimulatedGasPriceDefaultsWhenMinGasPriceEmpty(t *testing.T) {
	gasPrice, err := simulatedGasPrice("")
	require.NoError(t, err)
	assert.Equal(t, types.Denom, gasPrice.Denom)
	assert.Equal(t, sdkmath.NewInt(types.DefaultGasPrice), gasPrice.Amount)
}

func TestSimulatedGasPriceParsesMinGasPrice(t *testing.T) {
	gasPrice, err := simulatedGasPrice("123amoca")
	require.NoError(t, err)
	assert.Equal(t, types.Denom, gasPrice.Denom)
	assert.Equal(t, sdkmath.NewInt(123), gasPrice.Amount)
}

func TestSimulateTx(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	require.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	require.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	simulateRes, err := gnfdCli.SimulateTx(context.Background(), []sdk.Msg{transfer}, nil)
	require.NoError(t, err)
	t.Log(simulateRes.GasInfo.String())
}

func TestSendTokenWithCustomizedNonce(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	require.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	require.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	require.NoError(t, err)
	nonce, err := gnfdCli.GetNonce(context.Background())
	require.NoError(t, err)
	var response *tx.BroadcastTxResponse
	for i := 0; i < 50; i++ {
		txOpt := &types.TxOption{
			NoSimulate: true,
			GasLimit:   types.DefaultGasLimit,
			FeeAmount:  testFeeAmount(),
			Memo:       "test",
			FeePayer:   payerAddr,
			Nonce:      nonce,
		}
		response, err = gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
		require.NoError(t, err)
		nonce++
		require.Equal(t, uint32(0), response.TxResponse.Code, "raw log: %s", response.TxResponse.RawLog)
		t.Log(response.TxResponse.String())
	}
	waitForTx(t, gnfdCli, response.TxResponse.TxHash)
}

func TestSendTxWithGrpcConn(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km), WithGrpcConnectionAndDialOption(test.TestGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials())))
	require.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	require.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	require.NoError(t, err)
	nonce, err := gnfdCli.GetNonce(context.Background())
	require.NoError(t, err)
	txOpt := &types.TxOption{
		NoSimulate: true,
		GasLimit:   types.DefaultGasLimit,
		FeeAmount:  testFeeAmount(),
		Memo:       "test",
		FeePayer:   payerAddr,
		Nonce:      nonce,
	}
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	require.NoError(t, err)
	require.Equal(t, uint32(0), response.TxResponse.Code, "raw log: %s", response.TxResponse.RawLog)
	t.Log(response.TxResponse.String())
	waitForTx(t, gnfdCli, response.TxResponse.TxHash)
}

func TestSendTokenWithOverrideAccount(t *testing.T) {
	// which is not being used to send tx
	km, err := keys.NewPrivateKeyManager("2a3f0f19fbcb057e053696879207324c24f601ab47db92676cc4958ea9089761")
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	require.NoError(t, err)

	km2, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)

	require.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	require.NoError(t, err)
	transfer := banktypes.NewMsgSend(km2.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km2.GetAddr().String())
	require.NoError(t, err)
	mode := tx.BroadcastMode_BROADCAST_MODE_SYNC
	txOpt := &types.TxOption{
		Mode:               &mode,
		NoSimulate:         true,
		GasLimit:           types.DefaultGasLimit,
		Memo:               "test",
		FeePayer:           payerAddr,
		FeeAmount:          testFeeAmount(),
		OverrideKeyManager: &km2,
	}
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	require.NoError(t, err)
	require.Equal(t, uint32(0), response.TxResponse.Code, "raw log: %s", response.TxResponse.RawLog)
	t.Log(response.TxResponse.String())
	waitForTx(t, gnfdCli, response.TxResponse.TxHash)
}

func TestSendTXViaWebsocketClient(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	require.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	to := sdk.MustAccAddressFromHex(test.TestAddr)
	nonce, err := gnfdCli.GetNonce(ctx)
	require.NoError(t, err)
	var response *tx.BroadcastTxResponse
	for i := 0; i < 500; i++ {
		transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 12)))
		txOpt := &types.TxOption{NoSimulate: true, GasLimit: types.DefaultGasLimit, FeeAmount: testFeeAmount(), Nonce: nonce}
		response, err = gnfdCli.BroadcastTx(ctx, []sdk.Msg{transfer}, txOpt)
		require.NoError(t, err)
		nonce++
		require.Equal(t, uint32(0), response.TxResponse.Code, "raw log: %s", response.TxResponse.RawLog)
	}
	waitForTx(t, gnfdCli, response.TxResponse.TxHash)
}
