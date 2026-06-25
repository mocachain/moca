package upgrades

import (
	"bytes"
	"context"

	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CleanupFeegrantQueueOrphans removes orphaned fee-allowance expiration-queue
// entries left behind by the pre-fix x/feegrant revokeAllowance.
//
// This rebuilds the queue to match live state: an entry is kept only if there is
// a live allowance for its (granter, grantee) pair whose expiry equals the
// entry's expiry. Everything else is deleted. It is idempotent — a clean queue
// yields zero removals — and reads the pair from the entry's own key bytes, so
// it does not depend on any address-string encoding.
//
// Returns the number of orphaned entries removed.
func CleanupFeegrantQueueOrphans(ctx context.Context, k feegrantkeeper.Keeper, storeKey *storetypes.KVStoreKey) (int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.KVStore(storeKey)

	// Snapshot all queue entries first; do not mutate the store while iterating.
	type queueEntry struct {
		key              []byte
		granter, grantee sdk.AccAddress
	}
	var entries []queueEntry
	it := storetypes.KVStorePrefixIterator(store, feegrant.FeeAllowanceQueueKeyPrefix)
	for ; it.Valid(); it.Next() {
		key := append([]byte(nil), it.Key()...)
		granterBz, granteeBz := feegrant.ParseAddressesFromFeeAllowanceQueueKey(key)
		entries = append(entries, queueEntry{
			key:     key,
			granter: sdk.AccAddress(granterBz),
			grantee: sdk.AccAddress(granteeBz),
		})
	}
	if err := it.Close(); err != nil {
		return 0, err
	}

	removed := 0
	for _, e := range entries {
		if isOrphanQueueEntry(ctx, k, e.granter, e.grantee, e.key) {
			store.Delete(e.key)
			removed++
		}
	}

	sdkCtx.Logger().Info("feegrant: removed orphaned expiration-queue entries", "removed", removed)
	return removed, nil
}

// isOrphanQueueEntry reports whether queueKey is not backed by a live allowance
// (for the given pair) whose expiry matches the entry's expiry.
func isOrphanQueueEntry(ctx context.Context, k feegrantkeeper.Keeper, granter, grantee sdk.AccAddress, queueKey []byte) bool {
	allowance, err := k.GetAllowance(ctx, granter, grantee)
	if err != nil {
		return true // no live allowance for this pair
	}
	exp, err := allowance.ExpiresAt()
	if err != nil || exp == nil {
		return true // live allowance no longer carries an expiry
	}
	want := feegrant.FeeAllowancePrefixQueue(exp, feegrant.FeeAllowanceKey(granter, grantee)[1:])
	return !bytes.Equal(want, queueKey)
}
