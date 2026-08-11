package utils_test

import (
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/utils"
)

const notAnAddress = "not-an-address"

func TestSameAddress(t *testing.T) {
	canonical := sample.RandAccAddress().String()
	lowered := strings.ToLower(canonical)
	unprefixed := canonical[2:]
	require.NotEqual(t, canonical, lowered, "the sample address must contain hex letters")

	other := sample.RandAccAddress().String()
	bech32Form, err := bech32.ConvertAndEncode("mc", sample.RandAccAddress().Bytes())
	require.NoError(t, err)

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", canonical, canonical, true},
		{"casing differs only", canonical, lowered, true},
		{"casing differs only, reversed", lowered, canonical, true},

		// The 0x prefix is optional, so these are the same account even though a
		// case-insensitive string comparison reports them as different.
		{"prefix present on one side", unprefixed, canonical, true},
		{"prefix absent on both sides", unprefixed, strings.ToLower(unprefixed), true},

		{"different accounts", canonical, other, false},

		// Accounts here are Ethereum accounts, so a bech32 rendering is not an address
		// form this chain uses, and neither is the empty string or anything else that
		// is not 20 bytes of hex. Each denotes no account and matches nothing.
		{"bech32 form of the same account", bech32Form, canonical, false},
		{"both empty", "", "", false},
		{"empty against an address", "", canonical, false},
		{"address against empty", canonical, "", false},
		{"identical non-addresses", notAnAddress, notAnAddress, false},
		{"differing non-addresses", notAnAddress, "also-not", false},
		{"non-address against an address", notAnAddress, canonical, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, utils.SameAddress(tc.a, tc.b))
		})
	}
}

func TestSameAddressOrEmpty(t *testing.T) {
	canonical := sample.RandAccAddress().String()
	lowered := strings.ToLower(canonical)

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		// Unset is a value here, and matches only another unset value.
		{"both unset", "", "", true},
		{"unset against an address", "", canonical, false},
		{"address against unset", canonical, "", false},

		// Everything else is decided by SameAddress.
		{"casing differs only", canonical, lowered, true},
		{"different accounts", canonical, sample.RandAccAddress().String(), false},
		{"identical non-addresses", notAnAddress, notAnAddress, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, utils.SameAddressOrEmpty(tc.a, tc.b))
		})
	}
}

// The one behavior that separates the two: only SameAddressOrEmpty matches unset
// against unset.
func TestSameAddress_DiffersFromOrEmptyOnlyWhenUnset(t *testing.T) {
	require.False(t, utils.SameAddress("", ""))
	require.True(t, utils.SameAddressOrEmpty("", ""))
}

// A case-insensitive string comparison is not a substitute: it reports an address
// written without the 0x prefix as a different account.
func TestSameAddress_BeatsCaseInsensitiveStringCompare(t *testing.T) {
	canonical := sample.RandAccAddress().String()
	unprefixed := canonical[2:]

	require.False(t, strings.EqualFold(unprefixed, canonical), "precondition for this test")
	require.True(t, utils.SameAddress(unprefixed, canonical))
}

func TestSameAddress_EmptyDoesNotPanic(t *testing.T) {
	addr := sample.RandAccAddress().String()
	require.NotPanics(t, func() {
		utils.SameAddress("", "")
		utils.SameAddress("", addr)
		utils.SameAddressOrEmpty("", "")
		utils.SameAddressOrEmpty(addr, "")
	})
}
