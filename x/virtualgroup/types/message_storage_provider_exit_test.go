package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestMsgStorageProviderExit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgStorageProviderExit
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgStorageProviderExit{
				StorageProvider: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg:  *NewMsgStorageProviderExit(sample.RandAccAddress()),
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

func TestMsgStorageProviderExit_RouteTypeSignBytesSigners(t *testing.T) {
	addr := sample.RandAccAddress()
	msg := NewMsgStorageProviderExit(addr)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgStorageProviderExit, msg.Type())
	require.Equal(t, []sdk.AccAddress{addr}, msg.GetSigners())

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)
	require.Contains(t, string(bz), addr.String())
}
