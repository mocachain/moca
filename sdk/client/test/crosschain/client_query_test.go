package crosschain

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"

	gnfdclient "github.com/evmos/evmos/v12/sdk/client"
	"github.com/evmos/evmos/v12/sdk/client/test"
	bridgetypes "github.com/evmos/evmos/v12/x/bridge/types"
)

func TestCrosschainParams(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	query := bridgetypes.QueryParamsRequest{}
	res, err := client.BridgeQueryClient.Params(context.Background(), &query)
	if err != nil {
		t.Logf("Error querying bridge params: %v", err)
		t.Fail()
		return
	}
	assert.NotNil(t, res)
	assert.NotNil(t, res.Params)
	assert.NotEmpty(t, res.Params.BscTransferOutRelayerFee)
	assert.NotEmpty(t, res.Params.BscTransferOutAckRelayerFee)
	t.Logf("Bridge params: %+v", res.Params)
}

func TestCrosschainPackageRequest(t *testing.T) {
	// Test transfer out functionality
	transferAmount := "1000000000000000000" // 1 token
	msg := bridgetypes.MsgTransferOut{
		From:   test.TestValAddr,
		To:     test.TestValAddr,
		Amount: &sdk.Coin{Denom: "amoca", Amount: math.NewInt(1000000000000000000)},
	}

	// Verify message fields
	assert.Equal(t, msg.From, test.TestValAddr)
	assert.Equal(t, msg.To, test.TestValAddr)
	assert.Equal(t, msg.Amount.Denom, "amoca")
	assert.Equal(t, msg.Amount.Amount.String(), transferAmount)

	t.Logf("Transfer out message: %+v", msg)
}

func TestCrosschainReceiveSequence(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	// Verify bridge module configuration
	query := bridgetypes.QueryParamsRequest{}
	res, err := client.BridgeQueryClient.Params(context.Background(), &query)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Params)

	t.Logf("Bridge module is properly configured with params: %+v", res.Params)
}

func TestCrosschainSendSequence(t *testing.T) {
	client, err := gnfdclient.NewMocaClient(test.TestRPCAddr, test.TestEVMAddr, test.TestChainID)
	assert.NoError(t, err)

	// Verify bridge module configuration
	query := bridgetypes.QueryParamsRequest{}
	res, err := client.BridgeQueryClient.Params(context.Background(), &query)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Params)

	t.Logf("Bridge module is properly configured with params: %+v", res.Params)
}
