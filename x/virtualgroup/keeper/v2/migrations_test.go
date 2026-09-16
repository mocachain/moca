package v2_test

import (
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/challenge"
	v1 "github.com/mocachain/moca/v2/x/virtualgroup/keeper/v1"
	v2 "github.com/mocachain/moca/v2/x/virtualgroup/keeper/v2"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestMigrateStore asserts the v1 -> v2 params migration preserves the 4 original
// fields and backfills the 2 new fields (SwapInValidityPeriod, SpConcurrentExitNum)
// with their documented defaults.
func TestMigrateStore(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	oldParams := v1.Params{
		DepositDenom:                      "oldmoca",
		GvgStakingPerBytes:                math.NewInt(777),
		MaxLocalVirtualGroupNumPerBucket:  3,
		MaxGlobalVirtualGroupNumPerFamily: 9,
		MaxStoreSizePerFamily:             12345,
	}
	ctx.KVStore(key).Set(types.ParamsKey, encCfg.Codec.MustMarshal(&oldParams))

	err := v2.MigrateStore(ctx, key, encCfg.Codec)
	require.NoError(t, err)

	bz := ctx.KVStore(key).Get(types.ParamsKey)
	var got types.Params
	encCfg.Codec.MustUnmarshal(bz, &got)

	require.Equal(t, oldParams.DepositDenom, got.DepositDenom)
	require.True(t, oldParams.GvgStakingPerBytes.Equal(got.GvgStakingPerBytes))
	require.Equal(t, oldParams.MaxGlobalVirtualGroupNumPerFamily, got.MaxGlobalVirtualGroupNumPerFamily)
	require.Equal(t, oldParams.MaxStoreSizePerFamily, got.MaxStoreSizePerFamily)
	require.Equal(t, types.DefaultSwapInValidityPeriod, *got.SwapInValidityPeriod)
	require.Equal(t, types.DefaultSPConcurrentExitNum, *got.SpConcurrentExitNum)
}
