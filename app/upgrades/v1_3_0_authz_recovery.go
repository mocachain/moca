package upgrades

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	spkeeper "github.com/mocachain/moca/v2/x/sp/keeper"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// V1_3_0UpgradeName is the on-chain name of the v1.3.0 software upgrade.
const V1_3_0UpgradeName = "v1.3.0"

// V1_3_0ResetAuthzGrants clears the authz store and re-grants only the grants
// that moca's custom handlers require to function.
//
// Background: the moca-iavl commit-time bug left the authz store with
// fastnode-vs-merkle-tree drift on the original chain participants. Some grants
// live only in the fastnode index (prove=false returns them) but were dropped
// from the merkle tree (prove=true panics, apphash excludes them); state-synced
// nodes never received them. Notably, the grants every validator must hold —
// SelfDelAddress -> gov for MsgDelegate, enforced by CheckStakeAuthorization on
// MsgCreateValidator — were among those dropped, so on a clean node no new
// validator could be created.
//
// Two-phase, fully deterministic:
//
//  1. Purge every authz grant. The canonical merkle tree (identical on every
//     node by consensus) holds the healthy grants; deleting those empties the
//     tree. Phantom grants were never in the tree, so deleting them is a no-op.
//     The result is an empty authz tree on every node — no per-node-drift data
//     to reconstruct, no list to get wrong.
//
//  2. Re-grant the essential module-account grants, keyed off the canonical
//     staking and sp stores (identical on every node), so the set of writes is
//     deterministic regardless of drift:
//       - each validator's SelfDelAddress -> gov : MsgDelegate (Generic)
//       - each storage provider's FundingAddress -> gov : MsgDeposit (Generic)
//     SaveGrant uses Set, which writes the grant into the tree on every node
//     (restoring any that were dropped) and overwrites the phantom fastnode
//     entry — so these keys end up consistent (tree == fastnode) everywhere.
//
// Non-essential grants (user-to-user MsgVote, capped SP DepositAuthorizations,
// etc.) are intentionally dropped; their owners re-create them. Generic is used
// for the restored grants to match the standard moca setup flow
// (deployment/dockerup/create_validator_proposal.sh grants gov a Generic
// MsgDelegate authorization).
//
// Determinism notes: the handler runs at block level on the infinite gas meter
// (op counts never metered into consensus), and DeleteGrant/SaveGrant emit
// block-level events that are not part of LastResultsHash. MUST run on the
// v1.3.0 binary (carries cosmos/iavl#1009).
func V1_3_0ResetAuthzGrants(
	authzKeeper authzkeeper.Keeper,
	stakingKeeper *stakingkeeper.Keeper,
	spKeeper spkeeper.Keeper,
	mm *module.Manager,
	configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger().With("upgrade", V1_3_0UpgradeName)
		govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)

		// ── Phase 1: purge every grant ──────────────────────────────────────
		type gk struct {
			granter, grantee sdk.AccAddress
			msg              string
		}
		var existing []gk
		authzKeeper.IterateGrants(ctx, func(granter, grantee sdk.AccAddress, g authz.Grant) bool {
			m := ""
			if a, err := g.GetAuthorization(); err == nil && a != nil {
				m = a.MsgTypeURL()
			}
			existing = append(existing, gk{granter: granter, grantee: grantee, msg: m})
			return false
		})
		purged := 0
		for _, k := range existing {
			if k.msg == "" {
				continue
			}
			if err := authzKeeper.DeleteGrant(ctx, k.grantee, k.granter, k.msg); err != nil {
				// Iterator surfaced a grant DeleteGrant can't remove (e.g. a
				// drifted-node phantom): a no-op for the merkle tree. Continue so
				// the handler stays deterministic across nodes with different drift.
				logger.Error("authz reset: delete failed (continuing)",
					"granter", k.granter.String(), "grantee", k.grantee.String(), "msg", k.msg, "err", err)
				continue
			}
			purged++
		}
		logger.Info("authz reset: purged grants", "found", len(existing), "deleted", purged)

		// ── Phase 2: restore the essential module-account grants ────────────
		delegateMsg := sdk.MsgTypeURL(&stakingtypes.MsgDelegate{})
		depositMsg := sdk.MsgTypeURL(&sptypes.MsgDeposit{})

		// 2a. validator self-delegation account -> gov : MsgDelegate
		vals, err := stakingKeeper.GetAllValidators(ctx)
		if err != nil {
			return nil, fmt.Errorf("authz reset: list validators: %w", err)
		}
		valGrants := 0
		for _, v := range vals {
			granter, err := sdk.AccAddressFromHexUnsafe(v.SelfDelAddress)
			if err != nil || granter.Empty() {
				// Skip rather than halt the upgrade on an unexpected/empty
				// self-del address; deterministic since every node sees the same
				// validator store.
				logger.Error("authz reset: skip validator with bad self-del addr",
					"validator", v.OperatorAddress, "self_del", v.SelfDelAddress, "err", err)
				continue
			}
			if err := authzKeeper.SaveGrant(ctx, govAddr, granter, authz.NewGenericAuthorization(delegateMsg), nil); err != nil {
				return nil, fmt.Errorf("authz reset: grant MsgDelegate for %s: %w", v.SelfDelAddress, err)
			}
			valGrants++
		}

		// 2b. storage provider funding account -> gov : MsgDeposit
		spGrants := 0
		for _, sp := range spKeeper.GetAllStorageProviders(sdkCtx) {
			granter, err := sdk.AccAddressFromHexUnsafe(sp.FundingAddress)
			if err != nil || granter.Empty() {
				logger.Error("authz reset: skip sp with bad funding addr",
					"sp", sp.Id, "funding", sp.FundingAddress, "err", err)
				continue
			}
			if err := authzKeeper.SaveGrant(ctx, govAddr, granter, authz.NewGenericAuthorization(depositMsg), nil); err != nil {
				return nil, fmt.Errorf("authz reset: grant MsgDeposit for %s: %w", sp.FundingAddress, err)
			}
			spGrants++
		}

		logger.Info("authz reset: restored essential grants",
			"validator_delegate", valGrants, "sp_deposit", spGrants)
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
