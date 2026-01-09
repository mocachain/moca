package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.True(t, cfg.JSONRPC.Enable)
	require.Equal(t, cfg.JSONRPC.Address, DefaultJSONRPCAddress)
	require.Equal(t, cfg.JSONRPC.WsAddress, DefaultJSONRPCWsAddress)

	// Hardforks should be initialized as an empty map
	require.NotNil(t, cfg.Hardforks)
	require.Empty(t, cfg.Hardforks)
}

func TestHardforksValidation(t *testing.T) {
	testCases := []struct {
		name        string
		hardforks   map[string]HardforkEntry
		expectError bool
		errContains string
	}{
		{
			name:        "empty hardforks is valid",
			hardforks:   map[string]HardforkEntry{},
			expectError: false,
		},
		{
			name: "valid single hardfork",
			hardforks: map[string]HardforkEntry{
				"1200": {Name: "testnet-gov-param-fix", Info: `{"binaries":{}}`},
			},
			expectError: false,
		},
		{
			name: "valid multiple hardforks",
			hardforks: map[string]HardforkEntry{
				"1200": {Name: "testnet-gov-param-fix"},
				"5000": {Name: "another-upgrade", Info: "some info"},
			},
			expectError: false,
		},
		{
			name: "empty plan name fails",
			hardforks: map[string]HardforkEntry{
				"1200": {Name: "", Info: "some info"},
			},
			expectError: true,
			errContains: "plan name cannot be empty",
		},
		{
			name: "non-numeric height fails",
			hardforks: map[string]HardforkEntry{
				"invalid": {Name: "some-upgrade"},
			},
			expectError: true,
			errContains: "must be a positive integer",
		},
		{
			name: "zero height fails",
			hardforks: map[string]HardforkEntry{
				"0": {Name: "some-upgrade"},
			},
			expectError: true,
			errContains: "must be a positive integer",
		},
		{
			name: "negative height fails",
			hardforks: map[string]HardforkEntry{
				"-100": {Name: "some-upgrade"},
			},
			expectError: true,
			errContains: "must be a positive integer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewDefaultAppConfig("amoca")
			cfg.Hardforks = tc.hardforks

			err := cfg.ValidateBasic()
			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseHardforks(t *testing.T) {
	testCases := []struct {
		name     string
		config   map[string]interface{}
		expected map[string]HardforkEntry
	}{
		{
			name:     "empty hardforks",
			config:   nil,
			expected: map[string]HardforkEntry{},
		},
		{
			name: "single hardfork with name only",
			config: map[string]interface{}{
				"1200": map[string]interface{}{
					"name": "testnet-gov-param-fix",
				},
			},
			expected: map[string]HardforkEntry{
				"1200": {Name: "testnet-gov-param-fix", Info: ""},
			},
		},
		{
			name: "single hardfork with name and info",
			config: map[string]interface{}{
				"5000": map[string]interface{}{
					"name": "another-upgrade",
					"info": `{"binaries":{}}`,
				},
			},
			expected: map[string]HardforkEntry{
				"5000": {Name: "another-upgrade", Info: `{"binaries":{}}`},
			},
		},
		{
			name: "multiple hardforks",
			config: map[string]interface{}{
				"1200": map[string]interface{}{
					"name": "first-upgrade",
				},
				"5000": map[string]interface{}{
					"name": "second-upgrade",
					"info": "upgrade info",
				},
			},
			expected: map[string]HardforkEntry{
				"1200": {Name: "first-upgrade", Info: ""},
				"5000": {Name: "second-upgrade", Info: "upgrade info"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			if tc.config != nil {
				v.Set("hardforks", tc.config)
			}

			result := parseHardforks(v)
			require.Equal(t, tc.expected, result)
		})
	}
}
