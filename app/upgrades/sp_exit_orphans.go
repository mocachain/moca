package upgrades

import (
	"context"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	spkeeper "github.com/mocachain/moca/v2/x/sp/keeper"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// PruneExitedStorageProviderEntries removes the storage-price and maintenance-record
// entries of storage providers that exited before Keeper.Exit started deleting them.
//
// An entry is kept only if its storage provider still exists. Keys are collected
// first and deleted after the iterator is closed. It is idempotent: a state without
// such entries yields zero removals.
//
// Returns the number of removed storage-price and maintenance-record entries.
func PruneExitedStorageProviderEntries(ctx context.Context, k spkeeper.Keeper, storeKey *storetypes.KVStoreKey) (int, int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.KVStore(storeKey)

	var orphanPriceIDs []uint32
	priceIter := storetypes.KVStorePrefixIterator(prefix.NewStore(store, sptypes.SpStoragePriceKeyPrefix), []byte{})
	for ; priceIter.Valid(); priceIter.Next() {
		spID := sptypes.ParseSpStoragePriceKey(priceIter.Key())
		if _, found := k.GetStorageProvider(sdkCtx, spID); !found {
			orphanPriceIDs = append(orphanPriceIDs, spID)
		}
	}
	if err := priceIter.Close(); err != nil {
		return 0, 0, err
	}
	for _, spID := range orphanPriceIDs {
		k.DeleteSpStoragePrice(sdkCtx, spID)
	}

	var orphanRecordKeys [][]byte
	recordIter := storetypes.KVStorePrefixIterator(store, sptypes.StorageProviderMaintenanceRecordPrefix)
	for ; recordIter.Valid(); recordIter.Next() {
		operator := sdk.AccAddress(recordIter.Key()[len(sptypes.StorageProviderMaintenanceRecordPrefix):])
		if _, found := k.GetStorageProviderByOperatorAddr(sdkCtx, operator); !found {
			orphanRecordKeys = append(orphanRecordKeys, append([]byte(nil), recordIter.Key()...))
		}
	}
	if err := recordIter.Close(); err != nil {
		return len(orphanPriceIDs), 0, err
	}
	for _, key := range orphanRecordKeys {
		store.Delete(key)
	}

	sdkCtx.Logger().Info("sp: removed entries of exited storage providers", "storage_prices", len(orphanPriceIDs), "maintenance_records", len(orphanRecordKeys))
	return len(orphanPriceIDs), len(orphanRecordKeys), nil
}
