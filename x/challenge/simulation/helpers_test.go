package simulation_test

import (
	"math/rand"
	"testing"

	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/challenge/simulation"
)

func TestFindAccount(t *testing.T) {
	r := rand.New(rand.NewSource(1)) //nolint:gosec
	accs := simtypes.RandomAccounts(r, 3)

	found, ok := simulation.FindAccount(accs, accs[1].Address.String())
	require.True(t, ok)
	require.Equal(t, accs[1], found)

	_, ok = simulation.FindAccount(accs, sample.RandAccAddressHex())
	require.False(t, ok)

	require.Panics(t, func() {
		simulation.FindAccount(accs, "not-a-valid-hex-address")
	})
}
