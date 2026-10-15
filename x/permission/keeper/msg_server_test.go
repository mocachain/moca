package keeper_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/x/challenge"
	"github.com/mocachain/moca/v2/x/permission/keeper"
	"github.com/mocachain/moca/v2/x/permission/types"
)

type TestSuite struct {
	suite.Suite

	cdc              codec.Codec
	permissionKeeper *keeper.Keeper
	storeKey         storetypes.StoreKey

	accountKeeper *types.MockAccountKeeper

	ctx         sdk.Context
	queryClient types.QueryClient
	msgServer   types.MsgServer
}

func (s *TestSuite) SetupTest() {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))
	s.ctx = testCtx.Ctx

	ctrl := gomock.NewController(s.T())

	accountKeeper := types.NewMockAccountKeeper(ctrl)

	s.permissionKeeper = keeper.NewKeeper(
		encCfg.Codec,
		key,
		accountKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	s.cdc = encCfg.Codec
	s.storeKey = key
	s.accountKeeper = accountKeeper

	err := s.permissionKeeper.SetParams(s.ctx, types.DefaultParams())
	s.Require().NoError(err)

	queryHelper := baseapp.NewQueryServerTestHelper(testCtx.Ctx, encCfg.InterfaceRegistry)
	types.RegisterQueryServer(queryHelper, s.permissionKeeper)

	s.queryClient = types.NewQueryClient(queryHelper)
	s.msgServer = keeper.NewMsgServerImpl(*s.permissionKeeper)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) TestUpdateParams() {
	s.Run("success", func() {
		newParams := types.NewParams(20, 20, 200)
		resp, err := s.msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
			Authority: s.permissionKeeper.GetAuthority(),
			Params:    newParams,
		})
		s.Require().NoError(err)
		s.Require().Equal(&types.MsgUpdateParamsResponse{}, resp)
		s.Require().Equal(newParams, s.permissionKeeper.GetParams(s.ctx))

		// restore the defaults set up by SetupTest so this subtest can't affect others.
		s.Require().NoError(s.permissionKeeper.SetParams(s.ctx, types.DefaultParams()))
	})

	s.Run("wrong authority", func() {
		resp, err := s.msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
			Authority: sample.RandAccAddressHex(),
			Params:    types.DefaultParams(),
		})
		s.Require().Nil(resp)
		s.Require().ErrorIs(err, govtypes.ErrInvalidSigner)
	})

	s.Run("invalid params", func() {
		before := s.permissionKeeper.GetParams(s.ctx)

		invalid := types.DefaultParams()
		invalid.MaximumGroupNum = 0
		resp, err := s.msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
			Authority: s.permissionKeeper.GetAuthority(),
			Params:    invalid,
		})
		s.Require().Nil(resp)
		s.Require().Error(err)
		s.Require().Equal(before, s.permissionKeeper.GetParams(s.ctx), "a rejected UpdateParams must not persist anything")
	})
}
