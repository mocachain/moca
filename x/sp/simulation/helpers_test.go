package simulation_test

import (
	"testing"

	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	spsimulation "github.com/mocachain/moca/v2/x/sp/simulation"
)

// TestFindAccount covers the happy path: a well-formed hex address that
// matches one of the supplied simulation accounts.
func TestFindAccount(t *testing.T) {
	target := simtypes.Account{Address: sample.RandAccAddress()}
	accs := []simtypes.Account{{Address: sample.RandAccAddress()}, target}

	got, found := spsimulation.FindAccount(accs, target.Address.String())

	require.True(t, found)
	require.Equal(t, target.Address, got.Address)
}

// TestFindAccount_PanicsOnMalformedAddress covers the panic(err) branch: an
// address that isn't valid hex must not be swallowed as "not found".
func TestFindAccount_PanicsOnMalformedAddress(t *testing.T) {
	require.Panics(t, func() {
		spsimulation.FindAccount(nil, "not-a-valid-hex-address")
	})
}
