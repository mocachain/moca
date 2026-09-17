package sp_test

import (
	"encoding/json"
	"testing"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/x/sp"
	"github.com/mocachain/moca/v2/x/sp/keeper"
	"github.com/mocachain/moca/v2/x/sp/types"
)

// makeModuleKeeper builds a standalone sp keeper backed by an in-memory
// store, the same way abci_test.go's TestSuite.SetupTest does. It is
// duplicated here (rather than shared) because this file is the only one in
// the package that needs a keeper wired up for AppModule-level tests.
func makeModuleKeeper(t *testing.T) (*keeper.Keeper, sdk.Context, codec.Codec, *types.MockAccountKeeper, *types.MockBankKeeper) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{}, sp.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	ctrl := gomock.NewController(t)
	accountKeeper := types.NewMockAccountKeeper(ctrl)
	bankKeeper := types.NewMockBankKeeper(ctrl)
	authzKeeper := types.NewMockAuthzKeeper(ctrl)

	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		accountKeeper,
		bankKeeper,
		authzKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	require.NoError(t, k.SetParams(testCtx.Ctx, types.DefaultParams()))

	return k, testCtx.Ctx, encCfg.Codec, accountKeeper, bankKeeper
}

func TestAppModuleBasic(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	amb := sp.NewAppModuleBasic(encCfg.Codec)

	require.Equal(t, types.ModuleName, amb.Name())

	require.NotPanics(t, func() { amb.RegisterLegacyAminoCodec(encCfg.Amino) })
	require.NotPanics(t, func() { amb.RegisterInterfaces(encCfg.InterfaceRegistry) })

	bz := amb.DefaultGenesis(encCfg.Codec)
	var defaultGenesis types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(bz, &defaultGenesis))
	require.Equal(t, types.DefaultParams(), defaultGenesis.Params)
	require.Empty(t, defaultGenesis.StorageProviders)
	require.Empty(t, defaultGenesis.SpStoragePriceList)

	require.NotPanics(t, func() {
		amb.RegisterGRPCGatewayRoutes(client.Context{}, runtime.NewServeMux())
	})

	txCmd := amb.GetTxCmd()
	require.NotNil(t, txCmd)
	require.Equal(t, types.ModuleName, txCmd.Use)

	queryCmd := amb.GetQueryCmd()
	require.NotNil(t, queryCmd)
	require.Equal(t, types.ModuleName, queryCmd.Use)
}

func TestAppModuleBasic_ValidateGenesis(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	amb := sp.NewAppModuleBasic(encCfg.Codec)

	invalidParamsGenesis := types.DefaultGenesis()
	invalidParamsGenesis.Params.DepositDenom = ""

	testCases := []struct {
		name      string
		bz        json.RawMessage
		expectErr string
	}{
		{
			name: "valid genesis",
			bz:   encCfg.Codec.MustMarshalJSON(types.DefaultGenesis()),
		},
		{
			name:      "invalid json",
			bz:        json.RawMessage(`{invalid`),
			expectErr: "failed to unmarshal",
		},
		{
			name:      "genesis fails Params.Validate",
			bz:        encCfg.Codec.MustMarshalJSON(invalidParamsGenesis),
			expectErr: "deposit denom cannot be blank",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := amb.ValidateGenesis(encCfg.Codec, nil, tc.bz)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

// TestAppModule_RegisterServices wires up real MsgServiceRouter/GRPCQueryRouter
// instances (as app.go does) and asserts RegisterServices actually routes the
// module's Msg and Query services through them.
func TestAppModule_RegisterServices(t *testing.T) {
	k, _, cdc, accountKeeper, bankKeeper := makeModuleKeeper(t)
	am := sp.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{}, sp.AppModuleBasic{})
	msgRouter := baseapp.NewMsgServiceRouter()
	queryRouter := baseapp.NewGRPCQueryRouter()
	msgRouter.SetInterfaceRegistry(encCfg.InterfaceRegistry)
	queryRouter.SetInterfaceRegistry(encCfg.InterfaceRegistry)
	cfg := module.NewConfigurator(cdc, msgRouter, queryRouter)

	require.NotPanics(t, func() { am.RegisterServices(cfg) })
	require.NoError(t, cfg.Error())

	require.NotNil(t, msgRouter.Handler(&types.MsgDeposit{}), "MsgDeposit should be routed after RegisterServices")
	require.NotNil(t, queryRouter.Route("/moca.sp.Query/Params"), "Query/Params should be routed after RegisterServices")
}

func TestAppModule_RegisterInvariants(t *testing.T) {
	k, _, cdc, accountKeeper, bankKeeper := makeModuleKeeper(t)
	am := sp.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	require.NotPanics(t, func() { am.RegisterInvariants(nil) })
}

func TestAppModule_ConsensusVersion(t *testing.T) {
	k, _, cdc, accountKeeper, bankKeeper := makeModuleKeeper(t)
	am := sp.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	require.Equal(t, uint64(1), am.ConsensusVersion())
}

func TestAppModule_MarkerMethods(t *testing.T) {
	k, ctx, cdc, accountKeeper, bankKeeper := makeModuleKeeper(t)
	am := sp.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	require.NoError(t, am.BeginBlock(ctx))
	require.NoError(t, am.EndBlock(ctx))

	require.NotPanics(t, func() {
		am.IsAppModule()
		am.IsOnePerModuleType()
	})
}

// TestAppModule_InitExportGenesis drives InitGenesis/ExportGenesis through the
// AppModule wrapper with an empty-StorageProviders genesis, mirroring the
// zero-balance deposit-pool path keeper/genesis_test.go and the root
// package's own genesis_test.go already exercise in more depth; this test's
// job is only to prove the (un)marshal wiring in module.go itself.
func TestAppModule_InitExportGenesis(t *testing.T) {
	k, ctx, cdc, accountKeeper, bankKeeper := makeModuleKeeper(t)
	am := sp.NewAppModule(cdc, *k, accountKeeper, bankKeeper)

	genesisState := &types.GenesisState{Params: types.DefaultParams()}
	bz := cdc.MustMarshalJSON(genesisState)

	accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), types.ModuleName).
		Return(authtypes.NewEmptyModuleAccount(types.ModuleName))
	accountKeeper.EXPECT().SetModuleAccount(gomock.Any(), gomock.Any()).Return()
	bankKeeper.EXPECT().GetAllBalances(gomock.Any(), gomock.Any()).
		Return(sdk.NewCoins(sdk.NewCoin(genesisState.Params.DepositDenom, sdkmath.ZeroInt())))

	updates := am.InitGenesis(ctx, cdc, bz)
	require.Empty(t, updates)

	exportedBz := am.ExportGenesis(ctx, cdc)
	var exported types.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(exportedBz, &exported))
	require.Equal(t, genesisState.Params, exported.Params)
}
