package storage_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/storage"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
)

// newAppModule builds a real AppModule wired to k, mirroring how app.go
// constructs it in production. The account/bank/sp keepers are never invoked
// by the methods under test here, so zero-value mocks are enough (same trick
// genesis_test.go's makeKeeper already relies on for the keeper's own deps).
func newAppModule(k keeper.Keeper, cdc codec.Codec) storage.AppModule {
	return storage.NewAppModule(cdc, k, &types.MockAccountKeeper{}, &types.MockBankKeeper{}, &types.MockSpKeeper{})
}

func TestAppModuleBasic_Name(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)

	require.Equal(t, types.ModuleName, a.Name())
}

func TestAppModuleBasic_RegisterLegacyAminoCodec(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)
	legacyAmino := codec.NewLegacyAmino()

	a.RegisterLegacyAminoCodec(legacyAmino)

	bz, err := legacyAmino.MarshalJSON(&types.MsgCreateBucket{})
	require.NoError(t, err)
	require.Contains(t, string(bz), "storage/CreateBucket")
}

func TestAppModuleBasic_RegisterInterfaces(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)
	reg := cdctypes.NewInterfaceRegistry()

	a.RegisterInterfaces(reg)

	require.NoError(t, reg.EnsureRegistered(&types.MsgCreateBucket{}))
	require.NoError(t, reg.EnsureRegistered(&types.MsgUpdateParams{}))
}

func TestAppModuleBasic_DefaultGenesis(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)

	bz := a.DefaultGenesis(encCfg.Codec)

	var gs types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(bz, &gs))
	require.Equal(t, types.DefaultParams(), gs.Params)
}

func TestAppModuleBasic_ValidateGenesis(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)

	validBz := encCfg.Codec.MustMarshalJSON(types.DefaultGenesis())
	invalidParamsBz := encCfg.Codec.MustMarshalJSON(&types.GenesisState{})

	for _, tc := range []struct {
		name    string
		bz      []byte
		wantErr bool
	}{
		{"valid default genesis", validBz, false},
		{"malformed json", []byte("{not-json"), true},
		{"well-formed json but invalid params", invalidParamsBz, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := a.ValidateGenesis(encCfg.Codec, nil, tc.bz)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAppModuleBasic_RegisterGRPCGatewayRoutes(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)
	mux := runtime.NewServeMux()

	require.NotPanics(t, func() {
		a.RegisterGRPCGatewayRoutes(client.Context{}, mux)
	})
}

func TestAppModuleBasic_GetTxCmd(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)

	cmd := a.GetTxCmd()

	require.Equal(t, types.ModuleName, cmd.Use)
	require.NotEmpty(t, cmd.Commands())
}

func TestAppModuleBasic_GetQueryCmd(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	a := storage.NewAppModuleBasic(encCfg.Codec)

	cmd := a.GetQueryCmd()

	require.Equal(t, types.ModuleName, cmd.Use)
	require.NotEmpty(t, cmd.Commands())
}

func TestNewAppModule(t *testing.T) {
	k, _ := makeKeeper(t)
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})

	am := newAppModule(*k, encCfg.Codec)

	require.Equal(t, types.ModuleName, am.Name())
	require.Equal(t, uint64(2), am.ConsensusVersion())
	require.NotPanics(t, func() {
		am.IsAppModule()
		am.IsOnePerModuleType()
		am.RegisterInvariants(nil)
	})
}

func TestAppModule_RegisterServices(t *testing.T) {
	k, _ := makeKeeper(t)
	// Mirrors app.go: every module's RegisterInterfaces (storage's own included)
	// runs against the shared registry before RegisterServices is ever called,
	// which is what lets MsgServiceRouter resolve the storage Msg type URLs
	// registered via msgservice.RegisterMsgServiceDesc in codec.go.
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{}, storage.NewAppModuleBasic(nil))
	am := newAppModule(*k, encCfg.Codec)

	msr := baseapp.NewMsgServiceRouter()
	msr.SetInterfaceRegistry(encCfg.InterfaceRegistry)
	qr := baseapp.NewGRPCQueryRouter()
	qr.SetInterfaceRegistry(encCfg.InterfaceRegistry)
	cfg := module.NewConfigurator(encCfg.Codec, msr, qr)

	require.NotPanics(t, func() {
		am.RegisterServices(cfg)
	})

	// A real observable effect of RegisterServices: both routers now resolve
	// a handler for the module's Msg/Query services.
	require.NotNil(t, msr.Handler(&types.MsgCreateBucket{}))
	require.NotNil(t, qr.Route("/moca.storage.Query/Params"))
}

func TestAppModule_InitGenesis(t *testing.T) {
	k, ctx := makeKeeper(t)
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	am := newAppModule(*k, encCfg.Codec)

	genState := types.GenesisState{Params: types.DefaultParams()}
	bz := encCfg.Codec.MustMarshalJSON(&genState)

	updates := am.InitGenesis(ctx, encCfg.Codec, bz)

	require.Empty(t, updates)
	require.Equal(t, types.DefaultParams(), k.GetParams(ctx))
}

func TestAppModule_ExportGenesis(t *testing.T) {
	k, ctx := makeKeeper(t)
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	am := newAppModule(*k, encCfg.Codec)

	bz := am.ExportGenesis(ctx, encCfg.Codec)

	var got types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(bz, &got))
	require.Equal(t, types.DefaultParams(), got.Params)
}

func TestAppModule_BeginBlock(t *testing.T) {
	k, ctx := makeKeeper(t)
	params := types.DefaultParams()
	params.DiscontinueCountingWindow = 5
	require.NoError(t, k.SetParams(ctx, params))
	ctx = ctx.WithBlockHeight(5)

	// Seed a counter that BeginBlocker only clears when blockHeight is a
	// positive multiple of DiscontinueCountingWindow, so a passing test
	// proves AppModule.BeginBlock actually delegated to keeper.BeginBlocker
	// rather than being a no-op wrapper.
	addr := sample.RandAccAddress()
	k.SetDiscontinueObjectCount(ctx, addr, 3)

	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	am := newAppModule(*k, encCfg.Codec)

	require.NoError(t, am.BeginBlock(ctx))
	require.Equal(t, uint64(0), k.GetDiscontinueObjectCount(ctx, addr))
}

func TestAppModule_EndBlock(t *testing.T) {
	// Deliberately do not SetParams: DiscontinueDeletionMax must be positive
	// once set (Validate() rejects 0), so the only way to observe
	// EndBlocker's deletionMax==0 early return is an untouched param store,
	// which GetParams reads back as a zero-value Params. The discontinue-
	// deletion sweep and payment check beyond that point are x/storage/
	// keeper's own logic (covered by that package's own tests); this test
	// only proves AppModule.EndBlock delegates and surfaces the result.
	k, ctx := makeKeeper(t)

	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	am := newAppModule(*k, encCfg.Codec)

	require.NoError(t, am.EndBlock(ctx))
}
