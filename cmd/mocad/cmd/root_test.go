package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	servercfg "github.com/mocachain/moca/v2/server/config"
)

// TestInitAppConfigBakesEVMChainID asserts that initAppConfig threads the EVM
// chain ID into the rendered app.toml config, so `mocad init --chain-id
// moca_<evmid>-<epoch>` writes the correct evm.evm-chain-id with no operator step.
func TestInitAppConfigBakesEVMChainID(t *testing.T) {
	cases := []struct {
		name  string
		evmID uint64
	}{
		{"mainnet", 2288},
		{"testnet", 222888},
		{"devnet", 5151},
		{"unset keeps cosmos/evm default", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, cfg := initAppConfig(tc.evmID)
			ac, ok := cfg.(servercfg.AppConfig)
			require.True(t, ok, "unexpected app config type %T", cfg)
			require.Equal(t, tc.evmID, ac.EVM.EVMChainID)
		})
	}
}

// TestEVMChainIDFromChainID covers the init-time derivation of the EVM chain ID
// from --chain-id: any well-formed <name>_<evmid>-<epoch> yields its evmid (the
// prefix need not be "moca", matching types.ParseChainID), while empty, malformed,
// or out-of-range inputs fall back to 0 (cosmos/evm's default) rather than error
// or guess.
func TestEVMChainIDFromChainID(t *testing.T) {
	cases := []struct {
		name    string
		chainID string
		want    uint64
	}{
		{"mainnet chain-id", "moca_2288-1", 2288},
		{"testnet chain-id", "moca_222888-1", 222888},
		{"devnet chain-id", "moca_5151-1", 5151},
		// ParseChainID accepts any lowercase prefix, so a non-moca prefix with the
		// valid <name>_<evmid>-<epoch> shape still derives its evmid.
		{"non-moca prefix, valid shape", "other_2288-1", 2288},
		{"empty (no --chain-id)", "", 0},
		{"malformed (no evmid segment)", "cosmoshub-4", 0},
		{"malformed (no separators)", "not-a-chain-id", 0},
		{"non-numeric evm id", "moca_x-1", 0},
		// 2^64 + 5151: Uint64() would truncate to 5151; guard must return 0 (unset).
		{"out-of-range evm id fails closed", "moca_18446744073709556767-1", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, evmChainIDFromChainID(tc.chainID))
		})
	}
}
