package gensp_test

import (
	"context"
	"encoding/json"
	"testing"

	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltestutil "github.com/cosmos/cosmos-sdk/x/genutil/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/gensp"
	gensptypes "github.com/mocachain/moca/v2/x/gensp/types"
)

// TestNewAppModuleBasic covers the stateless AppModuleBasic constructor and
// its simple accessors, which need no fixtures beyond a bare codec.
func TestNewAppModuleBasic(t *testing.T) {
	cdc := codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	amb := gensp.NewAppModuleBasic(cdc)

	require.Equal(t, gensptypes.ModuleName, amb.Name())
	require.Nil(t, amb.GetTxCmd())
	require.Nil(t, amb.GetQueryCmd())

	require.NotPanics(t, func() { amb.RegisterLegacyAminoCodec(codec.NewLegacyAmino()) })
	require.NotPanics(t, func() { amb.RegisterInterfaces(cdctypes.NewInterfaceRegistry()) })

	// RegisterGRPCGatewayRoutes is a documented no-op; call it directly for
	// coverage rather than wrapping a call that can never panic.
	amb.RegisterGRPCGatewayRoutes(client.Context{}, nil)
}

// TestAppModuleBasic_DefaultGenesis asserts the default genesis bytes decode
// back into the module's own default state.
func TestAppModuleBasic_DefaultGenesis(t *testing.T) {
	cdc := codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	amb := gensp.NewAppModuleBasic(cdc)

	bz := amb.DefaultGenesis(cdc)

	var genState gensptypes.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(bz, &genState))
	require.Equal(t, *gensptypes.DefaultGenesisState(), genState)
}

// TestAppModuleBasic_ValidateGenesis covers the outer unmarshal-error branch
// (module.go:65) and the propagated types.ValidateGenesis error branch
// (module.go:68), alongside the happy path.
func TestAppModuleBasic_ValidateGenesis(t *testing.T) {
	encodingCfg := moduletestutil.MakeTestEncodingConfig(genutil.AppModuleBasic{})
	cdc := encodingCfg.Codec
	amb := gensp.NewAppModuleBasic(cdc)

	testCases := []struct {
		name    string
		bz      json.RawMessage
		wantErr bool
	}{
		{
			"valid empty genesis",
			cdc.MustMarshalJSON(gensptypes.DefaultGenesisState()),
			false,
		},
		{
			"malformed genesis json",
			json.RawMessage("not json"),
			true,
		},
		{
			"genesis with an undecodable gentx",
			cdc.MustMarshalJSON(&gensptypes.GenesisState{
				GenspTxs: []json.RawMessage{[]byte(`"not-a-tx"`)},
			}),
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := amb.ValidateGenesis(cdc, encodingCfg.TxConfig, tc.bz)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestAppModule_ZeroValueMethods covers the AppModule methods that are
// shadowed by module.GenesisOnlyAppModule and therefore unreachable through
// NewAppModule (RegisterInvariants, ConsensusVersion, IsAppModule,
// IsOnePerModuleType), plus BeginBlock/EndBlock which aren't promoted at
// all. All six touch no fields, so a zero-value AppModule{} is sufficient.
func TestAppModule_ZeroValueMethods(t *testing.T) {
	am := gensp.AppModule{}

	am.RegisterInvariants(nil)
	require.Equal(t, uint64(1), am.ConsensusVersion())
	require.NoError(t, am.BeginBlock(context.Background()))
	require.NoError(t, am.EndBlock(context.Background()))
	am.IsAppModule()
	am.IsOnePerModuleType()
}

// TestNewAppModule_InitGenesisAndExportGenesis drives NewAppModule's
// InitGenesis/ExportGenesis through the module.HasABCIGenesis interface,
// which module.GenesisOnlyAppModule promotes unshadowed - the only way to
// reach a *real* AppModule's genesis methods through the constructor.
func TestNewAppModule_InitGenesisAndExportGenesis(t *testing.T) {
	encodingCfg := moduletestutil.MakeTestEncodingConfig(genutil.AppModuleBasic{})
	banktypes.RegisterInterfaces(encodingCfg.InterfaceRegistry)
	cdc := encodingCfg.Codec

	key := storetypes.NewKVStoreKey("gensp_module_test")
	tkey := storetypes.NewTransientStoreKey("gensp_module_test_transient")
	ctx := testutil.DefaultContext(key, tkey)

	// newSendTxJSON builds an unsigned MsgSend genTx: DeliverGenTxs' mocked
	// ExecuteGenesisTx never checks signatures, and decode/encode only need
	// a message type registered in the interface registry.
	newSendTxJSON := func(t *testing.T) json.RawMessage {
		msg := banktypes.NewMsgSend(
			sample.RandAccAddress(), sample.RandAccAddress(),
			sdk.Coins{sdk.NewInt64Coin(sdk.DefaultBondDenom, 1)},
		)
		txBuilder := encodingCfg.TxConfig.NewTxBuilder()
		require.NoError(t, txBuilder.SetMsgs(msg))
		bz, err := encodingCfg.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
		require.NoError(t, err)
		return bz
	}

	t.Run("empty genesis performs no staking keeper call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		stakingKeeper := genutiltestutil.NewMockStakingKeeper(ctrl)

		am := gensp.NewAppModule(nil, stakingKeeper, &mockTxHandler{}, encodingCfg.TxConfig)
		abciGen, ok := am.(module.HasABCIGenesis)
		require.True(t, ok)

		gs := cdc.MustMarshalJSON(gensptypes.DefaultGenesisState())
		updates := abciGen.InitGenesis(ctx, cdc, gs)
		require.Empty(t, updates)

		exported := abciGen.ExportGenesis(ctx, cdc)
		var genState gensptypes.GenesisState
		require.NoError(t, cdc.UnmarshalJSON(exported, &genState))
		require.Equal(t, *gensptypes.DefaultGenesisState(), genState)
	})

	t.Run("non-empty genesis delivers the gentx and returns validator updates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		stakingKeeper := genutiltestutil.NewMockStakingKeeper(ctrl)
		stakingKeeper.EXPECT().
			ApplyAndReturnValidatorSetUpdates(gomock.Any()).
			Return([]abci.ValidatorUpdate{{Power: 7}}, nil)

		am := gensp.NewAppModule(nil, stakingKeeper, &mockTxHandler{errCode: 0}, encodingCfg.TxConfig)
		abciGen, ok := am.(module.HasABCIGenesis)
		require.True(t, ok)

		gs := cdc.MustMarshalJSON(&gensptypes.GenesisState{
			GenspTxs: []json.RawMessage{newSendTxJSON(t)},
		})
		updates := abciGen.InitGenesis(ctx, cdc, gs)
		require.Equal(t, []abci.ValidatorUpdate{{Power: 7}}, updates)
	})

	t.Run("a gentx that fails to deliver panics", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		stakingKeeper := genutiltestutil.NewMockStakingKeeper(ctrl)

		am := gensp.NewAppModule(nil, stakingKeeper, &mockTxHandler{}, encodingCfg.TxConfig)
		abciGen, ok := am.(module.HasABCIGenesis)
		require.True(t, ok)

		gs := cdc.MustMarshalJSON(&gensptypes.GenesisState{
			GenspTxs: []json.RawMessage{[]byte(`"not-a-tx"`)},
		})
		require.Panics(t, func() { abciGen.InitGenesis(ctx, cdc, gs) })
	})
}
