package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

func TestMsgCancelSwapOut_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCancelSwapOut
		err  error
	}{
		{
			name: "valid address",
			msg: *NewMsgCancelSwapOut(
				sample.RandAccAddress(),
				1,
				[]uint32{},
			),
		},
		{
			name: "invalid address",
			msg: MsgCancelSwapOut{
				StorageProvider:            "invalid_address",
				GlobalVirtualGroupFamilyId: 1,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid gvg groups",
			msg: MsgCancelSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupIds:      []uint32{1, 2},
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid gvg groups",
			msg: MsgCancelSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupIds:      []uint32{},
			},
			err: gnfderrors.ErrInvalidMessage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgCancelSwapOut_RouteTypeSignBytesSigners(t *testing.T) {
	addr := sample.RandAccAddress()
	msg := NewMsgCancelSwapOut(addr, 1, []uint32{})

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCancelSwapOut, msg.Type())
	require.Equal(t, []sdk.AccAddress{addr}, msg.GetSigners())

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)
	require.Contains(t, string(bz), addr.String())
}
