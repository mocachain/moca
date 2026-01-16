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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/evmos/evmos/v12/sdk/client/test"
	"github.com/evmos/evmos/v12/sdk/keys"
	"github.com/evmos/evmos/v12/sdk/types"
)

func TestSendTokenSucceedWithSimulatedGas(t *testing.T) {
	// Create key manager and verify address
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	assert.NoError(t, err)

	nonce, err := gnfdCli.GetNonce(context.Background())
	assert.NoError(t, err)

	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	assert.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 12)))

	txOpt := &types.TxOption{
		GasLimit:  50000,
		Nonce:     nonce,
		FeeAmount: sdk.NewCoins(sdk.NewCoin(test.TestTokenName, sdkmath.NewInt(300000000000000))),
		FeePayer:  addr,
	}

	// Send transaction and verify result
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	if err != nil {
		t.Logf("Transaction failed with error: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, response)
	assert.NotNil(t, response.TxResponse)
	if response.TxResponse.Code != 0 {
		t.Logf("Transaction failed with code %d: %s", response.TxResponse.Code, response.TxResponse.RawLog)
		t.Fail()
		return
	}
	t.Log(response.TxResponse.String())
}

func TestSendTokenWithTxOptionSucceed(t *testing.T) {
	// Wait for previous transactions to be confirmed
	time.Sleep(5 * time.Second)

	// Create key manager and verify address
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	// Create client
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	assert.NoError(t, err)

	// Get current nonce
	nonce, err := gnfdCli.GetNonce(context.Background())
	assert.NoError(t, err)

	// Set transaction parameters
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	assert.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	assert.NoError(t, err)

	// Set broadcast mode
	mode := tx.BroadcastMode_BROADCAST_MODE_SYNC

	// Set sufficient gas and fee
	gasLimit := uint64(50000)
	gasPrice := sdkmath.NewInt(6000000000) // 6 Gwei
	feeAmt := sdk.NewCoins(sdk.NewCoin(test.TestTokenName, gasPrice.Mul(sdkmath.NewInt(int64(gasLimit)))))

	// Set transaction options
	txOpt := &types.TxOption{
		Mode:       &mode,
		NoSimulate: true,
		GasLimit:   gasLimit,
		Memo:       "test",
		FeePayer:   payerAddr,
		FeeAmount:  feeAmt,
		Nonce:      nonce,
	}

	// Send transaction and verify result
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	if err != nil {
		t.Logf("Transaction failed with error: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, response)
	assert.NotNil(t, response.TxResponse)
	if response.TxResponse.Code != 0 {
		t.Logf("Transaction failed with code %d: %s", response.TxResponse.Code, response.TxResponse.RawLog)
		t.Fail()
		return
	}
	t.Log(response.TxResponse.String())
}

func TestErrorOutWhenGasInfoNotFullProvided(t *testing.T) {
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	assert.NoError(t, err)
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	assert.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	assert.NoError(t, err)
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

func TestSimulateTx(t *testing.T) {
	// Wait for previous transactions to be confirmed
	time.Sleep(5 * time.Second)

	// Create key manager and verify address
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	// Create client
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	assert.NoError(t, err)

	// Get current nonce
	nonce, err := gnfdCli.GetNonce(context.Background())
	assert.NoError(t, err)

	// Set transaction parameters
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	assert.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))

	// Set sufficient gas and fee
	gasLimit := uint64(50000)
	gasPrice := sdkmath.NewInt(6000000000) // 6 Gwei
	feeAmt := sdk.NewCoins(sdk.NewCoin(test.TestTokenName, gasPrice.Mul(sdkmath.NewInt(int64(gasLimit)))))

	// Set transaction options
	txOpt := &types.TxOption{
		GasLimit:  gasLimit,
		Nonce:     nonce,
		FeeAmount: feeAmt,
		FeePayer:  addr,
	}

	// Simulate transaction
	simulateRes, err := gnfdCli.SimulateTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	if err != nil {
		t.Logf("Transaction simulation failed with error: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, simulateRes)
	assert.NotNil(t, simulateRes.GasInfo)
	t.Log(simulateRes.GasInfo.String())
}

func TestSendTokenWithCustomizedNonce(t *testing.T) {
	// Create key manager and verify address
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	// Create client
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	assert.NoError(t, err)

	// Get current nonce
	nonce, err := gnfdCli.GetNonce(context.Background())
	assert.NoError(t, err)

	// Set transaction parameters
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	assert.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	assert.NoError(t, err)

	// Set sufficient gas and fee
	gasLimit := uint64(50000)
	gasPrice := sdkmath.NewInt(6000000000) // 6 Gwei
	feeAmt := sdk.NewCoins(sdk.NewCoin(test.TestTokenName, gasPrice.Mul(sdkmath.NewInt(int64(gasLimit)))))

	// Send only one transaction to avoid affecting other tests
	txOpt := &types.TxOption{
		GasLimit:  gasLimit,
		Memo:      "test",
		FeePayer:  payerAddr,
		FeeAmount: feeAmt,
		Nonce:     nonce,
	}

	// Send transaction and verify result
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	if err != nil {
		t.Logf("Transaction failed with error: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, response)
	assert.NotNil(t, response.TxResponse)
	if response.TxResponse.Code != 0 {
		t.Logf("Transaction failed with code %d: %s", response.TxResponse.Code, response.TxResponse.RawLog)
		t.Fail()
		return
	}
	t.Log(response.TxResponse.String())
}

func TestZZZ1_SendTxWithGrpcConn(t *testing.T) {
	// Wait for previous transactions to be confirmed
	time.Sleep(5 * time.Second)
	// Create key manager and verify address
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	// Create client with GRPC connection
	gnfdCli, err := NewMocaClient(
		test.TestRPCAddr,
		test.TestEVMAddr,
		test.TestChainID,
		WithKeyManager(km),
		WithGrpcConnectionAndDialOption(test.TestGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	assert.NoError(t, err)

	// Get current nonce
	nonce, err := gnfdCli.GetNonce(context.Background())
	assert.NoError(t, err)

	// Set transaction parameters
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	assert.NoError(t, err)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km.GetAddr().String())
	assert.NoError(t, err)

	// Set sufficient gas and fee
	gasLimit := uint64(50000)
	gasPrice := sdkmath.NewInt(6000000000) // 6 Gwei
	feeAmt := sdk.NewCoins(sdk.NewCoin(test.TestTokenName, gasPrice.Mul(sdkmath.NewInt(int64(gasLimit)))))

	// Set transaction options
	txOpt := &types.TxOption{
		GasLimit:  gasLimit,
		Memo:      "test",
		FeePayer:  payerAddr,
		Nonce:     nonce,
		FeeAmount: feeAmt,
	}

	// Send transaction and verify result
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	if err != nil {
		t.Logf("Transaction failed with error: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, response)
	assert.NotNil(t, response.TxResponse)
	if response.TxResponse.Code != 0 {
		t.Logf("Transaction failed with code %d: %s", response.TxResponse.Code, response.TxResponse.RawLog)
		t.Fail()
		return
	}
	t.Log(response.TxResponse.String())
}

func TestZZZ2_SendTokenWithOverrideAccount(t *testing.T) {
	// Wait for previous transactions to be confirmed
	time.Sleep(5 * time.Second)
	time.Sleep(5 * time.Second)

	// Create first key manager (not used for sending tx)
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	// Create client with first key manager
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km))
	assert.NoError(t, err)

	// Create second key manager (used for sending tx)
	km2, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr2 := km2.GetAddr()
	assert.Equal(t, test.TestAddr, addr2.String(), "Key manager address mismatch")

	// Get current nonce
	nonce, err := gnfdCli.GetNonce(context.Background())
	assert.NoError(t, err)

	// Set transaction parameters
	to, err := sdk.AccAddressFromHexUnsafe(test.TestAddr)
	assert.NoError(t, err)
	transfer := banktypes.NewMsgSend(km2.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 100)))
	payerAddr, err := sdk.AccAddressFromHexUnsafe(km2.GetAddr().String())
	assert.NoError(t, err)

	// Set broadcast mode
	mode := tx.BroadcastMode_BROADCAST_MODE_SYNC

	// Set sufficient gas and fee
	gasLimit := uint64(50000)
	gasPrice := sdkmath.NewInt(6000000000) // 6 Gwei
	feeAmt := sdk.NewCoins(sdk.NewCoin(test.TestTokenName, gasPrice.Mul(sdkmath.NewInt(int64(gasLimit)))))

	// Set transaction options
	txOpt := &types.TxOption{
		Mode:               &mode,
		NoSimulate:         true,
		GasLimit:           gasLimit,
		Memo:               "test",
		FeePayer:           payerAddr,
		FeeAmount:          feeAmt,
		Nonce:              nonce,
		OverrideKeyManager: &km2,
	}

	// Send transaction and verify result
	response, err := gnfdCli.BroadcastTx(context.Background(), []sdk.Msg{transfer}, txOpt)
	if err != nil {
		t.Logf("Transaction failed with error: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, response)
	assert.NotNil(t, response.TxResponse)
	if response.TxResponse.Code != 0 {
		t.Logf("Transaction failed with code %d: %s", response.TxResponse.Code, response.TxResponse.RawLog)
		t.Fail()
		return
	}
	t.Log(response.TxResponse.String())
}

func TestZZZ3_SendTXViaWebsocketClient(t *testing.T) {
	// Wait for previous transactions to be confirmed
	time.Sleep(5 * time.Second)
	// Create key manager and verify address
	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	// Create client with WebSocket
	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km), WithWebSocketClient())
	assert.NoError(t, err)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current nonce
	nonce, err := gnfdCli.GetNonce(ctx)
	assert.NoError(t, err)

	// Set transaction parameters
	to := sdk.MustAccAddressFromHex(test.TestAddr)
	transfer := banktypes.NewMsgSend(km.GetAddr(), to, sdk.NewCoins(sdk.NewInt64Coin(test.TestTokenName, 12)))

	// Set sufficient gas and fee
	gasLimit := uint64(50000)
	gasPrice := sdkmath.NewInt(6000000000) // 6 Gwei
	feeAmt := sdk.NewCoins(sdk.NewCoin(test.TestTokenName, gasPrice.Mul(sdkmath.NewInt(int64(gasLimit)))))

	// Send only one transaction to avoid affecting other tests
	txOpt := &types.TxOption{
		GasLimit:  gasLimit,
		Nonce:     nonce,
		FeeAmount: feeAmt,
		FeePayer:  addr,
	}

	// Send transaction and verify result
	response, err := gnfdCli.BroadcastTx(ctx, []sdk.Msg{transfer}, txOpt)
	if err != nil {
		t.Logf("Transaction failed with error: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, response)
	assert.NotNil(t, response.TxResponse)
	if response.TxResponse.Code != 0 {
		t.Logf("Transaction failed with code %d: %s", response.TxResponse.Code, response.TxResponse.RawLog)
		t.Fail()
		return
	}
	t.Log(response.TxResponse.String())
}
