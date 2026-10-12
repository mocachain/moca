package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	types2 "github.com/mocachain/moca/v2/sdk/types"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

// TestInitGenesisInvalidStatus asserts InitGenesis panics when a genesis storage
// provider is neither STATUS_IN_SERVICE nor STATUS_IN_JAILED. This must be
// checked before any account-keeper/bank-keeper call, so no mocks are armed.
func (s *KeeperTestSuite) TestInitGenesisInvalidStatus() {
	genState := types.GenesisState{
		Params: types.DefaultParams(),
		StorageProviders: []types.StorageProvider{
			{
				Id:              1,
				OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
				Status:          types.STATUS_GRACEFUL_EXITING,
			},
		},
	}

	require.Panics(s.T(), func() {
		s.spKeeper.InitGenesis(s.ctx, genState)
	})
}

// TestInitGenesisNoModuleAccount asserts InitGenesis panics when the sp module
// account has not been set up, without ever consulting the bank keeper.
func (s *KeeperTestSuite) TestInitGenesisNoModuleAccount() {
	genState := types.GenesisState{Params: types.DefaultParams()}

	s.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), types.ModuleName).Return(nil)

	require.Panics(s.T(), func() {
		s.spKeeper.InitGenesis(s.ctx, genState)
	})
}

// TestInitGenesisSetsModuleAccountOnZeroBalance covers the branch that
// (re-)registers the module account when its balance reads as zero, and must
// not panic afterward because a zero balance matches a zero expected deposit.
func (s *KeeperTestSuite) TestInitGenesisSetsModuleAccountOnZeroBalance() {
	genState := types.GenesisState{Params: types.DefaultParams()}
	macc := authtypes.NewEmptyModuleAccount(types.ModuleName)

	s.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), types.ModuleName).Return(macc)
	s.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), macc.GetAddress()).Return(sdk.NewCoins())
	s.accountKeeper.EXPECT().SetModuleAccount(gomock.Any(), macc)

	require.NotPanics(s.T(), func() {
		s.spKeeper.InitGenesis(s.ctx, genState)
	})
}

// TestInitGenesisBalanceMismatch asserts InitGenesis panics when the module
// account's actual balance doesn't match the sum of genesis SP deposits.
func (s *KeeperTestSuite) TestInitGenesisBalanceMismatch() {
	genState := types.GenesisState{
		Params: types.DefaultParams(),
		StorageProviders: []types.StorageProvider{
			{
				Id:              1,
				OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
				Status:          types.STATUS_IN_SERVICE,
				TotalDeposit:    math.NewIntWithDecimal(10, types2.DecimalMOCA),
			},
		},
	}
	macc := authtypes.NewEmptyModuleAccount(types.ModuleName)

	s.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), types.ModuleName).Return(macc)
	s.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), macc.GetAddress()).Return(
		sdk.NewCoins(sdk.NewCoin(types.DefaultDepositDenom, math.NewIntWithDecimal(999, types2.DecimalMOCA))),
	)

	require.Panics(s.T(), func() {
		s.spKeeper.InitGenesis(s.ctx, genState)
	})
}

// TestInitGenesisSuccess exercises the happy path across two genesis storage
// providers (one in each of the two allowed statuses) whose combined deposit
// matches the module account's actual balance.
func (s *KeeperTestSuite) TestInitGenesisSuccess() {
	deposit := math.NewIntWithDecimal(10, types2.DecimalMOCA)
	genState := types.GenesisState{
		Params: types.DefaultParams(),
		StorageProviders: []types.StorageProvider{
			{
				Id:              1,
				OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
				Status:          types.STATUS_IN_SERVICE,
				TotalDeposit:    deposit,
			},
			{
				Id:              2,
				OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
				Status:          types.STATUS_IN_JAILED,
				TotalDeposit:    deposit,
			},
		},
	}
	macc := authtypes.NewEmptyModuleAccount(types.ModuleName)

	s.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), types.ModuleName).Return(macc)
	s.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), macc.GetAddress()).Return(
		sdk.NewCoins(sdk.NewCoin(types.DefaultDepositDenom, deposit.MulRaw(2))),
	)

	require.NotPanics(s.T(), func() {
		s.spKeeper.InitGenesis(s.ctx, genState)
	})

	for _, id := range []uint32{1, 2} {
		_, found := s.spKeeper.GetStorageProvider(s.ctx, id)
		require.True(s.T(), found)
	}
}
