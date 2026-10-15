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
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestParamGetters exercises every keeper-level param accessor in one round trip,
// including the two accessors (SwapInValidityPeriod, SpConcurrentExitNum) that
// convert their math.Int params down to plain Go integers.
func (s *TestSuite) TestParamGetters() {
	params := types.NewParams("abc", math.NewInt(123), 5, 999, math.NewInt(10), math.NewInt(2))
	s.Require().NoError(s.virtualgroupKeeper.SetParams(s.ctx, params))

	s.Require().Equal("abc", s.virtualgroupKeeper.DepositDenomForGVG(s.ctx))
	s.Require().True(math.NewInt(123).Equal(s.virtualgroupKeeper.GVGStakingPerBytes(s.ctx)))
	s.Require().Equal(uint32(5), s.virtualgroupKeeper.MaxGlobalVirtualGroupNumPerFamily(s.ctx))
	s.Require().Equal(uint64(999), s.virtualgroupKeeper.MaxStoreSizePerFamily(s.ctx))
	s.Require().Equal(uint64(10), s.virtualgroupKeeper.SwapInValidityPeriod(s.ctx))
	s.Require().Equal(uint32(2), s.virtualgroupKeeper.SpConcurrentExitNum(s.ctx))
}

// TestGetParams_ReturnsZeroValueBeforeAnySet asserts the bz == nil branch: a keeper
// whose store has never had SetParams called returns the zero-value Params instead
// of panicking. Uses its own fresh key/ctx/keeper since the suite's SetupTest
// always calls SetParams before every test.
func (s *TestSuite) TestGetParams_ReturnsZeroValueBeforeAnySet() {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))
	freshKeeper := keeper.NewKeeper(encCfg.Codec, key, authtypes.NewModuleAddress(govtypes.ModuleName).String(), nil, nil, nil, nil)

	s.Require().Equal(types.Params{}, freshKeeper.GetParams(testCtx.Ctx))
}

// TestSetParams_RejectsInvalidParams asserts SetParams validates before writing: an
// invalid value is rejected and the previously-set params are left untouched.
func (s *TestSuite) TestSetParams_RejectsInvalidParams() {
	invalid := types.DefaultParams()
	invalid.MaxGlobalVirtualGroupNumPerFamily = 0

	err := s.virtualgroupKeeper.SetParams(s.ctx, invalid)
	s.Require().Error(err)
	s.Require().Equal(types.DefaultParams(), s.virtualgroupKeeper.GetParams(s.ctx))
}
