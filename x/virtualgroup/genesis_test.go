package virtualgroup_test

import (
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/nullify"
	"github.com/mocachain/moca/v2/x/challenge"
	"github.com/mocachain/moca/v2/x/virtualgroup"
	"github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// vgFixture bundles a fresh virtualgroup keeper with the mocks backing it,
// built the same way as the keeper package's own TestSuite (see
// keeper/payment_test.go SetupTest). Shared by genesis_test.go, module_test.go
// and module_simulation_test.go.
type vgFixture struct {
	encCfg        moduletestutil.TestEncodingConfig
	keeper        *keeper.Keeper
	ctx           sdk.Context
	accountKeeper *types.MockAccountKeeper
	bankKeeper    *types.MockBankKeeper
	spKeeper      *types.MockSpKeeper
	paymentKeeper *types.MockPaymentKeeper
}

func setupFixture(t *testing.T) vgFixture {
	encCfg := moduletestutil.MakeTestEncodingConfig(challenge.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	ctrl := gomock.NewController(t)
	accountKeeper := types.NewMockAccountKeeper(ctrl)
	bankKeeper := types.NewMockBankKeeper(ctrl)
	spKeeper := types.NewMockSpKeeper(ctrl)
	paymentKeeper := types.NewMockPaymentKeeper(ctrl)

	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		spKeeper,
		accountKeeper,
		bankKeeper,
		paymentKeeper,
	)

	return vgFixture{
		encCfg:        encCfg,
		keeper:        k,
		ctx:           testCtx.Ctx,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		spKeeper:      spKeeper,
		paymentKeeper: paymentKeeper,
	}
}

// expectEmptyGenesisAccount wires the account/bank keeper mocks for the
// no-existing-balance path through keeper.InitGenesis, mirroring
// keeper/genesis_test.go's TestGenesis.
func expectEmptyGenesisAccount(fx vgFixture, depositDenom string) {
	fx.accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), types.ModuleName).
		Return(authtypes.NewEmptyModuleAccount(types.ModuleName))
	fx.accountKeeper.EXPECT().SetModuleAccount(gomock.Any(), gomock.Any()).Return()
	fx.bankKeeper.EXPECT().GetAllBalances(gomock.Any(), gomock.Any()).
		Return(sdk.NewCoins(sdk.NewCoin(depositDenom, math.ZeroInt())))
}

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	fx := setupFixture(t)
	expectEmptyGenesisAccount(fx, genesisState.Params.DepositDenom)

	virtualgroup.InitGenesis(fx.ctx, *fx.keeper, genesisState)

	got := virtualgroup.ExportGenesis(fx.ctx, *fx.keeper)
	require.NotNil(t, got)
	require.Equal(t, genesisState.Params, got.Params)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}

// TestInitGenesis_PanicsOnInvalidParams covers InitGenesis' panic branch: an
// invalid Params blob fails keeper.SetParams' validation, and InitGenesis
// must not swallow that error.
func TestInitGenesis_PanicsOnInvalidParams(t *testing.T) {
	fx := setupFixture(t)

	require.Panics(t, func() {
		virtualgroup.InitGenesis(fx.ctx, *fx.keeper, types.GenesisState{Params: types.Params{}})
	})
}
