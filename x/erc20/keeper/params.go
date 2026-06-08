package keeper

import (
	"slices"

	"github.com/mocachain/moca/v2/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var isTrue = []byte("0x01")

const addressLength = 42

func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	enableErc20 := k.IsERC20Enabled(ctx)
	dynamicPrecompiles := k.getDynamicPrecompiles(ctx)
	nativePrecompiles := k.getNativePrecompiles(ctx)
	permissionlessRegistration := k.isPermissionlessRegistration(ctx)
	return types.NewParams(enableErc20, nativePrecompiles, dynamicPrecompiles, permissionlessRegistration)
}

func (k Keeper) SetParams(ctx sdk.Context, newParams types.Params) error {
	slices.Sort(newParams.DynamicPrecompiles)
	slices.Sort(newParams.NativePrecompiles)

	if err := newParams.Validate(); err != nil {
		return err
	}

	k.setERC20Enabled(ctx, newParams.EnableErc20)
	k.setDynamicPrecompiles(ctx, newParams.DynamicPrecompiles)
	k.setNativePrecompiles(ctx, newParams.NativePrecompiles)
	k.SetPermissionlessRegistration(ctx, newParams.PermissionlessRegistration)
	return nil
}

func (k Keeper) IsERC20Enabled(ctx sdk.Context) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.ParamStoreKeyEnableErc20)
}

func (k Keeper) setERC20Enabled(ctx sdk.Context, enable bool) {
	store := ctx.KVStore(k.storeKey)
	if enable {
		store.Set(types.ParamStoreKeyEnableErc20, isTrue)
		return
	}
	store.Delete(types.ParamStoreKeyEnableErc20)
}

func (k Keeper) setDynamicPrecompiles(ctx sdk.Context, dynamicPrecompiles []string) {
	store := ctx.KVStore(k.storeKey)
	bz := make([]byte, 0, addressLength*len(dynamicPrecompiles))
	for _, str := range dynamicPrecompiles {
		bz = append(bz, []byte(str)...)
	}
	store.Set(types.ParamStoreKeyDynamicPrecompiles, bz)
}

func (k Keeper) getDynamicPrecompiles(ctx sdk.Context) (dynamicPrecompiles []string) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ParamStoreKeyDynamicPrecompiles)

	for i := 0; i < len(bz); i += addressLength {
		dynamicPrecompiles = append(dynamicPrecompiles, string(bz[i:i+addressLength]))
	}
	return dynamicPrecompiles
}

func (k Keeper) setNativePrecompiles(ctx sdk.Context, nativePrecompiles []string) {
	store := ctx.KVStore(k.storeKey)
	bz := make([]byte, 0, addressLength*len(nativePrecompiles))
	for _, str := range nativePrecompiles {
		bz = append(bz, []byte(str)...)
	}
	store.Set(types.ParamStoreKeyNativePrecompiles, bz)
}

func (k Keeper) getNativePrecompiles(ctx sdk.Context) (nativePrecompiles []string) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ParamStoreKeyNativePrecompiles)
	for i := 0; i < len(bz); i += addressLength {
		nativePrecompiles = append(nativePrecompiles, string(bz[i:i+addressLength]))
	}
	return nativePrecompiles
}

func (k Keeper) isPermissionlessRegistration(ctx sdk.Context) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.ParamStoreKeyPermissionlessRegistration)
}

func (k Keeper) SetPermissionlessRegistration(ctx sdk.Context, permissionlessRegistration bool) {
	store := ctx.KVStore(k.storeKey)
	if permissionlessRegistration {
		store.Set(types.ParamStoreKeyPermissionlessRegistration, isTrue)
		return
	}
	store.Delete(types.ParamStoreKeyPermissionlessRegistration)
}
