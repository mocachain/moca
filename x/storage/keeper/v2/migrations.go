package v2

import (
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/x/storage/types"
)

// bucketRateLimitStatusKeyLength is a BucketFlowRateLimitStatus key's length
// as it was mistakenly written under BucketRateLimitPrefix: the 1-byte
// prefix plus a 32-byte keccak256 hash of the bucket name. A
// BucketFlowRateLimit key is longer -- prefix + paymentAccount (20 bytes) +
// bucketOwner (20 bytes) + hash (32 bytes) = 73 bytes -- so the two shapes
// never collide and this length alone tells them apart.
var bucketRateLimitStatusKeyLength = len(types.BucketRateLimitPrefix) + 32

// MigrateStore moves BucketFlowRateLimitStatus entries that were mistakenly
// written under the shared BucketRateLimitPrefix (0x71) to the dedicated
// BucketRateLimitStatusPrefix (0x72). BucketFlowRateLimit entries, which are
// longer, are left untouched. Matching keys are collected up front and only
// written/deleted afterwards, so the migration never mutates the store while
// iterating over it.
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey) error {
	store := ctx.KVStore(storeKey)

	iterator := storetypes.KVStorePrefixIterator(store, types.BucketRateLimitPrefix)
	defer iterator.Close()

	type staleEntry struct {
		oldKey []byte
		hash   []byte
		value  []byte
	}

	var stale []staleEntry
	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		if len(key) != bucketRateLimitStatusKeyLength {
			continue
		}

		stale = append(stale, staleEntry{
			oldKey: append([]byte{}, key...),
			hash:   append([]byte{}, key[len(types.BucketRateLimitPrefix):]...),
			value:  append([]byte{}, iterator.Value()...),
		})
	}

	for _, entry := range stale {
		newKey := append(append([]byte{}, types.BucketRateLimitStatusPrefix...), entry.hash...)
		store.Set(newKey, entry.value)
		store.Delete(entry.oldKey)
	}

	return nil
}
