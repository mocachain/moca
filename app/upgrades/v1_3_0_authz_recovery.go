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

	spkeeper "github.com/evmos/evmos/v12/x/sp/keeper"
	sptypes "github.com/evmos/evmos/v12/x/sp/types"
)

// V1_3_0UpgradeName is the on-chain name of the v1.3.0 software upgrade.
const V1_3_0UpgradeName = "v1.3.0"

// V1_3_0RestoreGovGrants re-grants the authz grants that moca's custom handlers
// require but that the moca-iavl commit-time bug dropped from the merkle tree:
//
//   - each validator's SelfDelAddress -> gov : MsgDelegate (Generic), required
//     by CheckStakeAuthorization on MsgCreateValidator
//   - each storage provider's FundingAddress -> gov : MsgDeposit (Generic),
//     required by CheckDepositAuthorization on MsgDeposit
//
// It is keyed off the canonical staking and sp stores (identical on every node
// by consensus), so the set of SaveGrant writes is the SAME on every node
// regardless of any per-node authz fastnode drift. SaveGrant's Set re-adds the
// grant to the merkle tree (so a node that lost it from the tree gets it back)
// and overwrites any stale fastnode entry for that exact key — healing the
// drift for the keys that matter, deterministically.
//
// Why this does NOT purge the authz store: a 2-node test proved that a handler
// CANNOT safely clear the store. The only enumeration reachable from a handler
// is the store iterator, which reads the IAVL fastnode index — not the tree. A
// node whose fastnode is *missing* a tree-backed key (fastnode < tree) never
// yields that key, so a purge keyed off the iterator skips it; the key survives
// in that node's tree while other nodes delete it -> divergent apphash -> fork.
// The fastnode drift itself is a node-local concern and is fixed the right way
// by an IAVL fastnode rebuild (state-sync, or bumping fastStorageVersionValue),
// which rebuilds the fastnode from the canonical tree and so covers both drift
// directions without any consensus risk.
//
// Determinism notes: the restored grants are Generic with no expiration (the
// standard moca setup grant — see deployment/dockerup/create_validator_proposal.sh),
// so SaveGrant touches no grant-queue state and its result does not depend on
// reading the drifted store. The handler runs at block level on the infinite
// gas meter. MUST run on the v1.3.0 binary (carries cosmos/iavl#1009).
func V1_3_0RestoreGovGrants(
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

		delegateMsg := sdk.MsgTypeURL(&stakingtypes.MsgDelegate{})
		depositMsg := sdk.MsgTypeURL(&sptypes.MsgDeposit{})

		// validator self-delegation account -> gov : MsgDelegate
		vals, err := stakingKeeper.GetAllValidators(ctx)
		if err != nil {
			return nil, fmt.Errorf("authz restore: list validators: %w", err)
		}
		valGrants := 0
		for _, v := range vals {
			granter, err := sdk.AccAddressFromHexUnsafe(v.SelfDelAddress)
			if err != nil || granter.Empty() {
				logger.Error("authz restore: skip validator with bad self-del addr",
					"validator", v.OperatorAddress, "self_del", v.SelfDelAddress, "err", err)
				continue
			}
			if err := authzKeeper.SaveGrant(ctx, govAddr, granter, authz.NewGenericAuthorization(delegateMsg), nil); err != nil {
				return nil, fmt.Errorf("authz restore: grant MsgDelegate for %s: %w", v.SelfDelAddress, err)
			}
			valGrants++
		}

		// storage provider funding account -> gov : MsgDeposit
		spGrants := 0
		for _, sp := range spKeeper.GetAllStorageProviders(sdkCtx) {
			granter, err := sdk.AccAddressFromHexUnsafe(sp.FundingAddress)
			if err != nil || granter.Empty() {
				logger.Error("authz restore: skip sp with bad funding addr",
					"sp", sp.Id, "funding", sp.FundingAddress, "err", err)
				continue
			}
			if err := authzKeeper.SaveGrant(ctx, govAddr, granter, authz.NewGenericAuthorization(depositMsg), nil); err != nil {
				return nil, fmt.Errorf("authz restore: grant MsgDeposit for %s: %w", sp.FundingAddress, err)
			}
			spGrants++
		}

		logger.Info("authz restore: re-granted required gov grants",
			"validator_delegate", valGrants, "sp_deposit", spGrants)
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
