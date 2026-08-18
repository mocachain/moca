package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUpgradeHandlersRegistered pins the handler names the network upgrades
// through. A plan whose name has no handler halts every node at the upgrade
// height, and nothing else in the build fails when one is missing, so the set is
// asserted here rather than discovered on the chain.
func TestUpgradeHandlersRegistered(t *testing.T) {
	mocaApp := EthSetup(false, nil)

	for _, name := range []string{"v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0"} {
		require.True(t, mocaApp.UpgradeKeeper.HasHandler(name),
			"no upgrade handler registered for %s: a plan with this name would halt the chain", name)
	}
}
