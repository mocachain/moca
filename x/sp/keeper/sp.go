package keeper

import (
	"encoding/binary"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/x/sp/types"
)

func (k Keeper) GetStorageProvider(ctx sdk.Context, id uint32) (sp *types.StorageProvider, found bool) {
	store := ctx.KVStore(k.storeKey)

	value := store.Get(types.GetStorageProviderKey(k.spSequence.EncodeSequence(id)))
	if value == nil {
		return sp, false
	}

	sp = types.MustUnmarshalStorageProvider(k.cdc, value)
	return sp, true
}

func (k Keeper) MustGetStorageProvider(ctx sdk.Context, id uint32) *types.StorageProvider {
	sp, found := k.GetStorageProvider(ctx, id)
	if !found {
		panic("must get storage provider, but it not exist")
	}
	return sp
}

func (k Keeper) GetStorageProviderByOperatorAddr(ctx sdk.Context, opAddr sdk.AccAddress) (sp *types.StorageProvider, found bool) {
	store := ctx.KVStore(k.storeKey)

	id := store.Get(types.GetStorageProviderByOperatorAddrKey(opAddr))
	if id == nil {
		return sp, false
	}

	return k.GetStorageProvider(ctx, k.spSequence.DecodeSequence(id))
}

func (k Keeper) GetStorageProviderByFundingAddr(ctx sdk.Context, fundAddr sdk.AccAddress) (sp *types.StorageProvider, found bool) {
	store := ctx.KVStore(k.storeKey)

	id := store.Get(types.GetStorageProviderByFundingAddrKey(fundAddr))
	if id == nil {
		return sp, false
	}

	return k.GetStorageProvider(ctx, k.spSequence.DecodeSequence(id))
}

func (k Keeper) GetStorageProviderBySealAddr(ctx sdk.Context, sealAddr sdk.AccAddress) (sp *types.StorageProvider, found bool) {
	store := ctx.KVStore(k.storeKey)

	id := store.Get(types.GetStorageProviderBySealAddrKey(sealAddr))
	if id == nil {
		return sp, false
	}

	return k.GetStorageProvider(ctx, k.spSequence.DecodeSequence(id))
}

func (k Keeper) GetStorageProviderByApprovalAddr(ctx sdk.Context, approvalAddr sdk.AccAddress) (sp *types.StorageProvider, found bool) {
	store := ctx.KVStore(k.storeKey)

	id := store.Get(types.GetStorageProviderByApprovalAddrKey(approvalAddr))
	if id == nil {
		return sp, false
	}

	return k.GetStorageProvider(ctx, k.spSequence.DecodeSequence(id))
}

func (k Keeper) GetStorageProviderByGcAddr(ctx sdk.Context, gcAddr sdk.AccAddress) (sp *types.StorageProvider, found bool) {
	store := ctx.KVStore(k.storeKey)

	id := store.Get(types.GetStorageProviderByGcAddrKey(gcAddr))
	if id == nil {
		return sp, false
	}

	return k.GetStorageProvider(ctx, k.spSequence.DecodeSequence(id))
}

func (k Keeper) SetStorageProvider(ctx sdk.Context, sp *types.StorageProvider) {
	store := ctx.KVStore(k.storeKey)
	bz := types.MustMarshalStorageProvider(k.cdc, sp)

	store.Set(types.GetStorageProviderKey(k.spSequence.EncodeSequence(sp.Id)), bz)
}

func (k Keeper) SetStorageProviderByOperatorAddr(ctx sdk.Context, sp *types.StorageProvider) {
	operatorAddr := sp.GetOperatorAccAddress()
	store := ctx.KVStore(k.storeKey)

	store.Set(types.GetStorageProviderByOperatorAddrKey(operatorAddr), k.spSequence.EncodeSequence(sp.Id))
}

func (k Keeper) SetStorageProviderByFundingAddr(ctx sdk.Context, sp *types.StorageProvider) {
	fundAddr := sp.GetFundingAccAddress()
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetStorageProviderByFundingAddrKey(fundAddr), k.spSequence.EncodeSequence(sp.Id))
}

