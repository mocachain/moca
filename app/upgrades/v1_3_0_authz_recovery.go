package upgrades

import (
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
)

// V1_3_0UpgradeName is the on-chain name of the v1.3.0 software upgrade.
const V1_3_0UpgradeName = "v1.3.0"

//go:embed recovery.json
var recoveryJSON []byte

// recoveryFile is the structure of recovery.json. It is keyed by chain-id so
// the same binary is a deterministic no-op on networks with no damage
// (devnet/testnet) and only re-inserts grants on the network that needs it.
type recoveryFile struct {
	Description   string                     `json:"description"`
	AuditedHeight int64                      `json:"audited_version"`
	Networks      map[string]networkRecovery `json:"networks"`
}

type networkRecovery struct {
	Note   string          `json:"note"`
	Grants []recoveryGrant `json:"grants"`
}

type recoveryGrant struct {
	Granter         string `json:"granter"`           // 0x-hex, 20 bytes
	Grantee         string `json:"grantee"`           // 0x-hex, 20 bytes
	MsgTypeURL      string `json:"msg_type_url"`      // e.g. /moca.sp.MsgDeposit
	IAVLLeafVersion int64  `json:"iavl_leaf_version"` // provenance only; unused at runtime
}

// V1_3_0AuthzRecovery re-inserts the authz grants that the moca-iavl
// commit-time bug dropped from the merkle tree at mainnet block 17,123,239.
//
// The affected grants are still present in live state (no-proof queries return
// them), but the merkle tree cannot reach them: proof queries panic, and any
// node that state-synced after the bug never received them. cmd/iavl_audit
// confirmed that authz is the ONLY damaged store on mainnet — evm, sp, staking,
// bank, acc and every other substore are fully reachable.
//
// Why unconditional SaveGrant (not "skip if present"): the keeper's
// GetAuthorization reads the fastnode cache, which still holds these grants, so
// it would wrongly report them present and skip the repair while the merkle
// tree stays broken. SaveGrant writes through to the IAVL tree, so it repairs
// the merkle on a block-synced node and inserts the leaf on a state-synced one.
// Because the committed authz root currently EXCLUDES these grants on every node
// (consensus), the same SaveGrant set produces the same new root everywhere —
// deterministic, no fork.
//
// MUST run on the v1.3.0 binary, which carries the cosmos/iavl#1009 GetNode
// reformatted-root fallback; without it, SaveGrant's tree traversal can hit the
// same missing node and panic.
func V1_3_0AuthzRecovery(
	authzKeeper authzkeeper.Keeper,
	mm *module.Manager,
	configurator module.Configurator,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		logger := sdkCtx.Logger().With("upgrade", V1_3_0UpgradeName)
		chainID := sdkCtx.ChainID()

		var rf recoveryFile
		if err := json.Unmarshal(recoveryJSON, &rf); err != nil {
			return nil, fmt.Errorf("authz recovery: parse recovery.json: %w", err)
		}

		spec, ok := rf.Networks[chainID]
		if !ok || len(spec.Grants) == 0 {
			logger.Info("authz recovery: nothing to do for this chain",
				"chain_id", chainID, "audited_version", rf.AuditedHeight)
			return mm.RunMigrations(ctx, configurator, fromVM)
		}

		logger.Info("authz recovery: begin",
			"chain_id", chainID, "grants", len(spec.Grants), "note", spec.Note)

		recovered := 0
		for _, g := range spec.Grants {
			granter, err := parseHexAddr(g.Granter)
			if err != nil {
				return nil, fmt.Errorf("authz recovery: granter %q: %w", g.Granter, err)
			}
			grantee, err := parseHexAddr(g.Grantee)
			if err != nil {
				return nil, fmt.Errorf("authz recovery: grantee %q: %w", g.Grantee, err)
			}

			authorization := authz.NewGenericAuthorization(g.MsgTypeURL)
			// expiration nil: all recovered mainnet grants were created without
			// an expiration (confirmed from their on-disk leaf bytes).
			if err := authzKeeper.SaveGrant(ctx, grantee, granter, authorization, nil); err != nil {
				return nil, fmt.Errorf("authz recovery: SaveGrant(%s -> %s, %s): %w",
					g.Granter, g.Grantee, g.MsgTypeURL, err)
			}
			logger.Info("authz recovery: re-inserted grant",
				"granter", g.Granter, "grantee", g.Grantee, "msg", g.MsgTypeURL)
			recovered++
		}

		// Sanity check: every recovered grant must now be retrievable. (This
		// reads the live store; the authoritative merkle/proof verification is
		// done off-chain post-upgrade with cmd/iavl_audit + prove=true queries.)
		var failed []string
		for _, g := range spec.Grants {
			granter, _ := parseHexAddr(g.Granter)
			grantee, _ := parseHexAddr(g.Grantee)
			if got, _ := authzKeeper.GetAuthorization(ctx, grantee, granter, g.MsgTypeURL); got == nil {
				failed = append(failed, fmt.Sprintf("%s->%s (%s)", g.Granter, g.Grantee, g.MsgTypeURL))
			}
		}
		if len(failed) > 0 {
			return nil, fmt.Errorf("authz recovery: post-check failed for %d grant(s): %s",
				len(failed), strings.Join(failed, ", "))
		}

		logger.Info("authz recovery: complete", "recovered", recovered)
		return mm.RunMigrations(ctx, configurator, fromVM)
	}
}

// parseHexAddr converts moca's 0x-hex address form to sdk.AccAddress (raw bytes).
func parseHexAddr(s string) (sdk.AccAddress, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		return nil, fmt.Errorf("missing 0x prefix: %q", s)
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil {
		return nil, fmt.Errorf("not hex: %q (%w)", s, err)
	}
	if len(b) != 20 {
		return nil, fmt.Errorf("expected 20-byte address, got %d bytes from %q", len(b), s)
	}
	return sdk.AccAddress(b), nil
}
