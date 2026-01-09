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
// current block height and schedules it. Returns true if a hardfork was found
// and handled (either scheduled or already present).
func (app *Evmos) scheduleConfiguredHardfork(ctx sdk.Context) bool {
	heightKey := strconv.FormatInt(ctx.BlockHeight(), 10)
	entry, ok := app.appConfig.Hardforks[heightKey]
	if !ok || entry.Name == "" {
		// 1.if no hardfork is configured, return false
		return false
	}

	// 2. Check for existing upgrade plan
	existing, err := app.UpgradeKeeper.GetUpgradePlan(ctx)
	switch {
	case err == nil && existing.Name == entry.Name && existing.Height == ctx.BlockHeight():
		return true // This has already been scheduled..., exit early.
	case err == nil:
		panic(fmt.Errorf(
			"hardfork config wants to schedule upgrade %q at height %d but existing upgrade plan is %q at height %d",
			entry.Name, ctx.BlockHeight(), existing.Name, existing.Height,
		)) // this should never happen, panic.
	case !errors.Is(err, upgradetypes.ErrNoUpgradePlanFound):
		panic(fmt.Errorf("failed to read existing upgrade plan: %w", err))
	}

	// 3. Schedule the upgrade
	upgradePlan := upgradetypes.Plan{
		Name:   entry.Name,
		Height: ctx.BlockHeight(),
		Info:   entry.Info, // optional, empty string if not set
	}
	if err := app.UpgradeKeeper.ScheduleUpgrade(ctx, upgradePlan); err != nil {
		panic(fmt.Errorf("failed to schedule upgrade %s at height %d: %w",
			upgradePlan.Name, ctx.BlockHeight(), err))
	}

	return true
}
