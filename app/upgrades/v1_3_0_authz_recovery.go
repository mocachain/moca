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
)

// V1_3_0UpgradeName is the on-chain name of the v1.3.0 software upgrade.
const V1_3_0UpgradeName = "v1.3.0"

// V1_3_0RestoreValidatorDelegateGrant re-grants the one authz grant that moca's
// state machine requires for an existing validator to keep functioning but that
// the moca-iavl commit-time bug dropped from the merkle tree:
//
//	each validator's SelfDelAddress -> gov : MsgDelegate (GenericAuthorization)
//
// moca's custom MsgCreateValidator handler enforces this grant via
// CheckStakeAuthorization, and the standard create-validator flow grants it as a
// GenericAuthorization (deployment/dockerup/create_validator_proposal.sh), so
// restoring it as Generic matches the original — no behavior change.
//
// Scope (deliberately minimal — this is NOT a full recovery of every dropped
// authz entry): the handler reconstructs only what is derivable from the
// canonical staking store, so the SaveGrant writes are identical on every node
// regardless of per-node authz fastnode drift. SaveGrant's Set re-adds the
// grant to the merkle tree and overwrites any stale fastnode entry for that key.
// Grants whose granter is not a current validator self-delegation address — SP
// deposit grants, user-to-user grants, grants from accounts that have since
// unbonded — cannot be reconstructed from on-chain state and are NOT restored;
// their owners re-create them with the correct authorization (e.g. SP operators
// re-grant a scoped DepositAuthorization, not an unbounded generic one).
//
// The residual fastnode drift on those non-restored keys is node-local and is
// fixed by an IAVL fastnode rebuild (state-sync, or bumping fastStorageVersionValue),
// which rebuilds the fastnode from the canonical tree and covers both drift
// directions without consensus risk.
//
// Determinism notes: the restored grant is Generic with no expiration, so
// SaveGrant touches no grant-queue state and does not depend on reading the
// drifted store. The handler runs at block level on the infinite gas meter.
// MUST run on the v1.3.0 binary (carries cosmos/iavl#1009).
func V1_3_0RestoreValidatorDelegateGrant(
	authzKeeper authzkeeper.Keeper,
	stakingKeeper *stakingkeeper.Keeper,
	mm *module.Manager,
	configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger().With("upgrade", V1_3_0UpgradeName)
		govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)
		delegateMsg := sdk.MsgTypeURL(&stakingtypes.MsgDelegate{})

		vals, err := stakingKeeper.GetAllValidators(ctx)
		if err != nil {
			return nil, fmt.Errorf("authz restore: list validators: %w", err)
		}
		granted := 0
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
			granted++
		}

		logger.Info("authz restore: re-granted validator delegate grants", "count", granted)
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}
