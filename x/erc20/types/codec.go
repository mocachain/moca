package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

var (
	amino = codec.NewLegacyAmino()

	ModuleCdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	AminoCdc  = codec.NewAminoCodec(amino) //nolint:staticcheck
)

func init() {
	RegisterLegacyAminoCodec(amino)
	amino.Seal()
}

func RegisterInterfaces(_ codectypes.InterfaceRegistry) {}

func RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}
