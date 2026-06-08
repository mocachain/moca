package keeper

import (
	"github.com/ethereum/go-ethereum/common"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/x/erc20/types"
)

func (k Keeper) SetToken(ctx sdk.Context, pair types.TokenPair) {
	k.SetTokenPair(ctx, pair)
	k.SetDenomMap(ctx, pair.Denom, pair.GetID())
	k.SetERC20Map(ctx, pair.GetERC20Contract(), pair.GetID())
}

func (k Keeper) GetTokenPairs(ctx sdk.Context) []types.TokenPair {
	tokenPairs := []types.TokenPair{}
	k.IterateTokenPairs(ctx, func(tokenPair types.TokenPair) (stop bool) {
		tokenPairs = append(tokenPairs, tokenPair)
		return false
	})
	return tokenPairs
}

func (k Keeper) IterateTokenPairs(ctx sdk.Context, cb func(tokenPair types.TokenPair) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	iterator := storetypes.KVStorePrefixIterator(store, types.KeyPrefixTokenPair)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var tokenPair types.TokenPair
		k.cdc.MustUnmarshal(iterator.Value(), &tokenPair)
		if cb(tokenPair) {
			break
		}
	}
}

func (k Keeper) GetTokenPairID(ctx sdk.Context, token string) []byte {
	if common.IsHexAddress(token) {
		return k.GetERC20Map(ctx, common.HexToAddress(token))
	}
	return k.GetDenomMap(ctx, token)
}

func (k Keeper) GetTokenPair(ctx sdk.Context, id []byte) (types.TokenPair, bool) {
	if id == nil {
		return types.TokenPair{}, false
	}

	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPair)
	var tokenPair types.TokenPair
	bz := store.Get(id)
	if len(bz) == 0 {
		return types.TokenPair{}, false
	}

	k.cdc.MustUnmarshal(bz, &tokenPair)
	return tokenPair, true
}

func (k Keeper) SetTokenPair(ctx sdk.Context, tokenPair types.TokenPair) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPair)
	bz := k.cdc.MustMarshal(&tokenPair)
	store.Set(tokenPair.GetID(), bz)
}

func (k Keeper) DeleteTokenPair(ctx sdk.Context, tokenPair types.TokenPair) {
	id := tokenPair.GetID()
	k.deleteTokenPair(ctx, id)
	k.deleteERC20Map(ctx, tokenPair.GetERC20Contract())
	k.deleteDenomMap(ctx, tokenPair.Denom)
	k.deleteAllowances(ctx, tokenPair.GetERC20Contract())
}

func (k Keeper) deleteTokenPair(ctx sdk.Context, id []byte) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPair)
	store.Delete(id)
}

func (k Keeper) GetERC20Map(ctx sdk.Context, erc20 common.Address) []byte {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByERC20)
	return store.Get(erc20.Bytes())
}

func (k Keeper) GetDenomMap(ctx sdk.Context, denom string) []byte {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByDenom)
	return store.Get([]byte(denom))
}

func (k Keeper) SetERC20Map(ctx sdk.Context, erc20 common.Address, id []byte) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByERC20)
	store.Set(erc20.Bytes(), id)
}

func (k Keeper) deleteERC20Map(ctx sdk.Context, erc20 common.Address) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByERC20)
	store.Delete(erc20.Bytes())
}

func (k Keeper) SetDenomMap(ctx sdk.Context, denom string, id []byte) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByDenom)
	store.Set([]byte(denom), id)
}

func (k Keeper) deleteDenomMap(ctx sdk.Context, denom string) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByDenom)
	store.Delete([]byte(denom))
}

func (k Keeper) IsERC20Registered(ctx sdk.Context, erc20 common.Address) bool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByERC20)
	return store.Has(erc20.Bytes())
}

func (k Keeper) IsDenomRegistered(ctx sdk.Context, denom string) bool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixTokenPairByDenom)
	return store.Has([]byte(denom))
}

func (k Keeper) GetTokenDenom(ctx sdk.Context, tokenAddress common.Address) (string, error) {
	tokenPairID := k.GetERC20Map(ctx, tokenAddress)
	if len(tokenPairID) == 0 {
		return "", errorsmod.Wrapf(types.ErrTokenPairNotFound, "token '%s' not registered", tokenAddress)
	}

	tokenPair, found := k.GetTokenPair(ctx, tokenPairID)
	if !found {
		return "", errorsmod.Wrapf(types.ErrTokenPairNotFound, "token '%s' not registered", tokenAddress)
	}

	return tokenPair.Denom, nil
}
