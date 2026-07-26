package app

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDenomFromBalanceKey(t *testing.T) {
	// Balance key format: prefix (02) + address length (14) + address (20 bytes) + denom
	// "amoca" in hex is 616d6f6361
	key, _ := hex.DecodeString("0214cf150037e47b0c53e826a2d0050de1da2c8f5caa616d6f6361")
	require.Equal(t, "amoca", parseDenomFromBalanceKey(key))
}

func TestParseAddressFromBalanceKey(t *testing.T) {
	key, _ := hex.DecodeString("0214040ffd5925d40e11c67b7238a7fc9957850b8b9a424e42")
	require.Equal(t, "0x040fFD5925D40E11c67b7238A7fc9957850B8b9a", parseAddressFromBalanceKey(key))
}

func TestParseDenomFromSupplyKey(t *testing.T) {
	// Supply key format: prefix (00) + denom
	// "amoca" in hex is 616d6f6361
	key, _ := hex.DecodeString("00616d6f6361")
	require.Equal(t, "amoca", parseDenomFromSupplyKey(key))
}

// TestSaveUnbalancedBlockHeight_UsesMountedStoreKey is the regression test for MOCA-670.
// saveUnbalancedBlockHeight resolved the reconciliation commit store with a freshly built
// storetypes.NewKVStoreKey(reconStoreKey). rootmulti keys its store map by StoreKey identity
// (rs.stores[key]), so a fresh key is absent -> GetCommitStore returns nil -> the (*iavl.Store)
// assertion panics ("interface conversion: types.CommitStore is nil, not *iavl.Store").
// getUnbalancedBlockHeight already uses the mounted app.GetKey and works; this asserts save
// resolves + round-trips the same way. It panics (fails) on the unfixed code.
func TestSaveUnbalancedBlockHeight_UsesMountedStoreKey(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false).WithBlockHeight(4242)

	require.NotPanics(t, func() { mocaApp.saveUnbalancedBlockHeight(ctx) },
		"saveUnbalancedBlockHeight must resolve the recon store via the mounted key (app.GetKey)")

	h, ok := mocaApp.getUnbalancedBlockHeight(ctx)
	require.True(t, ok, "unbalanced height must be persisted")
	require.Equal(t, uint64(4242), h)
}
