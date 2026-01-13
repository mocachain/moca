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
