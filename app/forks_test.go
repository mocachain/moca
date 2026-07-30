package app

import (
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/stretchr/testify/require"

	servercfg "github.com/mocachain/moca/v2/server/config"
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

// A configured entry at the current height schedules the plan.
func TestScheduleForkUpgrade_ConfiguredHardforkSchedules(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, "test-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)

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

// A configured entry claims its height even when it is skipped, so the caller
// must not fall through to the code-driven fork list. That list is empty today,
// so this pins the contract before a case is added at a height an operator has
// also configured.
func TestScheduleConfiguredHardfork_SkippedEntryStillClaimsHeight(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, "config-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)

	// An unrelated plan makes the configured entry take the skip path.
	require.NoError(t, mocaApp.UpgradeKeeper.ScheduleUpgrade(ctx,
		upgradetypes.Plan{Name: "gov-upgrade", Height: forkTestHeight + 50}))

	require.True(t, mocaApp.scheduleConfiguredHardfork(ctx),
		"a skipped entry must still report the height as claimed")
}
