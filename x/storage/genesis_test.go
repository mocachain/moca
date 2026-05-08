package storage_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/mocachain/moca/v2/testutil/nullify"
	"github.com/mocachain/moca/v2/x/storage"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
	"github.com/stretchr/testify/require"
)

func makeKeeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	tStoreKey := storetypes.NewTransientStoreKey(types.TStoreKey)

	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		tStoreKey,
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
