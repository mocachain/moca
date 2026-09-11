package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/permission/types"
)

func TestRegisterCodec(t *testing.T) {
	require.NotPanics(t, func() {
		types.RegisterCodec(codec.NewLegacyAmino())
	})
}

func TestRegisterInterfaces(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()

	// Neither the Msg nor its response is known to a fresh registry.
	require.Error(t, registry.EnsureRegistered(&types.MsgUpdateParams{}))
	require.Error(t, registry.EnsureRegistered(&types.MsgUpdateParamsResponse{}))

	types.RegisterInterfaces(registry)

	// RegisterInterfaces registers MsgUpdateParams directly as an sdk.Msg
	// implementation, and its response type indirectly via RegisterMsgServiceDesc.
	require.NoError(t, registry.EnsureRegistered(&types.MsgUpdateParams{}))
	require.NoError(t, registry.EnsureRegistered(&types.MsgUpdateParamsResponse{}))
}
