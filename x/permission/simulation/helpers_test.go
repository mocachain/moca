package simulation_test

import (
	"testing"

	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/permission/simulation"
)

func TestFindAccount(t *testing.T) {
	accs := []simtypes.Account{
		{Address: sample.RandAccAddress()},
		{Address: sample.RandAccAddress()},
	}

	got, ok := simulation.FindAccount(accs, accs[0].Address.String())
	require.True(t, ok)
	require.Equal(t, accs[0], got)

	notInList := sample.RandAccAddress()
	_, ok = simulation.FindAccount(accs, notInList.String())
	require.False(t, ok)

	require.Panics(t, func() {
		simulation.FindAccount(accs, "not-a-valid-address")
	})
}
