package client

import (
	"context"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"

	"github.com/evmos/evmos/v12/sdk/client/test"
	"github.com/evmos/evmos/v12/sdk/keys"
	"github.com/evmos/evmos/v12/sdk/types"
)

func TestTmClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	km, err := keys.NewPrivateKeyManager(test.TestPrivateKey)
	assert.NoError(t, err)
	addr := km.GetAddr()
	assert.Equal(t, test.TestAddr, addr.String(), "Key manager address mismatch")

	gnfdCli, err := NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID, WithKeyManager(km), WithWebSocketClient())
	assert.NoError(t, err)

	nonce, err := gnfdCli.GetNonce(ctx)
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

	response, err := gnfdCli.BroadcastTx(ctx, []sdk.Msg{transfer}, txOpt)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.TxResponse)

	time.Sleep(5 * time.Second)

	var res *coretypes.ResultTx
	for i := 0; i < 3; i++ {
		res, err = gnfdCli.Tx(ctx, response.TxResponse.TxHash)
		if err == nil && res != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	assert.NotNil(t, res)

	block, err := gnfdCli.GetBlock(ctx, nil)
	assert.NoError(t, err)
	t.Log(block)

	h := block.Block.Height

	block, err = gnfdCli.GetBlock(ctx, &h)
	assert.NoError(t, err)
	t.Log(block)

	var blockResult *coretypes.ResultBlockResults
	for i := 0; i < 3; i++ {
		blockResult, err = gnfdCli.GetBlockResults(ctx, &h)
		if err == nil && blockResult != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	t.Log(blockResult)

	var validators *coretypes.ResultValidators
	for i := 0; i < 3; i++ {
		validators, err = gnfdCli.GetValidators(ctx, &h)
		if err == nil && validators != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	t.Log(validators.Validators)
}
