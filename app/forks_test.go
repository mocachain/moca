package app

import (
	"strconv"
	"testing"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/stretchr/testify/require"

	servercfg "github.com/mocachain/moca/v2/server/config"
)

const forkTestHeight = 100

// withHardfork points the app at a node-local hardfork entry for the given height.
func withHardfork(mocaApp *Moca, height int64, name string) {
	if mocaApp.appConfig == nil {
		mocaApp.appConfig = &servercfg.AppConfig{}
	}
	mocaApp.appConfig.Hardforks = map[string]servercfg.HardforkEntry{
		strconv.FormatInt(height, 10): {Name: name},
	}
}

// A configured entry at the current height schedules the plan.
func TestScheduleForkUpgrade_ConfiguredHardforkSchedules(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, forkTestHeight, "test-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)

	mocaApp.ScheduleForkUpgrade(ctx)

	plan, err := mocaApp.UpgradeKeeper.GetUpgradePlan(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-hardfork", plan.Name)
	require.Equal(t, int64(forkTestHeight), plan.Height)
}

// A hardfork is the path for an emergency that cannot wait on a proposal, so it
// supersedes an upgrade that is only pending rather than being skipped. This runs
// in BeginBlock, so the disagreement must not stop the chain either.
func TestScheduleForkUpgrade_SupersedesPendingPlan(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, forkTestHeight, "config-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)

	// Pending: scheduled for a height the chain has not reached.
	require.NoError(t, mocaApp.UpgradeKeeper.ScheduleUpgrade(ctx,
		upgradetypes.Plan{Name: "gov-upgrade", Height: forkTestHeight + 50}))

	require.NotPanics(t, func() { mocaApp.ScheduleForkUpgrade(ctx) })

	plan, err := mocaApp.UpgradeKeeper.GetUpgradePlan(ctx)
	require.NoError(t, err)
	require.Equal(t, "config-hardfork", plan.Name, "the hardfork must take precedence")
	require.Equal(t, int64(forkTestHeight), plan.Height)
}

// A plan at or before the current height cannot still be waiting to be applied,
// so it is replaced rather than treated as a conflict.
func TestScheduleForkUpgrade_ReplacesStalePlan(t *testing.T) {
	mocaApp := EthSetup(false, nil)

	earlier := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)
	require.NoError(t, mocaApp.UpgradeKeeper.ScheduleUpgrade(earlier,
		upgradetypes.Plan{Name: "old-upgrade", Height: forkTestHeight}))

	later := int64(forkTestHeight + 50)
	withHardfork(mocaApp, later, "config-hardfork")
	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(later)

	require.NotPanics(t, func() { mocaApp.ScheduleForkUpgrade(ctx) })

	plan, err := mocaApp.UpgradeKeeper.GetUpgradePlan(ctx)
	require.NoError(t, err)
	require.Equal(t, "config-hardfork", plan.Name)
	require.Equal(t, later, plan.Height)
}

// Re-running the same height must be a no-op rather than a second write.
func TestScheduleForkUpgrade_AlreadyScheduledIsIdempotent(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, forkTestHeight, "test-hardfork")

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

// A configured height is handled by configuration whatever the outcome, so the
// caller must not go on to consult the code-driven fork list. That list is empty
// today, and an entry added to it at a configured height would otherwise be
// scheduled on top of the hardfork.
func TestScheduleConfiguredHardfork_ConfiguredHeightIsClaimed(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	withHardfork(mocaApp, forkTestHeight, "config-hardfork")

	ctx := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight)
	require.True(t, mocaApp.scheduleConfiguredHardfork(ctx))

	// And a height with no entry hands back to the code-driven list.
	other := mocaApp.NewContext(false).
		WithChainID("moca_5151-1").
		WithBlockHeight(forkTestHeight + 1)
	require.False(t, mocaApp.scheduleConfiguredHardfork(other))
}
