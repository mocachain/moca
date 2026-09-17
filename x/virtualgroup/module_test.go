package virtualgroup_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/virtualgroup"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func TestAppModuleBasic(t *testing.T) {
	fx := setupFixture(t)
	amb := virtualgroup.NewAppModuleBasic(fx.encCfg.Codec)

	require.Equal(t, types.ModuleName, amb.Name())

	require.NotPanics(t, func() { amb.RegisterLegacyAminoCodec(fx.encCfg.Amino) })
	require.NotPanics(t, func() { amb.RegisterInterfaces(fx.encCfg.InterfaceRegistry) })

	defaultGenesis := amb.DefaultGenesis(fx.encCfg.Codec)
	require.NotNil(t, defaultGenesis)
	var gs types.GenesisState
	require.NoError(t, fx.encCfg.Codec.UnmarshalJSON(defaultGenesis, &gs))
	require.Equal(t, types.DefaultParams(), gs.Params)

	require.NotNil(t, amb.GetTxCmd())
	require.NotNil(t, amb.GetQueryCmd())

	require.NotPanics(t, func() {
		amb.RegisterGRPCGatewayRoutes(client.Context{}, runtime.NewServeMux())
	})
}

func TestAppModuleBasic_ValidateGenesis(t *testing.T) {
	fx := setupFixture(t)
	amb := virtualgroup.NewAppModuleBasic(fx.encCfg.Codec)

	validBz := fx.encCfg.Codec.MustMarshalJSON(types.DefaultGenesis())
	require.NoError(t, amb.ValidateGenesis(fx.encCfg.Codec, nil, validBz))

	err := amb.ValidateGenesis(fx.encCfg.Codec, nil, json.RawMessage("{not-json"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal")

	invalidParamsGenesis := types.GenesisState{Params: types.DefaultParams()}
	invalidParamsGenesis.Params.MaxGlobalVirtualGroupNumPerFamily = 0
	invalidBz := fx.encCfg.Codec.MustMarshalJSON(&invalidParamsGenesis)
	err = amb.ValidateGenesis(fx.encCfg.Codec, nil, invalidBz)
	require.Error(t, err)
	require.Contains(t, err.Error(), "max GVG per family must be positive")
}

func TestNewAppModule(t *testing.T) {
	fx := setupFixture(t)
	am := virtualgroup.NewAppModule(fx.encCfg.Codec, *fx.keeper, fx.spKeeper)
	require.NotNil(t, am)

	require.Equal(t, uint64(1), am.ConsensusVersion())
	am.SetConsensusVersion(2)
	require.Equal(t, uint64(2), am.ConsensusVersion())

	require.NoError(t, am.BeginBlock(context.Background()))
	require.NoError(t, am.EndBlock(context.Background()))

	require.NotPanics(t, func() { am.RegisterInvariants(nil) })

	// IsAppModule/IsOnePerModuleType are marker methods with empty bodies;
	// calling them exercises the statements without needing any assertion.
	am.IsAppModule()
	am.IsOnePerModuleType()
}

func TestAppModule_InitExportGenesis(t *testing.T) {
	fx := setupFixture(t)
	am := virtualgroup.NewAppModule(fx.encCfg.Codec, *fx.keeper, fx.spKeeper)

	genesisState := types.GenesisState{Params: types.DefaultParams()}
	gsBz := fx.encCfg.Codec.MustMarshalJSON(&genesisState)

	expectEmptyGenesisAccount(fx, genesisState.Params.DepositDenom)

	updates := am.InitGenesis(fx.ctx, fx.encCfg.Codec, gsBz)
	require.Empty(t, updates)

	exportedBz := am.ExportGenesis(fx.ctx, fx.encCfg.Codec)
	var exported types.GenesisState
	require.NoError(t, fx.encCfg.Codec.UnmarshalJSON(exportedBz, &exported))
	require.Equal(t, genesisState.Params, exported.Params)
}

// TestAppModule_RegisterServices wires up real MsgServiceRouter/GRPCQueryRouter
// instances (as app.go does) and asserts RegisterServices actually routes the
// module's Msg and Query services through them.
func TestAppModule_RegisterServices(t *testing.T) {
	fx := setupFixture(t)
	amb := virtualgroup.NewAppModuleBasic(fx.encCfg.Codec)
	amb.RegisterInterfaces(fx.encCfg.InterfaceRegistry)

	am := virtualgroup.NewAppModule(fx.encCfg.Codec, *fx.keeper, fx.spKeeper)

	msr := baseapp.NewMsgServiceRouter()
	msr.SetInterfaceRegistry(fx.encCfg.InterfaceRegistry)
	qr := baseapp.NewGRPCQueryRouter()
	qr.SetInterfaceRegistry(fx.encCfg.InterfaceRegistry)

	cfg := module.NewConfigurator(fx.encCfg.Codec, msr, qr)

	require.NotPanics(t, func() { am.RegisterServices(cfg) })
	require.NoError(t, cfg.Error())

	require.NotNil(t, msr.Handler(&types.MsgSettle{}), "MsgSettle should be routed after RegisterServices")
	require.NotNil(t, qr.Route("/moca.virtualgroup.Query/Params"), "Query/Params should be routed after RegisterServices")
}

// TestAppModule_RegisterServices_MigrationConflictPanics covers the
// RegisterServices panic branch: if the v1->v2 migration slot is already
// taken, cfg.RegisterMigration errors and RegisterServices must panic rather
// than silently drop the migration.
func TestAppModule_RegisterServices_MigrationConflictPanics(t *testing.T) {
	fx := setupFixture(t)
	amb := virtualgroup.NewAppModuleBasic(fx.encCfg.Codec)
	amb.RegisterInterfaces(fx.encCfg.InterfaceRegistry)

	am := virtualgroup.NewAppModule(fx.encCfg.Codec, *fx.keeper, fx.spKeeper)

	msr := baseapp.NewMsgServiceRouter()
	msr.SetInterfaceRegistry(fx.encCfg.InterfaceRegistry)
	qr := baseapp.NewGRPCQueryRouter()
	qr.SetInterfaceRegistry(fx.encCfg.InterfaceRegistry)
	cfg := module.NewConfigurator(fx.encCfg.Codec, msr, qr)

	// Occupy the (module, fromVersion=1) migration slot before
	// RegisterServices gets a chance to, so its own RegisterMigration call
	// collides and returns an error.
	require.NoError(t, cfg.RegisterMigration(types.ModuleName, 1, func(sdk.Context) error { return nil }))

	require.Panics(t, func() { am.RegisterServices(cfg) })
}
