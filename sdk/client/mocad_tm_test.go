package client

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/sdk/client/test"
	"github.com/mocachain/moca/v2/sdk/keys"
)

func TestTmClient(t *testing.T) {
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
	waitForTx(t, gnfdCli, response.TxResponse.TxHash)
	// get tx
	res, err := gnfdCli.Tx(context.Background(), response.TxResponse.TxHash)
	require.NoError(t, err)
	t.Log(res.Hash)
	t.Log(res.TxResult.String())

	// get the latest block
	block, err := gnfdCli.GetBlock(context.Background(), nil)
	require.NoError(t, err)
	t.Log(block)

	h := block.Block.Height

	block, err = gnfdCli.GetBlock(context.Background(), &h)
	require.NoError(t, err)
	t.Log(block)

	// get block result
	blockResult, err := gnfdCli.GetBlockResults(context.Background(), &h)
	require.NoError(t, err)
	t.Log(blockResult)

	// get validator
	validators, err := gnfdCli.GetValidators(context.Background(), &h)
	require.NoError(t, err)
	t.Log(validators.Validators)
}
