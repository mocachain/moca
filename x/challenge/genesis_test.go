package challenge_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/nullify"
	"github.com/mocachain/moca/v2/x/challenge"
	"github.com/mocachain/moca/v2/x/challenge/keeper"
	"github.com/mocachain/moca/v2/x/challenge/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	k, ctx := makeKeeper(t)
	challenge.InitGenesis(ctx, *k, genesisState)
	got := challenge.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}

// InitGenesis must panic rather than silently accept params that fail
// validation: SetParams rejects them, and InitGenesis panics on that error.
func TestInitGenesis_InvalidParams_Panics(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}
	genesisState.Params.HeartbeatInterval = 0

	k, ctx := makeKeeper(t)
	require.Panics(t, func() {
		challenge.InitGenesis(ctx, *k, genesisState)
	})
}

func makeKeeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		key,
		&types.MockBankKeeper{},
		&types.MockStorageKeeper{},
		&types.MockSpKeeper{},
		&types.MockStakingKeeper{},
		&types.MockPaymentKeeper{},
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	return k, testCtx.Ctx
}
