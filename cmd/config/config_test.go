package config

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// TestSetBech32Prefixes_AddressVerifier pins the address-length invariant: moca renders
// EIP-55 0x-hex, so every address must resolve to exactly 20 bytes. Without the verifier
// the SDK default accepts 1..MaxAddrLen bytes, and a bech32-decoded value of any length
// would be accepted and then truncated by String().
func TestSetBech32Prefixes_AddressVerifier(t *testing.T) {
	config := sdk.NewConfig()
	SetBech32Prefixes(config)

	verify := config.GetAddressVerifier()
	require.NotNil(t, verify, "SetBech32Prefixes must register an address verifier")

	require.NoError(t, verify(make([]byte, sdk.EthAddressLength)), "a 20-byte address is valid")
	for _, n := range []int{0, 1, 19, 21, 32, 255} {
		require.Error(t, verify(make([]byte, n)), "a %d-byte address must be rejected", n)
	}
}
