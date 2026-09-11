package permission_test

import (
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/x/permission"
	"github.com/mocachain/moca/v2/x/permission/types"
)

func TestAppModuleBasic(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	amb := permission.NewAppModuleBasic(encCfg.Codec)

	require.Equal(t, types.ModuleName, amb.Name())

	require.NotPanics(t, func() { amb.RegisterLegacyAminoCodec(encCfg.Amino) })
	require.NotPanics(t, func() { amb.RegisterInterfaces(cdctypes.NewInterfaceRegistry()) })

	bz := amb.DefaultGenesis(encCfg.Codec)
	var defaultGenesis types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(bz, &defaultGenesis))
	require.Equal(t, *types.DefaultGenesis(), defaultGenesis)

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
	amb := permission.NewAppModuleBasic(encCfg.Codec)

	invalidParams := types.DefaultGenesis()
	invalidParams.Params.MaximumStatementsNum = 0

	testCases := []struct {
		name      string
		bz        json.RawMessage
		expectErr bool
	}{
		{
			name:      "valid genesis",
			bz:        encCfg.Codec.MustMarshalJSON(types.DefaultGenesis()),
			expectErr: false,
		},
		{
			name:      "invalid json",
			bz:        json.RawMessage(`{invalid`),
			expectErr: true,
		},
		{
			name:      "genesis fails Params.Validate",
			bz:        encCfg.Codec.MustMarshalJSON(invalidParams),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := amb.ValidateGenesis(encCfg.Codec, nil, tc.bz)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAppModule(t *testing.T) {
	// RegisterServices below requires permission's own Msg types (MsgUpdateParams) to
	// already be in the interface registry, so register the module's own AppModuleBasic
	// alongside mint's (mint keeps parity with the rest of the package's fixtures).
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{}, permission.AppModuleBasic{})
	k, ctx := makeKeeper(t)
	am := permission.NewAppModule(encCfg.Codec, *k, &types.MockAccountKeeper{}, &types.MockBankKeeper{})

	require.Equal(t, uint64(1), am.ConsensusVersion())

	require.NoError(t, am.BeginBlock(ctx))
	require.NoError(t, am.EndBlock(ctx))

	am.IsAppModule()
	am.IsOnePerModuleType()
	am.RegisterInvariants(nil)

	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(encCfg.InterfaceRegistry)
	queryHelper := baseapp.NewQueryServerTestHelper(ctx, encCfg.InterfaceRegistry)
	cfg := module.NewConfigurator(encCfg.Codec, msgRouter, queryHelper)
	require.NotPanics(t, func() { am.RegisterServices(cfg) })

	genesisState := types.GenesisState{Params: types.DefaultParams()}
	bz := encCfg.Codec.MustMarshalJSON(&genesisState)
	updates := am.InitGenesis(ctx, encCfg.Codec, bz)
	require.Empty(t, updates)

	exportedBz := am.ExportGenesis(ctx, encCfg.Codec)
	var exported types.GenesisState
	encCfg.Codec.MustUnmarshalJSON(exportedBz, &exported)
	require.Equal(t, genesisState.Params, exported.Params)
}
