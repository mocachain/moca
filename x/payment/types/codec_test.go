package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

// TestRegisterCodec checks that RegisterCodec actually teaches the amino
// codec the concrete type's name: before registration amino marshals the
// message with no type wrapper, after registration it adds one. The
// "before" check uses its own codec instance -- marshaling a type through an
// amino codec caches an unregistered TypeInfo for it on that instance, and
// RegisterConcrete panics if it is later registered on top of that cache.
func TestRegisterCodec(t *testing.T) {
	msg := types.NewMsgCreatePaymentAccount(sample.RandAccAddressHex())

	before, err := codec.NewLegacyAmino().MarshalJSON(msg)
	require.NoError(t, err)
	require.NotContains(t, string(before), "payment/CreatePaymentAccount")

	cdc := codec.NewLegacyAmino()
	require.NotPanics(t, func() { types.RegisterCodec(cdc) })

	after, err := cdc.MarshalJSON(msg)
	require.NoError(t, err)
	require.Contains(t, string(after), "payment/CreatePaymentAccount")
}

// TestRegisterInterfaces checks that RegisterInterfaces actually wires the
// message types into the registry: unpacking an Any built without a cached
// value fails before registration (the registry has no concrete type for the
// type URL) and succeeds after.
func TestRegisterInterfaces(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	msg := types.NewMsgCreatePaymentAccount(sample.RandAccAddressHex())

	bz, err := proto.Marshal(msg)
	require.NoError(t, err)
	packedAny := &cdctypes.Any{TypeUrl: cdctypes.MsgTypeURL(msg), Value: bz}

	var before sdk.Msg
	require.Error(t, registry.UnpackAny(packedAny, &before))

	require.NotPanics(t, func() { types.RegisterInterfaces(registry) })

	var after sdk.Msg
	require.NoError(t, registry.UnpackAny(packedAny, &after))
	require.Equal(t, msg, after)
}
