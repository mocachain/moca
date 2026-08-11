package app

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	mocatypes "github.com/mocachain/moca/v2/types"
)

// Every precompile named in the blocklist has to be blocked by the bank keeper.
// The list is written as lowercase hex literals, but the keeper looks entries up
// by AccAddress.String(), which is EIP-55 checksummed. An address whose checksum
// uppercases a character therefore never matches and is silently not blocked.
func TestBlockedAccountAddrs_PrecompilesAreBlockedByBankKeeper(t *testing.T) {
	mocaApp := EthSetup(false, nil)

	precompiles := []struct{ name, hex string }{
		{"bank", mocatypes.BankAddress},
		{"auth", mocatypes.AuthAddress},
		{"gov", mocatypes.GovAddress},
		{"staking", mocatypes.StakingAddress},
		{"distribution", mocatypes.DistributionAddress},
		{"slashing", mocatypes.SlashingAddress},
		{"evidence", mocatypes.EvidenceAddress},
		{"authz", mocatypes.AuthzAddress},
		{"feemarket", mocatypes.FeemarketAddress},
		{"payment", mocatypes.PaymentAddress},
		{"permission", mocatypes.PermissionAddress},
		{"virtualgroup", mocatypes.VirtualGroupAddress},
		{"storage", mocatypes.StorageAddress},
		{"sp", mocatypes.SpAddress},
	}

	for _, p := range precompiles {
		addr := sdk.AccAddress(common.HexToAddress(p.hex).Bytes())
		require.True(t, mocaApp.BankKeeper.BlockedAddr(addr),
			"the %s precompile (%s) must be blocked from receiving funds", p.name, p.hex)
	}
}