func (k Keeper) SetStorageProviderBySealAddr(ctx sdk.Context, sp *types.StorageProvider) {
	sealAddr := sp.GetSealAccAddress()
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetStorageProviderBySealAddrKey(sealAddr), k.spSequence.EncodeSequence(sp.Id))
}

func (k Keeper) SetStorageProviderByApprovalAddr(ctx sdk.Context, sp *types.StorageProvider) {
	approvalAddr := sp.GetApprovalAccAddress()
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetStorageProviderByApprovalAddrKey(approvalAddr), k.spSequence.EncodeSequence(sp.Id))
}

func (k Keeper) SetStorageProviderByGcAddr(ctx sdk.Context, sp *types.StorageProvider) {
	gcAddr := sp.GetGcAccAddress()
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetStorageProviderByGcAddrKey(gcAddr), k.spSequence.EncodeSequence(sp.Id))
}

func (k Keeper) GetAllStorageProviders(ctx sdk.Context) (sps []types.StorageProvider) {
	store := ctx.KVStore(k.storeKey)

	iter := storetypes.KVStorePrefixIterator(store, types.StorageProviderKey)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		sp := types.MustUnmarshalStorageProvider(k.cdc, iter.Value())
		sps = append(sps, *sp)
	}
	return sps
}

// SetDepositLockUntil keeps the deposit of a storage provider inside the module account until
// the given height. The watermark only moves forward, so a later challenge cannot shorten the
// lock an earlier one established.
func (k Keeper) SetDepositLockUntil(ctx sdk.Context, spID uint32, height uint64) {
	if height <= k.GetDepositLockUntil(ctx, spID) {
		return
	}
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, height)
	ctx.KVStore(k.storeKey).Set(types.GetDepositLockKey(spID), bz)
}

// ReleaseDepositLockUntil sets the lock to the given height unconditionally, unlike
// SetDepositLockUntil which only moves it forward. It is for the caller that has recomputed
// the height from what is still outstanding, and clears the entry when nothing is.
func (k Keeper) ReleaseDepositLockUntil(ctx sdk.Context, spID uint32, height uint64) {
	store := ctx.KVStore(k.storeKey)
	if height == 0 {
		store.Delete(types.GetDepositLockKey(spID))
		return
	}
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, height)
	store.Set(types.GetDepositLockKey(spID), bz)
}

// GetDepositLockUntil returns the height until which the deposit of a storage provider is locked.
func (k Keeper) GetDepositLockUntil(ctx sdk.Context, spID uint32) uint64 {
	bz := ctx.KVStore(k.storeKey).Get(types.GetDepositLockKey(spID))
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) Exit(ctx sdk.Context, sp *types.StorageProvider) error {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.GetDepositLockKey(sp.Id))
	store.Delete(types.GetStorageProviderByOperatorAddrKey(sdk.MustAccAddressFromHex(sp.OperatorAddress)))
	store.Delete(types.GetStorageProviderByFundingAddrKey(sdk.MustAccAddressFromHex(sp.FundingAddress)))
	store.Delete(types.GetStorageProviderBySealAddrKey(sdk.MustAccAddressFromHex(sp.SealAddress)))
	store.Delete(types.GetStorageProviderByApprovalAddrKey(sdk.MustAccAddressFromHex(sp.ApprovalAddress)))
	store.Delete(types.GetStorageProviderByGcAddrKey(sdk.MustAccAddressFromHex(sp.GcAddress)))
	store.Delete(types.GetStorageProviderKey(k.spSequence.EncodeSequence(sp.Id)))
	store.Delete(types.GetStorageProviderByBlsKeyKey(sp.GetBlsKey()))
	return nil
}

func (k Keeper) SetStorageProviderByBlsKey(ctx sdk.Context, sp *types.StorageProvider) {
	blsPk := sp.GetBlsKey()
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetStorageProviderByBlsKeyKey(blsPk), k.spSequence.EncodeSequence(sp.Id))
}

func (k Keeper) GetStorageProviderByBlsKey(ctx sdk.Context, blsPk []byte) (sp *types.StorageProvider, found bool) {
	store := ctx.KVStore(k.storeKey)
	id := store.Get(types.GetStorageProviderByBlsKeyKey(blsPk))
	if id == nil {
		return sp, false
	}
	return k.GetStorageProvider(ctx, k.spSequence.DecodeSequence(id))
}
