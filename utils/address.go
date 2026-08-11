package utils

import (
	"github.com/ethereum/go-ethereum/common"
)

// SameAddress reports whether two strings denote the same account.
//
// Accounts on this chain are Ethereum accounts: a 20-byte value, written in hex.
// How that hex is written is not part of the identity — casing carries no meaning
// and the 0x prefix is optional — so both sides are decoded and the addresses
// themselves are compared. A case-insensitive string comparison is not enough: it
// reports "0xAb…" and "Ab…" as different accounts.
//
// Anything that is not a 20-byte hex address denotes no account and matches
// nothing, including a string identical to it and including the empty string.
// Comparing two such strings verbatim would report them as the same account,
// which is a claim this cannot make.
//
// Use SameAddressOrEmpty where an address is legitimately unset and two unset
// values should count as the same.
func SameAddress(a, b string) bool {
	if !common.IsHexAddress(a) || !common.IsHexAddress(b) {
		return false
	}

	return common.HexToAddress(a) == common.HexToAddress(b)
}

// SameAddressOrEmpty reports whether two strings denote the same account, treating
// the empty string as a value in its own right: it is the absence of an address
// rather than an address, so it matches another empty string and nothing else.
//
// This is for fields where being unset is legitimate — a bucket's payment address,
// for instance — and where "both unset" should read as unchanged. Everything else
// is decided by SameAddress.
func SameAddressOrEmpty(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}

	return SameAddress(a, b)
}
