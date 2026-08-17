// Copyright 2022 Evmos Foundation
// This file is part of the Evmos Network packages.
//
// Evmos is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Evmos packages are distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Evmos packages. If not, see https://github.com/evmos/evmos/blob/main/LICENSE

package app

import (
	"errors"
	"fmt"
	"strconv"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/evmos/evmos/v12/utils"
)

// ScheduleForkUpgrade executes any necessary fork logic for based upon the current
// block height and chain ID (mainnet or testnet). It sets an upgrade plan once
// the chain reaches the pre-defined upgrade height.
//
// CONTRACT: for this logic to work properly it is required to:
//
//  1. Release a non-breaking patch version so that the chain can set the scheduled upgrade plan at upgrade-height.
//  2. Release the software defined in the upgrade-info
//
//nolint:all
func (app *Evmos) ScheduleForkUpgrade(ctx sdk.Context) {
	// 1) Config-driven hardfork scheduling (recommended for localnet/testnet and emergencies).
	// This allows operators to schedule an x/upgrade plan without governance by coordinating
	// the upgrade height and binaries (e.g. via cosmovisor).
	if app.appConfig != nil && len(app.appConfig.Hardforks) > 0 {
		if app.scheduleConfiguredHardfork(ctx) {
			return
		}
	}

	// 2) Code-driven fork scheduling (historical behavior).
	// NOTE: there are no testnet forks for the existing versions
	if !utils.IsMainnet(ctx.ChainID()) {
		return
	}

	upgradePlan := upgradetypes.Plan{
		Height: ctx.BlockHeight(),
	}

	// handle mainnet forks with their corresponding upgrade name and info
	switch ctx.BlockHeight() {
	default:
		// No-op
		return
	}

	// schedule the upgrade plan to the current block hight, effectively performing
	// a hard fork that uses the upgrade handler to manage the migration.
	if err := app.UpgradeKeeper.ScheduleUpgrade(ctx, upgradePlan); err != nil {
		panic(
			fmt.Errorf(
				"failed to schedule upgrade %s during BeginBlock at height %d: %w",
				upgradePlan.Name, ctx.BlockHeight(), err,
			),
		)
	}
}

// scheduleConfiguredHardfork checks if theres a hardfork configured for the
// current block height and schedules it. Returns true if this height is claimed
// by configuration — scheduled, already present, or deliberately left alone —
// which tells the caller not to also consult the code-driven fork list. Only a
// height with no entry returns false.
func (app *Evmos) scheduleConfiguredHardfork(ctx sdk.Context) bool {
	heightKey := strconv.FormatInt(ctx.BlockHeight(), 10)
	entry, ok := app.appConfig.Hardforks[heightKey]
	if !ok || entry.Name == "" {
		// 1.if no hardfork is configured, return false
		return false
	}

	// 2. Check for an existing upgrade plan. A configured hardfork is the path for
	// an emergency that cannot wait on a proposal, so it takes precedence over an
	// upgrade that is only pending: ScheduleUpgrade below replaces the stored plan
	// and clears the old plan's IBC state. This used to panic on any mismatch,
	// which stopped the node in BeginBlock with nothing to recover to.
	existing, err := app.UpgradeKeeper.GetUpgradePlan(ctx)
	switch {
	case err == nil && existing.Name == entry.Name && existing.Height == ctx.BlockHeight():
		return true // This has already been scheduled..., exit early.
	case err == nil && existing.Height >= ctx.BlockHeight():
		// Still to be applied — this runs ahead of the upgrade module, so a plan
		// at this very height has not been acted on yet. Cancel it outright rather
		// than letting ScheduleUpgrade overwrite it: a silently replaced plan
		// leaves everything tracking upgrade state waiting for an upgrade that
		// will never arrive, and if scheduling then failed the plan would still be
		// sitting there to fire at its own height.
		if err := app.cancelUpgradePlan(ctx); err != nil {
			ctx.Logger().Error("failed to cancel the pending upgrade plan",
				"pending", existing.Name, "pendingHeight", existing.Height, "error", err)
			return true
		}
		ctx.Logger().Warn("canceled a pending upgrade plan for the configured hardfork",
			"canceled", existing.Name, "canceledHeight", existing.Height,
			"configured", entry.Name, "height", ctx.BlockHeight())
	case err == nil:
		// Below this height the upgrade module has already had its chance at it.
		ctx.Logger().Warn("replacing a stale upgrade plan with the configured hardfork",
			"stale", existing.Name, "staleHeight", existing.Height,
			"configured", entry.Name, "height", ctx.BlockHeight())
	case !errors.Is(err, upgradetypes.ErrNoUpgradePlanFound):
		// Nothing about the stored plan can be trusted, so do not overwrite it.
		ctx.Logger().Error("skipping configured hardfork: cannot read the existing upgrade plan",
			"configured", entry.Name, "height", ctx.BlockHeight(), "error", err)
		return true
	}

	// 3. Schedule the upgrade
	upgradePlan := upgradetypes.Plan{
		Name:   entry.Name,
		Height: ctx.BlockHeight(),
		Info:   entry.Info, // optional, empty string if not set
	}
	if err := app.UpgradeKeeper.ScheduleUpgrade(ctx, upgradePlan); err != nil {
		ctx.Logger().Error("failed to schedule configured hardfork",
			"configured", entry.Name, "height", ctx.BlockHeight(), "error", err)
		return true
	}

	// This write is part of the app hash, so record it in the node's own log.
	ctx.Logger().Info("scheduled hardfork from node configuration",
		"name", upgradePlan.Name, "height", upgradePlan.Height)

	return true
}

// cancelUpgradePlan cancels the stored upgrade plan by executing the upgrade
// module's own MsgCancelUpgrade through the message router, the same way
// governance executes the messages in a passed proposal. Routing it rather than
// reaching into the keeper keeps the module's authority check and leaves one
// path for canceling a plan, whether the request came from a proposal or from
// this node's own configuration.
func (app *Evmos) cancelUpgradePlan(ctx sdk.Context) error {
	msg := &upgradetypes.MsgCancelUpgrade{
		Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	}
	handler := app.MsgServiceRouter().Handler(msg)
	if handler == nil {
		return fmt.Errorf("no handler registered for %T", msg)
	}
	if _, err := handler(ctx, msg); err != nil {
		return err
	}
	// baseapp emits this around every message it runs; the handler on its own does
	// not, and neither does the upgrade module. Without it the cancellation leaves
	// no trace for anything watching the chain rather than the node's log.
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		sdk.EventTypeMessage,
		sdk.NewAttribute(sdk.AttributeKeyAction, sdk.MsgTypeURL(msg)),
	))
	return nil
}
