package upgrades

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
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
//  1. Hard-purge the entire authz store, scrubbing BOTH the merkle tree and the
//     IAVL fastnode index. A plain Delete only clears the fastnode entry when
//     the key is in the tree (iavl's Remove returns removed=true); a phantom is
//     in fastnode only, so Delete short-circuits and leaves it. Doing
//     Set(key)+Delete(key) instead first puts the key in the tree, so the Delete
//     then removes it from both the tree and the fastnode. Sweeping every key
//     the store iterator yields (which includes the drifted-node phantoms) thus
//     ends with an empty authz tree AND an empty authz fastnode on every node.
//     The final tree is empty regardless of how many keys each node swept, so
//     the apphash is identical; the fastnode is empty everywhere, so the drift
//     is gone — no app.toml toggle, state-sync, or moca-iavl change needed.
//     (cmd/iavl_audit confirmed authz is the only drifted store.)
//
//  2. Re-grant the essential module-account grants, keyed off the canonical
//     staking and sp stores (identical on every node), so the set of writes is
//     deterministic regardless of drift:
//       - each validator's SelfDelAddress -> gov : MsgDelegate (Generic)
//       - each storage provider's FundingAddress -> gov : MsgDeposit (Generic)
//
// Non-essential grants (user-to-user MsgVote, capped SP DepositAuthorizations,
// etc.) are intentionally dropped; their owners re-create them. Generic is used
// for the restored grants to match the standard moca setup flow
// (deployment/dockerup/create_validator_proposal.sh grants gov a Generic
// MsgDelegate authorization).
//
// Determinism notes: the handler runs at block level on the infinite gas meter
// (op counts never metered into consensus), and the store writes emit no
// module events. MUST run on the v1.3.0 binary (carries cosmos/iavl#1009).
func V1_3_0ResetAuthzGrants(
	authzStoreKey storetypes.StoreKey,
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

		// ── Phase 1: hard-purge the authz store (tree + fastnode) ───────────
		store := sdkCtx.KVStore(authzStoreKey)
		var keys [][]byte
		it := store.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			k := make([]byte, len(it.Key()))
			copy(k, it.Key())
			keys = append(keys, k)
		}
		if err := it.Close(); err != nil {
			return nil, fmt.Errorf("authz reset: close iterator: %w", err)
		}
		for _, k := range keys {
			// Set first so the key is guaranteed present in the merkle tree, then
			// Delete so it is removed from BOTH the tree and the fastnode index
			// (a plain Delete leaves a not-in-tree phantom in the fastnode).
			store.Set(k, []byte{0})
			store.Delete(k)
		}
		logger.Info("authz reset: hard-purged store (tree+fastnode)", "keys", len(keys))

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
