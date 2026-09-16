package payment_test

import (
	"errors"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestAppModuleBasic_Name(t *testing.T) {
	require.Equal(t, types.ModuleName, payment.AppModuleBasic{}.Name())
}

func TestAppModuleBasic_DefaultGenesis(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})

	bz := payment.AppModuleBasic{}.DefaultGenesis(encCfg.Codec)

	var gs types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(bz, &gs))
	require.Equal(t, types.DefaultParams(), gs.Params)
	require.Empty(t, gs.StreamRecordList)
	require.Empty(t, gs.PaymentAccountCountList)
	require.Empty(t, gs.PaymentAccountList)
	require.Empty(t, gs.AutoSettleRecordList)
}

func TestAppModuleBasic_ValidateGenesis(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	basic := payment.AppModuleBasic{}

	validBz := encCfg.Codec.MustMarshalJSON(types.DefaultGenesis())
	require.NoError(t, basic.ValidateGenesis(encCfg.Codec, nil, validBz))

	err := basic.ValidateGenesis(encCfg.Codec, nil, []byte("not-json"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal")

	dupAddr := sample.RandAccAddress()
	invalidGs := &types.GenesisState{
		Params: types.DefaultParams(),
		StreamRecordList: []types.StreamRecord{
			*types.NewStreamRecord(dupAddr, 0),
			*types.NewStreamRecord(dupAddr, 0),
		},
	}
	invalidBz := encCfg.Codec.MustMarshalJSON(invalidGs)
	err = basic.ValidateGenesis(encCfg.Codec, nil, invalidBz)
	require.ErrorContains(t, err, "duplicated index for streamRecord")
}

func TestAppModuleBasic_RegisterGRPCGatewayRoutes(t *testing.T) {
	mux := runtime.NewServeMux()
	require.NotPanics(t, func() {
		payment.AppModuleBasic{}.RegisterGRPCGatewayRoutes(client.Context{}, mux)
	})
}

func TestAppModuleBasic_Commands(t *testing.T) {
	require.NotNil(t, payment.AppModuleBasic{}.GetTxCmd())
	require.NotNil(t, payment.AppModuleBasic{}.GetQueryCmd())
}

func TestAppModule_RegisterServices(t *testing.T) {
	k, _, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	newConfigurator := func() module.Configurator {
		msgRouter := baseapp.NewMsgServiceRouter()
		queryRouter := baseapp.NewGRPCQueryRouter()
		msgRouter.SetInterfaceRegistry(encCfg.InterfaceRegistry)
		queryRouter.SetInterfaceRegistry(encCfg.InterfaceRegistry)
		return module.NewConfigurator(cdc, msgRouter, queryRouter)
	}

	cfg := newConfigurator()
	require.NotPanics(t, func() { am.RegisterServices(cfg) })

	// RegisterServices must have registered the v1->v2 migration at
	// fromVersion 1: on a configurator (with fresh, conflict-free routers)
	// that already has a handler registered for that module+version,
	// cfg.RegisterMigration rejects RegisterServices's own registration
	// attempt, which RegisterServices wraps into a panic.
	poisoned := newConfigurator()
	require.NoError(t, poisoned.RegisterMigration(types.ModuleName, 1, func(sdk.Context) error { return nil }))
	func() {
		defer func() {
			r := recover()
			require.NotNil(t, r, "RegisterServices must panic when its migration registration is rejected")
			err, ok := r.(error)
			require.True(t, ok, "panic value must be an error")
			require.ErrorContains(t, err, "already exists")
		}()
		am.RegisterServices(poisoned)
	}()
}

func TestAppModule_RegisterInvariants(t *testing.T) {
	k, _, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	require.NotPanics(t, func() { am.RegisterInvariants(nil) })
}

func TestAppModule_ConsensusVersion(t *testing.T) {
	k, _, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	require.Equal(t, uint64(1), am.ConsensusVersion())
	am.SetConsensusVersion(5)
	require.Equal(t, uint64(5), am.ConsensusVersion())
}

func TestAppModule_MarkerMethods(t *testing.T) {
	k, _, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	require.NotPanics(t, func() {
		am.IsAppModule()
		am.IsOnePerModuleType()
	})
}

func TestAppModule_InitGenesis(t *testing.T) {
	k, ctx, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	addr := sample.RandAccAddress()
	gs := &types.GenesisState{
		Params:                  types.DefaultParams(),
		PaymentAccountCountList: []types.PaymentAccountCount{{Owner: addr.String(), Count: 7}},
	}
	bz := cdc.MustMarshalJSON(gs)

	updates := am.InitGenesis(ctx, cdc, bz)
	require.Empty(t, updates)

	count, found := k.GetPaymentAccountCount(ctx, addr)
	require.True(t, found)
	require.Equal(t, uint64(7), count.Count)
}

func TestAppModule_ExportGenesis(t *testing.T) {
	k, ctx, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	addr := sample.RandAccAddress()
	k.SetPaymentAccountCount(ctx, &types.PaymentAccountCount{Owner: addr.String(), Count: 4})

	bz := am.ExportGenesis(ctx, cdc)

	var gs types.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(bz, &gs))
	require.Len(t, gs.PaymentAccountCountList, 1)
	require.Equal(t, addr.String(), gs.PaymentAccountCountList[0].Owner)
	require.Equal(t, uint64(4), gs.PaymentAccountCountList[0].Count)
}

func TestAppModule_BeginBlock(t *testing.T) {
	k, ctx, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	require.NoError(t, am.BeginBlock(ctx))
}

func TestAppModule_EndBlock(t *testing.T) {
	k, ctx, cdc, bankKeeper, accountKeeper := makeKeeper(t)
	am := payment.NewAppModule(cdc, *k, accountKeeper, bankKeeper)
	ctx = ctx.WithBlockTime(time.Now())

	// AutoResume side: a frozen, zero-rate, zero-out-flow account queued to
	// resume. With no frozen out-flows to walk, AutoResume should resolve it
	// in this single block: flip it back to ACTIVE and drop the queue entry.
	resumeUser := sample.RandAccAddress()
	resumeRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           resumeUser.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		NetflowRate:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.ZeroInt(),
	}
	k.SetStreamRecord(ctx, resumeRecord)
	resumeTs := ctx.BlockTime().Unix()
	k.SetAutoResumeRecord(ctx, &types.AutoResumeRecord{Timestamp: resumeTs, Addr: resumeUser.String()})

	// AutoSettle side: an active account whose only out-flow drains its zero
	// balance and must be force-frozen. This mirrors the fixture proven in
	// keeper/stream_record_test.go's TestAutoSettle_SettleInOneBlock: the
	// natural elapsed-time recompute drives the static balance deeply
	// negative, the bank pull to cover it is made to fail, and only the
	// EndBlocker's forced flag lets the settle go through instead of erroring.
	accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("fail to transfer")).AnyTimes()

	rate := sdkmath.NewInt(100)
	settleUser := sample.RandAccAddress()
	settleUserRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           settleUser.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       rate.Neg(),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		OutFlowCount:      1,
	}
	k.SetStreamRecord(ctx, settleUserRecord)

	gvg := sample.RandAccAddress()
	gvgRecord := &types.StreamRecord{
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		Account:           gvg.String(),
		Status:            types.STREAM_ACCOUNT_STATUS_ACTIVE,
		NetflowRate:       rate,
		FrozenNetflowRate: sdkmath.ZeroInt(),
	}
	k.SetStreamRecord(ctx, gvgRecord)
	k.SetOutFlow(ctx, settleUser, &types.OutFlow{ToAddress: gvg.String(), Rate: rate, Status: types.OUT_FLOW_STATUS_ACTIVE})

	settleTs := ctx.BlockTime().Unix()
	k.SetAutoSettleRecord(ctx, &types.AutoSettleRecord{Timestamp: settleTs, Addr: settleUser.String()})

	// EndBlock is expected to set the force-update flag itself before calling
	// AutoResume/AutoSettle: without it, neither scenario above resolves and
	// both queue entries would remain forever, so this call is the actual
	// wiring under test, not just "doesn't panic".
	require.NoError(t, am.EndBlock(ctx))

	_, resumeStillQueued := k.GetAutoResumeRecord(ctx, resumeTs, resumeUser)
	require.False(t, resumeStillQueued)
	resumed, found := k.GetStreamRecord(ctx, resumeUser)
	require.True(t, found)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_ACTIVE, resumed.Status)

	// The original queued entry must be gone. (UpdateStreamRecord's natural
	// pre-freeze settle-time recompute separately queues its own projected
	// entry for the same address; that one is a same-block artifact that the
	// next block's AutoSettle self-heals once the out-flow is frozen, so it
	// is deliberately not asserted away here.)
	settleStillQueued := false
	for _, r := range k.GetAllAutoSettleRecord(ctx) {
		if r.Addr == settleUser.String() && r.Timestamp == settleTs {
			settleStillQueued = true
		}
	}
	require.False(t, settleStillQueued)
	frozenSettleUser, found := k.GetStreamRecord(ctx, settleUser)
	require.True(t, found)
	require.Equal(t, types.STREAM_ACCOUNT_STATUS_FROZEN, frozenSettleUser.Status)
	frozenOutFlow := k.GetOutFlow(ctx, settleUser, types.OUT_FLOW_STATUS_FROZEN, gvg)
	require.NotNil(t, frozenOutFlow)
	require.Equal(t, types.OUT_FLOW_STATUS_FROZEN, frozenOutFlow.Status)
}
