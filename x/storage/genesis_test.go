package storage_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/nullify"
	"github.com/mocachain/moca/v2/x/storage"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
	"github.com/stretchr/testify/require"
)

func makeKeeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)

	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		&types.MockAccountKeeper{},
		&types.MockSpKeeper{},
		&types.MockPaymentKeeper{},
		&types.MockPermissionKeeper{},
		&types.MockVirtualGroupKeeper{},
		&types.MockEVMKeeper{},
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	return k, testCtx.Ctx
}

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	k, ctx := makeKeeper(t)
	storage.InitGenesis(ctx, *k, genesisState)
	got := storage.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}

func TestInitGenesis_PanicsOnInvalidParams(t *testing.T) {
	// A zero-value Params fails Params.Validate() (e.g. MaxSegmentSize must be
	// positive), so SetParams returns an error and InitGenesis must panic on it.
	invalidGenesisState := types.GenesisState{
		Params: types.Params{},
	}

	k, ctx := makeKeeper(t)
	require.Panics(t, func() {
		storage.InitGenesis(ctx, *k, invalidGenesisState)
	})
}
