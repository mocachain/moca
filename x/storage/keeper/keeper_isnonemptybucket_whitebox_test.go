package keeper

// MOCA-413: behavior regression test for (Keeper).isNonEmptyBucket.
//
// The fix under test rewrote isNonEmptyBucket to acquire an object-store
// iterator, defer iter.Close(), and return iter.Valid() so the iterator is
// always released. A leak-catching test is impractical here: moca's KVStore
// does NOT panic on a write-while-iterator-open, so the dangling iterator from
// the pre-fix code is not directly observable from a test. Instead, this is a
// BEHAVIOR test that guards the function's correctness (emptiness detection and
// per-bucket isolation) alongside the defer iter.Close() fix.
//
// isNonEmptyBucket is unexported, so this is a true whitebox test in
// package keeper. We build a minimal Keeper inline (only storeKey is used) and
// avoid testutil/keeper, which imports package keeper and would create an
// import cycle for a package-keeper test.

import (
	"testing"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/stretchr/testify/require"

	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

func TestIsNonEmptyBucket(t *testing.T) {
	key := storetypes.NewKVStoreKey(storagetypes.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	// Only storeKey is exercised by isNonEmptyBucket; a partial literal is fine.
	k := Keeper{storeKey: key}

	// 1. An empty bucket has no objects.
	require.False(t, k.isNonEmptyBucket(ctx, "empty-bucket"))

	// 2. A bucket with an object under its object prefix is non-empty.
	prefix.NewStore(ctx.KVStore(key), storagetypes.GetObjectKeyOnlyBucketPrefix("full-bucket")).
		Set([]byte("obj-1"), []byte{1})
	require.True(t, k.isNonEmptyBucket(ctx, "full-bucket"))

	// 3. Isolation: the object in full-bucket must not leak into another bucket.
	require.False(t, k.isNonEmptyBucket(ctx, "other-bucket"))
}
