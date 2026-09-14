package simulation_test

import (
	"math/rand"
	"testing"

	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/storage/simulation"
)

func TestFindAccount(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	allAccs := simtypes.RandomAccounts(r, 4)
	accs := allAccs[:3]
	other := allAccs[3]

	t.Run("found", func(t *testing.T) {
		got, found := simulation.FindAccount(accs, accs[1].Address.String())

		require.True(t, found)
		require.Equal(t, accs[1], got)
	})

	t.Run("not found", func(t *testing.T) {
		_, found := simulation.FindAccount(accs, other.Address.String())

		require.False(t, found)
	})

	t.Run("panics on unparseable address", func(t *testing.T) {
		require.Panics(t, func() {
			simulation.FindAccount(accs, "not-a-valid-address")
		})
	})
}
