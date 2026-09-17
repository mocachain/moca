package keeper_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/x/payment"
	"github.com/mocachain/moca/v2/x/payment/keeper"
	v1 "github.com/mocachain/moca/v2/x/payment/keeper/v1"
	"github.com/mocachain/moca/v2/x/payment/types"
)

// TestMigrator_MigrateV1toV2 seeds a v1-shaped Params under the shared ParamsKey
// and asserts NewMigrator(...).MigrateV1toV2 rewrites it into a v2 Params: the six
// pre-existing fields carried over unchanged, and the two new withdraw-time-lock
// fields backfilled with their defaults (v2's own keeper/v2 package has the
// dedicated test for MigrateStore's own coverage; this only proves the keeper
// package's Migrator wiring calls through to it).
func TestMigrator_MigrateV1toV2(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))
	ctx := testCtx.Ctx

	ctrl := gomock.NewController(t)
	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		types.NewMockBankKeeper(ctrl),
		types.NewMockAccountKeeper(ctrl),
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	oldParams := &v1.Params{
		VersionedParams: v1.VersionedParams{
			ReserveTime:      types.DefaultReserveTime,
			ValidatorTaxRate: types.DefaultValidatorTaxRate,
		},
		PaymentAccountCountLimit: 321,
		ForcedSettleTime:         types.DefaultForcedSettleTime,
		MaxAutoSettleFlowCount:   types.DefaultMaxAutoSettleFlowCount,
		MaxAutoResumeFlowCount:   types.DefaultMaxAutoResumeFlowCount,
		FeeDenom:                 types.DefaultFeeDenom,
	}
	store := ctx.KVStore(key)
	store.Set(types.ParamsKey, encCfg.Codec.MustMarshal(oldParams))

	err := keeper.NewMigrator(*k).MigrateV1toV2(ctx)
	require.NoError(t, err)

	got := k.GetParams(ctx)
	require.Equal(t, oldParams.PaymentAccountCountLimit, got.PaymentAccountCountLimit)
	require.Equal(t, oldParams.ForcedSettleTime, got.ForcedSettleTime)
	require.Equal(t, oldParams.MaxAutoSettleFlowCount, got.MaxAutoSettleFlowCount)
	require.Equal(t, oldParams.MaxAutoResumeFlowCount, got.MaxAutoResumeFlowCount)
	require.Equal(t, oldParams.FeeDenom, got.FeeDenom)
	require.Equal(t, oldParams.VersionedParams.ReserveTime, got.VersionedParams.ReserveTime)
	require.True(t, oldParams.VersionedParams.ValidatorTaxRate.Equal(got.VersionedParams.ValidatorTaxRate))
	require.True(t, types.DefaultWithdrawTimeLockThreshold.Equal(*got.WithdrawTimeLockThreshold))
	require.Equal(t, types.DefaultWithdrawTimeLockDuration, got.WithdrawTimeLockDuration)
}
