package keeper_test

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/sp/keeper"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func (s *KeeperTestSuite) TestGetParams() {
	k := s.spKeeper
	ctx := s.ctx
	params := types.DefaultParams()

	err := k.SetParams(ctx, params)
	s.Require().NoError(err)

	require.EqualValues(s.T(), params, k.GetParams(ctx))
}

func (s *KeeperTestSuite) TestSecondarySpStorePriceRatio() {
	s.Require().True(types.DefaultSecondarySpStorePriceRatio.Equal(s.spKeeper.SecondarySpStorePriceRatio(s.ctx)))
}

// TestGetParamsEmptyStore exercises GetParams against a store that has never had
// params written to it (SetupTest always seeds DefaultParams, so this needs a
// second, fresh keeper/store pair).
func (s *KeeperTestSuite) TestGetParamsEmptyStore() {
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test_empty_params"))
	k := keeper.NewKeeper(s.cdc, key, s.accountKeeper, s.bankKeeper, s.authzKeeper, s.spKeeper.GetAuthority())

	require.Equal(s.T(), types.Params{}, k.GetParams(testCtx.Ctx))
}

// TestSetParamsInvalid asserts SetParams rejects invalid params and leaves the
// previously stored params untouched.
func (s *KeeperTestSuite) TestSetParamsInvalid() {
	err := s.spKeeper.SetParams(s.ctx, types.Params{})
	require.Error(s.T(), err)
	require.Equal(s.T(), types.DefaultParams(), s.spKeeper.GetParams(s.ctx))
}
