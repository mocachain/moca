package sp_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/x/sp"
	"github.com/mocachain/moca/v2/x/sp/types"
)

// TestAppModule_GenerateGenesisState covers module_simulation.go's only
// stateful method: whatever simulation accounts are supplied, it must emit a
// GenesisState with default Params under the module's own genesis key.
func TestAppModule_GenerateGenesisState(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig()
	accs := simtypes.RandomAccounts(rand.New(rand.NewSource(1)), 2) //nolint:gosec // deterministic seed for a reproducible sim fixture, not crypto use

	simState := &module.SimulationState{
		Accounts: accs,
		Cdc:      encCfg.Codec,
		GenState: make(map[string]json.RawMessage),
	}

	sp.AppModule{}.GenerateGenesisState(simState)

	bz, ok := simState.GenState[types.ModuleName]
	require.True(t, ok, "GenerateGenesisState must populate the sp genesis key")

	var gs types.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(bz, &gs))
	require.Equal(t, types.DefaultParams(), gs.Params)
}

func TestAppModule_ProposalContents(t *testing.T) {
	require.Nil(t, sp.AppModule{}.ProposalContents(module.SimulationState{}))
}

func TestAppModule_RegisterStoreDecoder(t *testing.T) {
	require.NotPanics(t, func() {
		sp.AppModule{}.RegisterStoreDecoder(nil)
	})
}

func TestAppModule_WeightedOperations(t *testing.T) {
	ops := sp.AppModule{}.WeightedOperations(module.SimulationState{})
	require.NotNil(t, ops)
	require.Empty(t, ops)
}
