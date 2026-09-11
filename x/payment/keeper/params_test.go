package keeper_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/x/payment"
	"github.com/mocachain/moca/v2/x/payment/keeper"
	"github.com/mocachain/moca/v2/x/payment/types"
)

// TestGetParams_NotFound and TestGetVersionedParamsWithTs_NotFound build a keeper
// directly (rather than via makePaymentKeeper) because that helper always seeds
// DefaultParams, which would hide the not-found paths under test here.

func TestGetParams_NotFound(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	ctrl := gomock.NewController(t)
	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		types.NewMockBankKeeper(ctrl),
		types.NewMockAccountKeeper(ctrl),
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	// no SetParams call was ever made on this store
	got := k.GetParams(testCtx.Ctx)
	require.Equal(t, types.Params{}, got)
}

func TestSetParams_InvalidParamsRejected(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)
	original := k.GetParams(ctx)

	invalid := types.DefaultParams()
	invalid.PaymentAccountCountLimit = 0 // fails validatePaymentAccountCountLimit

	err := k.SetParams(ctx, invalid)
	require.Error(t, err)

	// the store must not have been touched by the rejected params
	require.Equal(t, original, k.GetParams(ctx))
}

func TestGetVersionedParamsWithTs_NotFound(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	ctrl := gomock.NewController(t)
	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		types.NewMockBankKeeper(ctrl),
		types.NewMockAccountKeeper(ctrl),
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	// no versioned params were ever stored
	_, err := k.GetVersionedParamsWithTs(testCtx.Ctx, testCtx.Ctx.BlockTime().Unix()+1)
	require.ErrorContains(t, err, "no versioned params found")
}
