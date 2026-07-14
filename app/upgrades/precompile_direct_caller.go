package upgrades

import (
	"context"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// PrecompileDirectCallerUpgradeName is the upgrade that activates the precompile
// direct-caller semantics: the EOA-only guard is removed so contracts may call
// precompiles, and the business identity becomes the direct caller
// (contract.Caller()) instead of tx.origin. See x/evm/precompiles/README.md and
// the caller-semantics change (removes "only allow EOA can call this method").
const PrecompileDirectCallerUpgradeName = "v2.1.0-precompile-direct-caller"

// PrecompileDirectCaller returns the upgrade handler that activates the
// direct-caller precompile semantics at the upgrade height.
//
// WHY A CHAIN UPGRADE: removing the EOA-only guard changes the consensus
// execution result of already-deployed contract transactions (a contract calling
// a precompile is rejected before the upgrade and succeeds after). It must switch
// on at a single, coordinated height across all validators — not at binary deploy
// time — or nodes fork.
//
// GATING (pending design decision — intentionally NOT implemented here):
// the precompile-side guard must be *conditional* on this activation so that
// pre-upgrade blocks keep enforcing EOA-only and only post-upgrade blocks allow
// contract callers. The activation hook belongs here once the mechanism is
// chosen. Two candidate mechanisms:
//
//   - On-chain param flag (recommended): flip a deterministic param (e.g. an
//     "allow contract call" flag on x/vm or a moca-owned param store) in this
//     handler; the precompile base reads it and enforces EOA-only only while the
//     flag is unset. Deterministic and independent of node-local config.
//   - Height gate: no state change here; each precompile compares
//     ctx.BlockHeight() against this upgrade's plan height.
//
// The companion PR that removes the EOA-only guard currently does so
// UNCONDITIONALLY; it must be reworked to gate on the chosen mechanism before
// this upgrade can safely ship. Until then this handler performs no
// direct-caller state change and only runs the standard module migrations.
//
// DELIVERY CHECKLIST (owner: chain ops):
//   - choose the upgrade height and cut the versioned binary
//   - governance MsgSoftwareUpgrade at that height
//   - testnet or local fork/replay proving pre- vs post-upgrade behavior for:
//     EOA direct call, contract forwarding, internal keeper EVM calls, revert,
//     and balance sync
//   - publish the SDK / CLI / contract-integrator migration notes
//     (see app/upgrades/PRECOMPILE_DIRECT_CALLER.md)
func PrecompileDirectCaller(mm *module.Manager, configurator module.Configurator) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// TODO(precompile-direct-caller): once the gating mechanism is chosen,
		// activate direct-caller here (e.g. set the "allow contract call" param).
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
