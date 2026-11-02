package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUpgradeHandlersRegistered fails the build if any planned upgrade name
// lacks a handler -- a missing one would halt the chain at that height.
func TestUpgradeHandlersRegistered(t *testing.T) {
	mocaApp := EthSetup(false, nil)

	for _, name := range []string{"v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0", upgradeNameV2} {
		require.True(t, mocaApp.UpgradeKeeper.HasHandler(name),
			"no upgrade handler registered for %s: a plan with this name would halt the chain", name)
	}
}
