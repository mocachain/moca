package legacyevm

import (
	"context"
	"strconv"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// The EVM base fee query reads cosmos/evm's feemarket state. A height before
// v2.0.0 has none, so the query answers zero there. Such a block records the
// base fee it used in the BeginBlock event of the in-tree feemarket module,
// which this package reads instead.
const (
	baseFeeMethod          = "/cosmos.evm.vm.v1.Query/BaseFee"
	legacyFeeMarketEvent   = "fee_market"
	legacyBaseFeeAttribute = "base_fee"
)

// GRPCDialOptions returns the options for the node's own gRPC client that make
// the EVM base fee query answer pre-v2 heights. It returns none when clientCtx
// has no CometBFT client to read the block results from.
func GRPCDialOptions(clientCtx client.Context) []grpc.DialOption {
	client, ok := clientCtx.Client.(rpcclient.Client)
	if !ok {
		return nil
	}
	return []grpc.DialOption{grpc.WithChainUnaryInterceptor(BaseFeeInterceptor(client))}
}

// BaseFeeInterceptor fills in a zero base fee answered for a pre-v2 height from
// that block's fee_market event.
func BaseFeeInterceptor(client rpcclient.Client) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		if err := invoker(ctx, method, req, reply, cc, opts...); err != nil || method != baseFeeMethod {
			return err
		}
		res, ok := reply.(*evmtypes.QueryBaseFeeResponse)
		if !ok || !isZero(res.BaseFee) {
			return nil
		}
		if fee, ok := legacyBaseFee(ctx, client, heightFromOutgoing(ctx)); ok {
			res.BaseFee = &fee
		}
		return nil
	}
}

// ABCIQueryWithOptions does the same as BaseFeeInterceptor for queries that go
// through ABCI, which is the path when the node's gRPC server is disabled.
func (c cometClient) ABCIQueryWithOptions(
	ctx context.Context, path string, data bytes.HexBytes, opts rpcclient.ABCIQueryOptions,
) (*coretypes.ResultABCIQuery, error) {
	res, err := c.Client.ABCIQueryWithOptions(ctx, path, data, opts)
	if err != nil || res == nil || path != baseFeeMethod || res.Response.Code != abci.CodeTypeOK {
		return res, err
	}
	var reply evmtypes.QueryBaseFeeResponse
	if reply.Unmarshal(res.Response.Value) != nil || !isZero(reply.BaseFee) {
		return res, nil
	}
	height := opts.Height
	if height == 0 {
		height = res.Response.Height
	}
	fee, ok := legacyBaseFee(ctx, c.Client, height)
	if !ok {
		return res, nil
	}
	reply.BaseFee = &fee
	value, err := reply.Marshal()
	if err != nil {
		return res, nil
	}
	out := *res
	out.Response.Value = value
	return &out, nil
}

// legacyBaseFee returns the base fee the fee_market event of block height
// recorded, if the block has one.
func legacyBaseFee(ctx context.Context, client rpcclient.Client, height int64) (sdkmath.Int, bool) {
	if height <= 0 {
		return sdkmath.Int{}, false
	}
	res, err := client.BlockResults(ctx, &height)
	if err != nil || res == nil {
		return sdkmath.Int{}, false
	}
	for _, event := range res.FinalizeBlockEvents {
		if event.Type != legacyFeeMarketEvent {
			continue
		}
		for _, attr := range event.Attributes {
			if attr.Key != legacyBaseFeeAttribute {
				continue
			}
			if fee, ok := sdkmath.NewIntFromString(attr.Value); ok && !fee.IsNegative() {
				return fee, true
			}
		}
	}
	return sdkmath.Int{}, false
}

func heightFromOutgoing(ctx context.Context) int64 {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return 0
	}
	values := md.Get(grpctypes.GRPCBlockHeightHeader)
	if len(values) == 0 {
		return 0
	}
	height, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return 0
	}
	return height
}

func isZero(fee *sdkmath.Int) bool {
	return fee == nil || fee.IsNil() || fee.IsZero()
}
