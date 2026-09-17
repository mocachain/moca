package virtualgroup_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/virtualgroup"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func TestAppModule_GenerateGenesisState(t *testing.T) {
	fx := setupFixture(t)
	am := virtualgroup.NewAppModule(fx.encCfg.Codec, *fx.keeper, fx.spKeeper)

	r := rand.New(rand.NewSource(1)) //nolint:gosec
	simState := &module.SimulationState{
		Accounts: simtypes.RandomAccounts(r, 3),
		Cdc:      fx.encCfg.Codec,
		GenState: make(map[string]json.RawMessage),
	}

	am.GenerateGenesisState(simState)

	bz, ok := simState.GenState[types.ModuleName]
	require.True(t, ok, "GenerateGenesisState must populate GenState for the module")

	var gs types.GenesisState
	require.NoError(t, fx.encCfg.Codec.UnmarshalJSON(bz, &gs))
	require.Equal(t, types.DefaultParams(), gs.Params)
}

func TestAppModule_RegisterStoreDecoder(t *testing.T) {
	am := virtualgroup.AppModule{}
	require.NotPanics(t, func() {
		am.RegisterStoreDecoder(simtypes.StoreDecoderRegistry{})
	})
}

func TestAppModule_ProposalContents(t *testing.T) {
	am := virtualgroup.AppModule{}
	require.Nil(t, am.ProposalContents(module.SimulationState{}))
}

func TestAppModule_WeightedOperations(t *testing.T) {
	fx := setupFixture(t)
	am := virtualgroup.NewAppModule(fx.encCfg.Codec, *fx.keeper, fx.spKeeper)

	// An empty AppParams map makes GetOrGenerate always take the
	// default-value closure, so the resulting weights are deterministic.
	ops := am.WeightedOperations(module.SimulationState{AppParams: simtypes.AppParams{}})
	require.Len(t, ops, 4)
	for _, op := range ops {
		require.Equal(t, 100, op.Weight())
		require.NotNil(t, op.Op())
	}
}

func TestAppModule_ProposalMsgs(t *testing.T) {
	fx := setupFixture(t)
	am := virtualgroup.NewAppModule(fx.encCfg.Codec, *fx.keeper, fx.spKeeper)

	msgs := am.ProposalMsgs(module.SimulationState{})
	require.Len(t, msgs, 4)

	wantKeys := []string{
		"op_weight_msg_storage_provider_exit",
		"op_weight_msg_complete_storage_provider_exit",
		"op_weight_msg_complete_swap_out",
		"op_weight_msg_cancel_swap_out",
	}
	r := rand.New(rand.NewSource(1)) //nolint:gosec
	for i, wpm := range msgs {
		require.Equal(t, wantKeys[i], wpm.AppParamsKey())
		require.Equal(t, 100, wpm.DefaultWeight())
		// The inline closures ignore all of their arguments and always
		// return nil; calling them exercises those statements too.
		require.Nil(t, wpm.MsgSimulatorFn()(r, sdk.Context{}, nil))
	}
}
