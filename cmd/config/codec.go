package config

import (
	"encoding/hex"
	"strings"

	"cosmossdk.io/core/address"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
)

var _ address.Codec = &MultiPrefixBech32Codec{}

type MultiPrefixBech32Codec struct {
	primaryCodec   address.Codec
	secondaryCodec address.Codec
	outputPrefix   string
}

func NewMultiPrefixBech32Codec(primaryPrefix, secondaryPrefix string) address.Codec {
	return &MultiPrefixBech32Codec{
		primaryCodec:   addresscodec.NewBech32Codec(primaryPrefix),
		secondaryCodec: addresscodec.NewBech32Codec(secondaryPrefix),
		outputPrefix:   primaryPrefix,
	}
}

func (bc *MultiPrefixBech32Codec) StringToBytes(text string) ([]byte, error) {
	// moca renders addresses as EIP-55 0x-hex; accept hex input directly.
	// hex.DecodeString accepts mixed-case (checksummed) input.
	if len(text) >= 2 && (text[0:2] == "0x" || text[0:2] == "0X") {
		return hex.DecodeString(text[2:])
	}

	if strings.HasPrefix(text, bc.outputPrefix) {
		return bc.primaryCodec.StringToBytes(text)
	}

	bytes, err := bc.secondaryCodec.StringToBytes(text)
	if err == nil {
		return bytes, nil
	}

	return bc.primaryCodec.StringToBytes(text)
}

func (bc *MultiPrefixBech32Codec) BytesToString(bz []byte) (string, error) {
	return bc.primaryCodec.BytesToString(bz)
}

func NewMultiPrefixBech32AccCodec() address.Codec {
	return NewMultiPrefixBech32Codec(Bech32PrefixAccAddr, "cosmos")
}

func NewMultiPrefixBech32ValCodec() address.Codec {
	legacyPrefix := "cosmos" + "valoper"
	return NewMultiPrefixBech32Codec(Bech32PrefixValAddr, legacyPrefix)
}

func NewMultiPrefixBech32ConsCodec() address.Codec {
	legacyPrefix := "cosmos" + "valcons"
	return NewMultiPrefixBech32Codec(Bech32PrefixConsAddr, legacyPrefix)
}
