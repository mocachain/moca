package keeper_test

import (
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"

	"github.com/mocachain/moca/v2/x/challenge"
	"github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	v1 "github.com/mocachain/moca/v2/x/virtualgroup/keeper/v1"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestMigrator_MigrateV1toV2 asserts the keeper-level Migrator wraps
// keeper/v2.MigrateStore correctly: the 4 original v1.Params fields are preserved
// and the 2 new fields land at their documented defaults. It builds its own
// key/ctx/keeper (rather than using the suite's) so the v1 blob can be written
// directly at the params key before the migration runs.
func (s *TestSuite) TestMigrator_MigrateV1toV2() {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	migKeeper := keeper.NewKeeper(encCfg.Codec, key, authtypes.NewModuleAddress(govtypes.ModuleName).String(), nil, nil, nil, nil)

	oldParams := v1.Params{
		DepositDenom:                      "oldmoca",
		GvgStakingPerBytes:                math.NewInt(777),
		MaxLocalVirtualGroupNumPerBucket:  3,
		MaxGlobalVirtualGroupNumPerFamily: 9,
		MaxStoreSizePerFamily:             12345,
	}
	ctx.KVStore(key).Set(types.ParamsKey, encCfg.Codec.MustMarshal(&oldParams))

	err := keeper.NewMigrator(*migKeeper).MigrateV1toV2(ctx)
	s.Require().NoError(err)

	got := migKeeper.GetParams(ctx)
	s.Require().Equal(oldParams.DepositDenom, got.DepositDenom)
	s.Require().True(oldParams.GvgStakingPerBytes.Equal(got.GvgStakingPerBytes))
	s.Require().Equal(oldParams.MaxGlobalVirtualGroupNumPerFamily, got.MaxGlobalVirtualGroupNumPerFamily)
	s.Require().Equal(oldParams.MaxStoreSizePerFamily, got.MaxStoreSizePerFamily)
	s.Require().Equal(types.DefaultSwapInValidityPeriod, *got.SwapInValidityPeriod)
	s.Require().Equal(types.DefaultSPConcurrentExitNum, *got.SpConcurrentExitNum)
}
