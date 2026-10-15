package keeper_test

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

	"github.com/mocachain/moca/v2/x/permission/keeper"
	"github.com/mocachain/moca/v2/x/permission/types"
)

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

func TestGetParams(t *testing.T) {
	k, ctx := makeKeeper(t)
	params := types.DefaultParams()

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}

func TestGetParams_Empty(t *testing.T) {
	k, ctx := makeKeeper(t)

	require.Equal(t, types.Params{}, k.GetParams(ctx), "GetParams on a fresh store must return the zero value")
}

func TestSetParams_InvalidDoesNotPersist(t *testing.T) {
	k, ctx := makeKeeper(t)

	invalid := types.DefaultParams()
	invalid.MaximumGroupNum = 0
	err := k.SetParams(ctx, invalid)
	require.Error(t, err)
	require.Equal(t, types.Params{}, k.GetParams(ctx), "a rejected SetParams on an empty store must leave it empty")
}

func TestSetParams_InvalidDoesNotOverwriteExisting(t *testing.T) {
	k, ctx := makeKeeper(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, params))

	invalid := types.DefaultParams()
	invalid.MaximumRemoveExpiredPoliciesIteration = 0
	err := k.SetParams(ctx, invalid)
	require.Error(t, err)
	require.Equal(t, params, k.GetParams(ctx), "a rejected SetParams must not overwrite the previously stored params")
}
