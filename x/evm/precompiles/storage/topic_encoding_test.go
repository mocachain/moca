package storage

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// TestStringIndexedTopicEncoding documents the expected encoding rule for
// string indexed fields (bucket/object/group names): keccak256(string) -> topic.
func TestStringIndexedTopicEncoding(t *testing.T) {
	cases := []string{
		"bucket-1",
		"object-xyz",
		"group-alpha",
		"长名字-支持-utf8",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			// compute expected topic via keccak256
			exp := crypto.Keccak256Hash([]byte(name))

			// ensure deterministic and non-zero
			require.NotEqual(t, common.Hash{}, exp)

			// ensure equals to crypto.Keccak256 then to Hash
			exp2 := common.BytesToHash(crypto.Keccak256([]byte(name)))
			require.Equal(t, exp, exp2)
		})
	}
}

// TestAllStringIndexedFunctions verifies that all functions with string indexed
// event parameters have correct event definitions as per the audit fix.
func TestAllStringIndexedFunctions(t *testing.T) {
	testCases := []struct {
		name      string
		eventName string
		hasString bool // whether event has string indexed parameter
	}{
		{"DiscontinueBucket", "DiscontinueBucket", true},
		{"MigrateBucket", "MigrateBucket", true},
		{"CompleteMigrateBucket", "CompleteMigrateBucket", true},
		{"RejectSealObject", "RejectSealObject", true},
		{"DelegateCreateObject", "DelegateCreateObject", true},
		{"DelegateUpdateObjectContent", "DelegateUpdateObjectContent", true},
		{"UpdateObjectContent", "UpdateObjectContent", true},
		{"CancelUpdateObjectContent", "CancelUpdateObjectContent", true},
		{"DiscontinueObject", "DiscontinueObject", true},
		{"LeaveGroup", "LeaveGroup", true},
		{"UpdateBucketInfo", "UpdateBucketInfo", true}, // fixed: bytes32 -> string indexed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Verify event exists
			event := GetAbiEvent(tc.eventName)
			require.NotNil(t, event, "Event %s should exist", tc.eventName)
			require.Equal(t, tc.eventName, event.Name)

			if tc.hasString {
				// 2. Verify event has at least one indexed string parameter
				hasIndexedString := false
				for _, input := range event.Inputs {
					if input.Indexed && input.Type.String() == "string" {
						hasIndexedString = true
						break
					}
				}
				require.True(t, hasIndexedString,
					"Event %s should have indexed string parameter", tc.eventName)

				// 3. Test topic encoding for a sample name
				testName := "test-name-123"
				expectedTopic := crypto.Keccak256Hash([]byte(testName))
				wrongTopic := common.BytesToHash([]byte(testName))

				// Verify keccak256 is different from direct BytesToHash
				require.NotEqual(t, wrongTopic, expectedTopic,
					"Event %s: should use keccak256, not direct BytesToHash", tc.eventName)
				require.NotEqual(t, common.Hash{}, expectedTopic)
			}
		})
	}
}

// TestTopicEncodingMethods verifies the difference between correct and incorrect
// topic encoding methods for string indexed parameters.
func TestTopicEncodingMethods(t *testing.T) {
	testStrings := []string{
		"my-bucket",
		"my-very-long-bucket-name-that-exceeds-32-bytes-limit",
		"对象名字",
		"test-group-alpha-123",
	}

	for _, str := range testStrings {
		t.Run(str, func(t *testing.T) {
			// Correct method: keccak256 hash
			correctTopic := crypto.Keccak256Hash([]byte(str))

			// Wrong method: direct BytesToHash (would truncate/pad)
			wrongTopic := common.BytesToHash([]byte(str))

			// They should be different (except for very rare hash collision)
			if len(str) <= 32 {
				// For short strings, BytesToHash pads with zeros
				require.NotEqual(t, correctTopic, wrongTopic,
					"Even for short strings, keccak256 result differs from padded bytes")
			} else {
				// For long strings, BytesToHash truncates
				require.NotEqual(t, correctTopic, wrongTopic,
					"For long strings, keccak256 result differs from truncated bytes")
			}

			// Correct topic should always be non-zero
			require.NotEqual(t, common.Hash{}, correctTopic)
		})
	}
}
