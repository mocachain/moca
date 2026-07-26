package permission_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/nullify"
	"github.com/mocachain/moca/v2/x/permission"
	"github.com/mocachain/moca/v2/x/permission/keeper"
	"github.com/mocachain/moca/v2/x/permission/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}
	k, ctx := makeKeeper(t)
	permission.InitGenesis(ctx, *k, genesisState)
	got := permission.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}

func makeKeeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		&types.MockAccountKeeper{},
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	return k, testCtx.Ctx
}
