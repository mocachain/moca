package payment_test

import (
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestAppModule_GenerateGenesisState(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})

	simState := &module.SimulationState{
		Accounts: []simtypes.Account{{Address: sample.RandAccAddress()}},
		Cdc:      encCfg.Codec,
		GenState: map[string]json.RawMessage{},
	}

	payment.AppModule{}.GenerateGenesisState(simState)

	raw, ok := simState.GenState[types.ModuleName]
	require.True(t, ok)

	var gs types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(raw, &gs))
	require.Equal(t, types.DefaultParams(), gs.Params)
}

func TestAppModule_ProposalContents(t *testing.T) {
	require.Nil(t, payment.AppModule{}.ProposalContents(module.SimulationState{}))
}

func TestAppModule_RegisterStoreDecoder(t *testing.T) {
	registry := make(simtypes.StoreDecoderRegistry)

	require.NotPanics(t, func() { payment.AppModule{}.RegisterStoreDecoder(registry) })
	require.Empty(t, registry)
}

func TestAppModule_WeightedOperations(t *testing.T) {
	ops := payment.AppModule{}.WeightedOperations(module.SimulationState{})

	require.NotNil(t, ops)
	require.Empty(t, ops)
}
