package types

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRegisterCodec(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	RegisterCodec(cdc)

	// The sdk.Msg interface itself is normally registered once at the app
	// level (by auth); register it here too so we can observe, through the
	// public JSON API, that RegisterCodec actually registered the concrete
	// types under an interface-typed slot -- marshaling the concrete struct
	// directly would look the same whether or not RegisterConcrete ran.
	cdc.RegisterInterface((*sdk.Msg)(nil), nil)
	wrapper := struct{ Msg sdk.Msg }{Msg: &MsgSubmit{BucketName: "bucket"}}
	bz, err := cdc.MarshalJSON(wrapper)
	require.NoError(t, err)
	require.Contains(t, string(bz), "challenge/Submit")
}

func TestRegisterInterfaces(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	RegisterInterfaces(registry)

	for _, msg := range []sdk.Msg{&MsgSubmit{}, &MsgAttest{}, &MsgUpdateParams{}} {
		resolved, err := registry.Resolve(cdctypes.MsgTypeURL(msg))
		require.NoError(t, err)
		require.IsType(t, msg, resolved)
	}
}
