package permission_test

import (
	"encoding/json"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/permission"
	"github.com/mocachain/moca/v2/x/permission/types"
)

func TestAppModuleSimulation(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	k, _ := makeKeeper(t)
	am := permission.NewAppModule(encCfg.Codec, *k, &types.MockAccountKeeper{}, &types.MockBankKeeper{})

	t.Run("GenerateGenesisState", func(t *testing.T) {
		simState := &module.SimulationState{
			Accounts: []simtypes.Account{{Address: sample.RandAccAddress()}},
			Cdc:      encCfg.Codec,
			GenState: map[string]json.RawMessage{},
		}
		am.GenerateGenesisState(simState)

		var got types.GenesisState
		encCfg.Codec.MustUnmarshalJSON(simState.GenState[types.ModuleName], &got)
		require.Equal(t, *types.DefaultGenesis(), got)
	})

	t.Run("ProposalContents", func(t *testing.T) {
		require.Nil(t, am.ProposalContents(module.SimulationState{}))
	})

	t.Run("RegisterStoreDecoder", func(t *testing.T) {
		require.NotPanics(t, func() { am.RegisterStoreDecoder(nil) })
	})

	t.Run("WeightedOperations", func(t *testing.T) {
		require.Empty(t, am.WeightedOperations(module.SimulationState{}))
	})
}
