package legacyevm_test

import (
	"context"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/server/legacyevm"
)

const baseFeeMethod = "/cosmos.evm.vm.v1.Query/BaseFee"

// blockClient serves block results with the given finalize-block events per height
// and answers the base fee query over ABCI with baseFee.
type blockClient struct {
	rpcclient.Client
	events  map[int64][]abci.Event
	baseFee sdkmath.Int
}

func (c blockClient) BlockResults(_ context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	return &coretypes.ResultBlockResults{Height: *height, FinalizeBlockEvents: c.events[*height]}, nil
}

func (c blockClient) ABCIQueryWithOptions(
	_ context.Context, _ string, _ bytes.HexBytes, opts rpcclient.ABCIQueryOptions,
) (*coretypes.ResultABCIQuery, error) {
	fee := c.baseFee
	value, err := (&evmtypes.QueryBaseFeeResponse{BaseFee: &fee}).Marshal()
	if err != nil {
		return nil, err
	}
	return &coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Value: value, Height: opts.Height}}, nil
}

func feeMarketEvent(name, baseFee string) abci.Event {
	return abci.Event{Type: name, Attributes: []abci.EventAttribute{
		{Key: "base_fee", Value: baseFee},
		{Key: "mode", Value: `"BeginBlock"`},
	}}
}

func pre2Client() blockClient {
	return blockClient{events: map[int64][]abci.Event{
		300:  {feeMarketEvent("fee_market", "7")},
		5000: {feeMarketEvent("evm_fee_market", "20000000000")},
	}, baseFee: sdkmath.ZeroInt()}
}

func callBaseFee(t *testing.T, client rpcclient.Client, method string, height int64, answer sdkmath.Int) *evmtypes.QueryBaseFeeResponse {
	t.Helper()
	interceptor := legacyevm.BaseFeeInterceptor(client)
	ctx := metadata.AppendToOutgoingContext(context.Background(), grpctypes.GRPCBlockHeightHeader, sdkmath.NewInt(height).String())
	reply := &evmtypes.QueryBaseFeeResponse{}
	invoker := func(_ context.Context, _ string, _, reply any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		fee := answer
		reply.(*evmtypes.QueryBaseFeeResponse).BaseFee = &fee
		return nil
	}
	require.NoError(t, interceptor(ctx, method, &evmtypes.QueryBaseFeeRequest{}, reply, nil, invoker))
	return reply
}

func TestBaseFeeInterceptor(t *testing.T) {
	client := pre2Client()

	// A pre-v2 height answered with zero takes the fee its fee_market event recorded.
	require.Equal(t, "7", callBaseFee(t, client, baseFeeMethod, 300, sdkmath.ZeroInt()).BaseFee.String())
	// A height without the old event keeps the answer.
	require.True(t, callBaseFee(t, client, baseFeeMethod, 5000, sdkmath.ZeroInt()).BaseFee.IsZero())
	// A non-zero answer is never replaced.
	require.Equal(t, "20000000000", callBaseFee(t, client, baseFeeMethod, 300, sdkmath.NewInt(20_000_000_000)).BaseFee.String())
	// Other methods are left alone.
	require.True(t, callBaseFee(t, client, "/cosmos.evm.vm.v1.Query/Params", 300, sdkmath.ZeroInt()).BaseFee.IsZero())
}

func TestBaseFeeOverABCI(t *testing.T) {
	clientCtx := legacyevm.WrapClientContext(client.Context{}.WithClient(pre2Client()))

	query := func(path string, height int64) *evmtypes.QueryBaseFeeResponse {
		res, err := clientCtx.Client.ABCIQueryWithOptions(context.Background(), path, nil, rpcclient.ABCIQueryOptions{Height: height})
		require.NoError(t, err)
		var reply evmtypes.QueryBaseFeeResponse
		require.NoError(t, reply.Unmarshal(res.Response.Value))
		return &reply
	}
	require.Equal(t, "7", query(baseFeeMethod, 300).BaseFee.String())
	require.True(t, query(baseFeeMethod, 5000).BaseFee.IsZero())
	require.True(t, query("/cosmos.evm.vm.v1.Query/Params", 300).BaseFee.IsZero())
}

func TestGRPCDialOptions_NeedCometClient(t *testing.T) {
	require.Empty(t, legacyevm.GRPCDialOptions(client.Context{}))
	require.Len(t, legacyevm.GRPCDialOptions(client.Context{}.WithClient(pre2Client())), 1)
}
