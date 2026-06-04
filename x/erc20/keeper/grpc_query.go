package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/mocachain/moca/v2/x/erc20/types"
)

var _ types.QueryServer = Keeper{}

func (k Keeper) TokenPairs(goCtx context.Context, req *types.QueryTokenPairsRequest) (*types.QueryTokenPairsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPair)

	var tokenPairs []types.TokenPair
	pageRes, err := query.Paginate(store, req.Pagination, func(_, value []byte) error {
		var tokenPair types.TokenPair
		k.cdc.MustUnmarshal(value, &tokenPair)
		tokenPairs = append(tokenPairs, tokenPair)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryTokenPairsResponse{TokenPairs: tokenPairs, Pagination: pageRes}, nil
}

func (k Keeper) TokenPair(goCtx context.Context, req *types.QueryTokenPairRequest) (*types.QueryTokenPairResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	id := k.GetTokenPairID(ctx, req.Token)
	tokenPair, found := k.GetTokenPair(ctx, id)
	if !found {
		return nil, types.ErrTokenPairNotFound
	}
	return &types.QueryTokenPairResponse{TokenPair: tokenPair}, nil
}

func (k Keeper) Params(goCtx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return &types.QueryParamsResponse{Params: k.GetParams(ctx)}, nil
}
