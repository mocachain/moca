package app

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/stretchr/testify/require"

	servercfg "github.com/mocachain/moca/v2/server/config"
	"github.com/mocachain/moca/v2/utils"
)

const forkTestHeight = 100

// withHardfork points the app at a node-local hardfork entry for forkTestHeight.
func withHardfork(mocaApp *Moca, name string) {
	if mocaApp.appConfig == nil {
		mocaApp.appConfig = &servercfg.AppConfig{}
	}
	mocaApp.appConfig.Hardforks = map[string]servercfg.HardforkEntry{
		"100": {Name: name},
	}
}

// Scheduling an upgrade writes to consensus state, so a per-node config must not
// be able to trigger it on mainnet: two validators whose app.toml disagreed would
// produce different app hashes for the same block.
func TestScheduleForkUpgrade_ConfiguredHardforkIgnoredOnMainnet(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, "test-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID(utils.MainnetChainID + "-1").
		WithBlockHeight(forkTestHeight)
	require.True(t, utils.IsMainnet(ctx.ChainID()), "test must run against a mainnet chain-id")

	mocaApp.ScheduleForkUpgrade(ctx)

	_, err := mocaApp.UpgradeKeeper.GetUpgradePlan(ctx)
	require.ErrorIs(t, err, upgradetypes.ErrNoUpgradePlanFound,
		"a node-local config entry must not schedule an upgrade on mainnet")
}

// The mechanism is still available where it is meant to be used.
func TestScheduleForkUpgrade_ConfiguredHardforkSchedulesOffMainnet(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, "test-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)
	require.False(t, utils.IsMainnet(ctx.ChainID()))

	mocaApp.ScheduleForkUpgrade(ctx)

	plan, err := mocaApp.UpgradeKeeper.GetUpgradePlan(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-hardfork", plan.Name)
	require.Equal(t, int64(forkTestHeight), plan.Height)
}

// A governance plan and a configured hardfork can coexist. This runs in
// BeginBlock, so the disagreement must not stop the chain, and the plan already
// agreed on-chain is the one to keep.
func TestScheduleForkUpgrade_ExistingPlanIsKept(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, "config-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)

	govPlan := upgradetypes.Plan{Name: "gov-upgrade", Height: forkTestHeight + 50}
	require.NoError(t, mocaApp.UpgradeKeeper.ScheduleUpgrade(ctx, govPlan))

	require.NotPanics(t, func() { mocaApp.ScheduleForkUpgrade(ctx) })

	plan, err := mocaApp.UpgradeKeeper.GetUpgradePlan(ctx)
	require.NoError(t, err)
	require.Equal(t, "gov-upgrade", plan.Name, "the on-chain plan must survive")
	require.Equal(t, int64(forkTestHeight+50), plan.Height)
}

// Re-running the same height must be a no-op rather than a second write.
func TestScheduleForkUpgrade_AlreadyScheduledIsIdempotent(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, "test-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)

	mocaApp.ScheduleForkUpgrade(ctx)
	require.NotPanics(t, func() { mocaApp.ScheduleForkUpgrade(ctx) })

	plan, err := mocaApp.UpgradeKeeper.GetUpgradePlan(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-hardfork", plan.Name)
	require.Equal(t, int64(forkTestHeight), plan.Height)
}
