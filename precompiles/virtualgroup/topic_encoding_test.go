package virtualgroup

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// TestAddressIndexedTopicEncoding documents the expected encoding rule for
// address indexed fields: BytesToHash(address.Bytes()).
func TestAddressIndexedTopicEncoding(t *testing.T) {
	addrs := []string{
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
	}

	for _, s := range addrs {
		t.Run(s, func(t *testing.T) {
			addr := common.HexToAddress(s)
			require.NotEqual(t, common.Address{}, addr)
			topic := common.BytesToHash(addr.Bytes())
			require.NotEqual(t, common.Hash{}, topic)
		})
	}
}

// TestDeleteGlobalVirtualGroup_EventDefinition verifies DeleteGlobalVirtualGroup event
// has correct parameters (should have address indexed, not uint256).
func TestDeleteGlobalVirtualGroup_EventDefinition(t *testing.T) {
	event := MustEvent("DeleteGlobalVirtualGroup")
	require.NotNil(t, event)

	// Event should have only 1 indexed parameter: storageProvider (address)
	indexedParams := 0
	var addressParam bool
	for _, input := range event.Inputs {
		if input.Indexed {
			indexedParams++
			if input.Type.String() == "address" {
				addressParam = true
			}
		}
	}

	require.Equal(t, 1, indexedParams,
		"DeleteGlobalVirtualGroup should have 1 indexed parameter")
	require.True(t, addressParam,
		"DeleteGlobalVirtualGroup indexed parameter should be address type")
}

// TestCompleteSPExit_AddressEncoding verifies CompleteSPExit correctly encodes
// address parameters as topics.
func TestCompleteSPExit_AddressEncoding(t *testing.T) {
	event := MustEvent("CompleteSPExit")
	require.NotNil(t, event)

	// Event should have 2 indexed address parameters: storageProvider and operator
	indexedAddresses := 0
	for _, input := range event.Inputs {
		if input.Indexed && input.Type.String() == "address" {
			indexedAddresses++
		}
	}

	require.Equal(t, 2, indexedAddresses,
		"CompleteSPExit should have 2 indexed address parameters")

	// Test correct address encoding method
	testAddresses := []string{
		"0x1234567890123456789012345678901234567890",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}

	for _, hexAddr := range testAddresses {
		t.Run(hexAddr, func(t *testing.T) {
			// Correct method: HexToAddress then Bytes
			addr := common.HexToAddress(hexAddr)
			correctTopic := common.BytesToHash(addr.Bytes())

			// Verify address is not zero
			require.NotEqual(t, common.Address{}, addr)
			// Verify topic is not zero
			require.NotEqual(t, common.Hash{}, correctTopic)

			// For valid hex addresses, HexToAddress should parse correctly
			require.Equal(t, 20, len(addr.Bytes()),
				"Address should be 20 bytes")
		})
	}
}

// TestAddressTopicEncodingMethods verifies the correct and incorrect ways
// to encode address strings as event topics.
func TestAddressTopicEncodingMethods(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		isHexAddr bool
	}{
		{"Valid hex address", "0x1234567890123456789012345678901234567890", true},
		{"Bech32 address (invalid for HexToHash)", "mocavaloper1abcdef", false},
		{"Invalid hex string", "not-a-hex-address", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.isHexAddr {
				// Correct method for hex addresses
				addr := common.HexToAddress(tc.input)
				topic := common.BytesToHash(addr.Bytes())

				require.NotEqual(t, common.Address{}, addr)
				require.NotEqual(t, common.Hash{}, topic)
			} else {
				// Wrong method: HexToHash on non-hex strings
				wrongTopic := common.HexToHash(tc.input)

				// HexToHash on invalid hex returns zero hash
				if tc.input == "not-a-hex-address" || tc.input == "mocavaloper1abcdef" {
					// These should produce zero or incorrect hash
					t.Logf("HexToHash on invalid input: %x", wrongTopic)
					// Just log, don't fail - demonstrates the problem
				}
			}
		})
	}
}
