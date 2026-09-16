package storage_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/x/storage"
	"github.com/mocachain/moca/v2/x/storage/types"
)

func TestAppModule_GenerateGenesisState(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	r := rand.New(rand.NewSource(1))
	accs := simtypes.RandomAccounts(r, 3)

	simState := &module.SimulationState{
		Cdc:      encCfg.Codec,
		Accounts: accs,
		GenState: make(map[string]json.RawMessage),
	}

	storage.AppModule{}.GenerateGenesisState(simState)

	raw, ok := simState.GenState[types.ModuleName]
	require.True(t, ok)

	var got types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(raw, &got))
	require.Equal(t, types.DefaultParams(), got.Params)
}

func TestAppModule_ProposalContents(t *testing.T) {
	result := storage.AppModule{}.ProposalContents(module.SimulationState{})

	require.Nil(t, result)
}

func TestAppModule_RegisterStoreDecoder(t *testing.T) {
	registry := make(simtypes.StoreDecoderRegistry)

	require.NotPanics(t, func() {
		storage.AppModule{}.RegisterStoreDecoder(registry)
	})
	require.Empty(t, registry)
}

func TestAppModule_WeightedOperations(t *testing.T) {
	ops := storage.AppModule{}.WeightedOperations(module.SimulationState{})

	require.Empty(t, ops)
}
