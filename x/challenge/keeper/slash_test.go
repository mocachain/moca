package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/x/challenge/keeper"
	"github.com/evmos/evmos/v12/x/challenge/types"
)

func createSlash(keeper *keeper.Keeper, ctx sdk.Context, n int) []types.Slash {
	items := make([]types.Slash, n)
	for i := range items {
		items[i].ObjectId = sdkmath.NewUint(uint64(i))
		items[i].Height = uint64(i)
		items[i].SpId = uint32(i + 1)
		keeper.SaveSlash(ctx, items[i])
	}
	return items
}

func TestRemoveRecentSlash(t *testing.T) {
	keeper, ctx := makeKeeper(t)
	items := createSlash(keeper, ctx, 10)
	for _, item := range items {
		keeper.RemoveSlashUntil(ctx, item.Height)
		found := keeper.ExistsSlash(ctx, item.SpId, item.ObjectId)
		require.False(t, found)
	}
}

func TestRemoveSpSlashAmount(t *testing.T) {
	keeper, ctx := makeKeeper(t)
	keeper.SetSpSlashAmount(ctx, 1, sdkmath.NewInt(100))
	keeper.SetSpSlashAmount(ctx, 2, sdkmath.NewInt(200))
	keeper.ClearSpSlashAmount(ctx)
	require.True(t, keeper.GetSpSlashAmount(ctx, 1).Int64() == 0)
	require.True(t, keeper.GetSpSlashAmount(ctx, 2).Int64() == 0)
}

// TestSlashKeyUniqueness validates the fix for HIGH-006 by ensuring that the key generation
// logic for slashes is unique per storage provider (SP). It acts as a precise regression test.
//
// The test operates in a black-box manner from the perspective of the keeper_test package.
// It uses the public methods `SaveSlash` and `ExistsSlash` to indirectly verify the behavior
// of the private `getSlashKeyBytes` function.
//
// How it works:
// 1. It saves two slash records for the SAME objectID but with DIFFERENT spIDs.
// 2. It then checks for the existence of both records.
//
//   - On the BUGGY code: `getSlashKeyBytes` would generate the same key for both spIDs.
//     The second `SaveSlash` call would OVERWRITE the first one. As a result, the assertion
//     `require.True(t, keeper.ExistsSlash(ctx, spID1, objectID))` would FAIL.
//
//   - On the FIXED code: `getSlashKeyBytes` generates unique keys for each spID. Both slash
//     records will coexist in the store. Therefore, both `ExistsSlash` calls will return true,
//     and the test will PASS.
func TestSlashKeyUniqueness(t *testing.T) {
	keeper, ctx := makeKeeper(t)
	objectID1 := sdkmath.NewUint(12345)
	objectID2 := sdkmath.NewUint(67890)
	zeroObjectID := sdkmath.ZeroUint()

	spID1 := uint32(1)
	spID2 := uint32(2)

	height1 := uint64(ctx.BlockHeight())
	height2 := height1 + 1

	// 1. Save slash records with different spIDs, objectIDs, and heights.
	// (sp1, obj1, h1)
	keeper.SaveSlash(ctx, types.Slash{
		SpId:     spID1,
		ObjectId: objectID1,
		Height:   height1,
	})
	// (sp2, obj1, h2) - Same object, different sp, different height
	keeper.SaveSlash(ctx, types.Slash{
		SpId:     spID2,
		ObjectId: objectID1,
		Height:   height2,
	})
	// (sp1, obj2, h1) - Same sp, different object
	keeper.SaveSlash(ctx, types.Slash{
		SpId:     spID1,
		ObjectId: objectID2,
		Height:   height1,
	})
	// (sp1, zero_obj, h1) - Boundary condition: zero objectID
	keeper.SaveSlash(ctx, types.Slash{
		SpId:     spID1,
		ObjectId: zeroObjectID,
		Height:   height1,
	})
	// (sp2, zero_obj, h1) - Boundary condition: zero objectID with different sp
	keeper.SaveSlash(ctx, types.Slash{
		SpId:     spID2,
		ObjectId: zeroObjectID,
		Height:   height1,
	})

	// 2. Verify initial existence - This proves key uniqueness across spID and objectID.
	require.True(t, keeper.ExistsSlash(ctx, spID1, objectID1), "Slash for (sp1, obj1) should exist")
	require.True(t, keeper.ExistsSlash(ctx, spID2, objectID1), "Slash for (sp2, obj1) should exist")
	require.True(t, keeper.ExistsSlash(ctx, spID1, objectID2), "Slash for (sp1, obj2) should exist")
	require.True(t, keeper.ExistsSlash(ctx, spID1, zeroObjectID), "Slash for (sp1, zero_obj) should exist")
	require.True(t, keeper.ExistsSlash(ctx, spID2, zeroObjectID), "Slash for (sp2, zero_obj) should exist")

	// 3. Test precise cleanup logic with RemoveSlashUntil.
	// This should only remove records at or below height1.
	keeper.RemoveSlashUntil(ctx, height1)

	// 4. Verify post-cleanup state.
	// Records at height1 should be gone.
	require.False(t, keeper.ExistsSlash(ctx, spID1, objectID1), "Slash for (sp1, obj1) at h1 should have been removed")
	require.False(t, keeper.ExistsSlash(ctx, spID1, objectID2), "Slash for (sp1, obj2) at h1 should have been removed")
	require.False(t, keeper.ExistsSlash(ctx, spID1, zeroObjectID), "Slash for (sp1, zero_obj) at h1 should have been removed")
	require.False(t, keeper.ExistsSlash(ctx, spID2, zeroObjectID), "Slash for (sp2, zero_obj) at h1 should have been removed")

	// The record at height2 should remain, proving that cleanup logic didn't suffer from key collision.
	require.True(t, keeper.ExistsSlash(ctx, spID2, objectID1), "Slash for (sp2, obj1) at h2 should NOT have been removed")
}
