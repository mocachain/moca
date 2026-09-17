package challenge_test

import (
	"encoding/json"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/mocachain/moca/v2/x/challenge/types"
)

func (s *TestSuite) TestAppModule_GenerateGenesisState() {
	am := newTestAppModule(s)

	r := rand.New(rand.NewSource(1)) //nolint:gosec
	simState := &module.SimulationState{
		Accounts: simtypes.RandomAccounts(r, 3),
		Cdc:      s.cdc,
		GenState: make(map[string]json.RawMessage),
	}

	am.GenerateGenesisState(simState)

	bz, ok := simState.GenState[types.ModuleName]
	s.Require().True(ok, "GenerateGenesisState must populate GenState for the module")

	var gs types.GenesisState
	s.Require().NoError(s.cdc.UnmarshalJSON(bz, &gs))
	s.Require().Equal(types.DefaultParams(), gs.Params)
}

func (s *TestSuite) TestAppModule_ProposalContents() {
	am := newTestAppModule(s)

	s.Require().Nil(am.ProposalContents(module.SimulationState{}))
}

func (s *TestSuite) TestAppModule_RegisterStoreDecoder() {
	am := newTestAppModule(s)

	s.Require().NotPanics(func() {
		am.RegisterStoreDecoder(simtypes.StoreDecoderRegistry{})
	})
}

// TestAppModule_WeightedOperations asserts the shape of the two weighted
// operations (Submit, Attest) and that their weights resolve through the
// GetOrGenerate default-value closures when AppParams carries no override.
// The operations' own runtime behavior is covered by the simulation
// package's own tests (submit_test.go / attest_test.go).
func (s *TestSuite) TestAppModule_WeightedOperations() {
	am := newTestAppModule(s)

	ops := am.WeightedOperations(module.SimulationState{AppParams: make(simtypes.AppParams)})
	s.Require().Len(ops, 2)
	for _, op := range ops {
		s.Require().Equal(100, op.Weight())
		s.Require().NotNil(op.Op())
	}
}
