package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("mc", "mcpub")
}

func TestMocaCoinDenom(t *testing.T) {
	testCases := []struct {
		name     string
		denom    string
		expError bool
	}{
		{
			"valid denom - native coin",
			"amoca",
			false,
		},
		{
			"valid denom - ibc coin",
			"ibc/7B2A4F6E798182988D77B6B884919AF617A73503FDAC27C916CD7A69A69013CF",
			false,
		},
		{
			"valid denom - ethereum address (ERC-20 contract)",
			"erc20/0x52908400098527886e0f7030069857D2E4169EE7",
			false,
		},
		{
			"invalid denom - only one character",
			"a",
			true,
		},
		{
			"invalid denom - too large (> 127 chars)",
			"ibc/7B2A4F6E798182988D77B6B884919AF617A73503FDAC27C916CD7A69A69013CF7B2A4F6E798182988D77B6B884919AF617A73503FDAC27C916CD7A69A69013CF",
			true,
		},
		{
			"invalid denom - starts with 0 but not followed by 'x'",
			"0a52908400098527886E0F7030069857D2E4169EE7",
			true,
		},
		{
			"invalid denom - hex address but 19 bytes long",
			"0x52908400098527886E0F7030069857D2E4169E",
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Case %s", tc.name), func(t *testing.T) {
			err := sdk.ValidateDenom(tc.denom)
			if tc.expError {
				require.Error(t, err, tc.name)
			} else {
				require.NoError(t, err, tc.name)
			}
		})
	}
}
