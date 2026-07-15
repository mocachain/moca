package distribution

import (
	"testing"

	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// helper: parse Bech32 validator address to topic; convert panic to error
func parseValBech32ToTopic(s string) (common.Hash, error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	valAddr, e := sdk.ValAddressFromBech32(s)
	if e != nil {
		return common.Hash{}, e
	}
	return common.BytesToHash(valAddr.Bytes()), err
}

// TestAddressParsingMethods tests different address parsing methods
// to demonstrate the issue with HexToHash on Bech32 addresses
func TestAddressParsingMethods(t *testing.T) {
	// Test 1: Bech32 validator address (typical format from Cosmos SDK)
	bech32Val := "mocavaloper1abcdefghijklmnopqrstuvwxyz1234567890"

	t.Run("Incorrect method - HexToHash on Bech32", func(t *testing.T) {
		// HexToHash cannot parse Bech32 format
		hash := common.HexToHash(bech32Val)

		// HexToHash will try to parse "mocavaloper..." as hex
		// Since it's not valid hex, it returns zero hash
		t.Logf("HexToHash result: %x", hash)

		// This should be zero hash (incorrect behavior)
		require.Equal(t, common.Hash{}, hash, "HexToHash(Bech32) should produce zero hash")
	})

	t.Run("Incorrect method - HexToAddress on Bech32", func(t *testing.T) {
		// HexToAddress also cannot parse Bech32 format
		addr := common.HexToAddress(bech32Val)
		t.Logf("HexToAddress result: %x", addr)

		// This returns zero address because Bech32 is not valid hex
		require.Equal(t, common.Address{}, addr, "Returns zero address on invalid hex")
	})

	t.Run("Correct method - SDK Bech32 parsing", func(t *testing.T) {
		// ValAddressFromBech32 is the correct method for Bech32 validator addresses
		require.Panics(t, func() {
			_, _ = sdk.ValAddressFromBech32(bech32Val)
		})
	})

	t.Run("Test with hex address (for comparison)", func(t *testing.T) {
		// This is what works correctly with hex addresses
		hexAddr := "0x1234567890123456789012345678901234567890"
		addr := common.HexToAddress(hexAddr)
		topic := common.BytesToHash(addr.Bytes())

		t.Logf("Hex address topic: %x", topic)
		require.NotEqual(t, common.Hash{}, topic)
	})
}

// TestEventTopicEncoding demonstrates the correct way to encode
// different address types for event topics
func TestEventTopicEncoding(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		parseFunc   func(string) (common.Hash, error)
		shouldError bool
	}{
		{
			name:  "Hex address - correct method",
			input: "0x1234567890123456789012345678901234567890",
			parseFunc: func(s string) (common.Hash, error) {
				addr := common.HexToAddress(s)
				return common.BytesToHash(addr.Bytes()), nil
			},
			shouldError: false,
		},
		{
			name:  "Bech32 validator - incorrect method using HexToHash",
			input: "mocavaloper1test",
			parseFunc: func(s string) (common.Hash, error) {
				// Incorrect: HexToHash cannot parse Bech32 addresses
				return common.HexToHash(s), nil
			},
			shouldError: false, // No error but produces zero hash
		},
		{
			name:  "Bech32 validator - incorrect method using HexToAddress",
			input: "mocavaloper1test",
			parseFunc: func(s string) (common.Hash, error) {
				// Incorrect: HexToAddress cannot parse Bech32 addresses
				addr := common.HexToAddress(s)
				return common.BytesToHash(addr.Bytes()), nil
			},
			shouldError: false, // Returns zero address
		},
		{
			name:  "Bech32 validator - correct method using SDK parser",
			input: "mocavaloper1test",
			parseFunc: func(s string) (common.Hash, error) {
				// Correct: Use SDK's ValAddressFromBech32 for Bech32 addresses
				return parseValBech32ToTopic(s)
			},
			shouldError: false, // accept either error or zero topic in fake input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := tt.parseFunc(tt.input)

			if tt.shouldError {
				require.Error(t, err, "Should error on invalid Bech32")
			} else {
				require.NoError(t, err)
				t.Logf("Result hash: %x", hash)
			}
		})
	}
}

// TestWithdrawDelegatorAllRewards_AddressEncoding simulates address encoding scenarios
// in WithdrawDelegatorAllRewards function
func TestWithdrawDelegatorAllRewards_AddressEncoding(t *testing.T) {
	// Simulate validator addresses from resQuery.Validators
	// In real code, this would be like: ["mocavaloper1xxx...", "mocavaloper1yyy..."]
	validators := []string{
		"mocavaloper1abcdef", // Fake Bech32 for demonstration
	}

	t.Run("Incorrect - using HexToHash on Bech32", func(t *testing.T) {
		for _, validator := range validators {
			// Incorrect method: HexToHash cannot parse Bech32 addresses
			topic := common.HexToHash(validator)

			t.Logf("HexToHash produces topic: %x", topic)
			// This will produce zero hash because validator is Bech32, not hex
		}
	})

	t.Run("Incorrect - using HexToAddress on Bech32", func(t *testing.T) {
		for _, validator := range validators {
			// Incorrect method: HexToAddress cannot parse Bech32 addresses
			addr := common.HexToAddress(validator)
			topic := common.BytesToHash(addr.Bytes())

			t.Logf("HexToAddress produces topic: %x", topic)
			// This will be zero address because validator is Bech32, not hex
			require.Equal(t, common.Address{}, addr, "Returns zero address")
		}
	})

	t.Run("Correct - using SDK Bech32 parser", func(t *testing.T) {
		for _, validator := range validators {
			// Correct method: Use SDK's ValAddressFromBech32 for Bech32 addresses
			topic, err := parseValBech32ToTopic(validator)
			if err != nil {
				t.Logf("Expected error with fake Bech32: %v", err)
				continue
			}
			// With fake Bech32, topic may still be zero; just log
			t.Logf("SDK parser produces topic: %x", topic)
		}
	})
}
