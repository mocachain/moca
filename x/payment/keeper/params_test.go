package keeper_test

import (
	"testing"
	"time"

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

// TestGetVersionedParamsWithTs_ExclusiveUpperBound pins the deliberately
// exclusive upper bound documented on GetVersionedParamsWithTs: a version
// written exactly at ts is not yet visible to a query at that same ts (the
// prior version is returned instead, matching txs executed "at" ts using the
// old parameters), and becomes visible starting at ts+1.
func TestGetVersionedParamsWithTs_ExclusiveUpperBound(t *testing.T) {
	k, ctx, _ := makePaymentKeeper(t)

	const t1 = int64(1000)
	const t2 = int64(2000)

	params1 := types.DefaultParams()
	params1.VersionedParams.ReserveTime = 100_000
	ctx = ctx.WithBlockTime(time.Unix(t1, 0))
	require.NoError(t, k.SetParams(ctx, params1))

	params2 := types.DefaultParams()
	params2.VersionedParams.ReserveTime = 200_000
	ctx = ctx.WithBlockTime(time.Unix(t2, 0))
	require.NoError(t, k.SetParams(ctx, params2))

	// exactly at t2: the version written at t2 must be excluded, so the t1
	// version (still in effect for anything executing "at" t2) is returned.
	got, err := k.GetVersionedParamsWithTs(ctx, t2)
	require.NoError(t, err)
	require.Equal(t, uint64(100_000), got.ReserveTime, "query at t2 must still return the t1 version")

	// t2+1: the t2 version is now visible
	got, err = k.GetVersionedParamsWithTs(ctx, t2+1)
	require.NoError(t, err)
	require.Equal(t, uint64(200_000), got.ReserveTime, "query at t2+1 must return the t2 version")
}
