package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	types2 "github.com/cosmos/cosmos-sdk/x/auth/types"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/x/virtualgroup"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func (s *TestSuite) TestGenesis() {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	s.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), gomock.Any()).Return(types2.NewEmptyModuleAccount(types.ModuleName))
	s.accountKeeper.EXPECT().SetModuleAccount(gomock.Any(), gomock.Any()).Return()
	s.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), gomock.Any()).Return(sdk.NewCoins(sdk.NewCoin(genesisState.Params.DepositDenom, math.ZeroInt())))
	virtualgroup.InitGenesis(s.ctx, *s.virtualgroupKeeper, genesisState)

	got := virtualgroup.ExportGenesis(s.ctx, *s.virtualgroupKeeper)
	s.Require().NotNil(got)
	s.Require().Equal(genesisState.Params, got.Params)
}

// TestInitGenesis_PanicsWhenModuleAccountMissing asserts InitGenesis refuses to
// proceed if the gvg staking module account was never created during app wiring.
func (s *TestSuite) TestInitGenesis_PanicsWhenModuleAccountMissing() {
	genesisState := types.GenesisState{Params: types.DefaultParams()}

	s.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), gomock.Any()).Return(nil)

	s.Require().Panics(func() {
		virtualgroup.InitGenesis(s.ctx, *s.virtualgroupKeeper, genesisState)
	})
}

// TestInitGenesis_PanicsWhenDepositPoolBalanceMismatched asserts InitGenesis
// refuses to proceed if the deposit pool already holds a balance that does not
// match the (zero) balance genesis expects to start from.
func (s *TestSuite) TestInitGenesis_PanicsWhenDepositPoolBalanceMismatched() {
	genesisState := types.GenesisState{Params: types.DefaultParams()}

	s.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), gomock.Any()).Return(types2.NewEmptyModuleAccount(types.ModuleName))
	s.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), gomock.Any()).
		Return(sdk.NewCoins(sdk.NewCoin(genesisState.Params.DepositDenom, math.NewInt(500))))

	s.Require().Panics(func() {
		virtualgroup.InitGenesis(s.ctx, *s.virtualgroupKeeper, genesisState)
	})
}
