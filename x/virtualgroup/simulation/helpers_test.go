package simulation_test

import (
	"math/rand"
	"strings"
	"testing"

	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/virtualgroup/simulation"
)

func TestFindAccount(t *testing.T) {
	r := rand.New(rand.NewSource(1)) //nolint:gosec
	accs := simtypes.RandomAccounts(r, 3)

	found, ok := simulation.FindAccount(accs, accs[1].Address.String())
	require.True(t, ok)
	require.Equal(t, accs[1], found)

	missing := "0x" + strings.Repeat("00", 20)
	_, ok = simulation.FindAccount(accs, missing)
	require.False(t, ok)

	require.Panics(t, func() {
		simulation.FindAccount(accs, "not-a-hex-address")
	})
}
