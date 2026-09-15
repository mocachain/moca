package v2_test

import (
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/stretchr/testify/require"

	v1 "github.com/mocachain/moca/v2/x/payment/keeper/v1"
	v2 "github.com/mocachain/moca/v2/x/payment/keeper/v2"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestMigrateStore(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	oldParams := v1.Params{
		VersionedParams: v1.VersionedParams{
			ReserveTime:      12345,
			ValidatorTaxRate: math.LegacyNewDecWithPrec(5, 2),
		},
		PaymentAccountCountLimit: 50,
		ForcedSettleTime:         999,
		MaxAutoSettleFlowCount:   10,
		MaxAutoResumeFlowCount:   20,
		FeeDenom:                 "atest",
	}
	store := ctx.KVStore(storeKey)
	store.Set(types.ParamsKey, cdc.MustMarshal(&oldParams))

	require.NoError(t, v2.MigrateStore(ctx, storeKey, cdc))

	bz := store.Get(types.ParamsKey)
	require.NotNil(t, bz)
	var newParams types.Params
	cdc.MustUnmarshal(bz, &newParams)

	require.Equal(t, oldParams.VersionedParams.ReserveTime, newParams.VersionedParams.ReserveTime)
	require.True(t, oldParams.VersionedParams.ValidatorTaxRate.Equal(newParams.VersionedParams.ValidatorTaxRate))
	require.Equal(t, oldParams.ForcedSettleTime, newParams.ForcedSettleTime)
	require.Equal(t, oldParams.PaymentAccountCountLimit, newParams.PaymentAccountCountLimit)
	require.Equal(t, oldParams.MaxAutoSettleFlowCount, newParams.MaxAutoSettleFlowCount)
	require.Equal(t, oldParams.MaxAutoResumeFlowCount, newParams.MaxAutoResumeFlowCount)
	require.Equal(t, oldParams.FeeDenom, newParams.FeeDenom)
	require.NotNil(t, newParams.WithdrawTimeLockThreshold)
	require.True(t, types.DefaultWithdrawTimeLockThreshold.Equal(*newParams.WithdrawTimeLockThreshold))
	require.Equal(t, types.DefaultWithdrawTimeLockDuration, newParams.WithdrawTimeLockDuration)
}
