package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestMsgCompleteStorageProviderExit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCompleteStorageProviderExit
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgCompleteStorageProviderExit{
				StorageProvider: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid operator",
			msg: MsgCompleteStorageProviderExit{
				Operator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "valid address",
			msg:  *NewMsgCompleteStorageProviderExit(sample.RandAccAddress(), sample.RandAccAddress()),
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

func TestMsgCompleteStorageProviderExit_RouteTypeSignBytesSigners(t *testing.T) {
	operator := sample.RandAccAddress()
	sp := sample.RandAccAddress()
	msg := NewMsgCompleteStorageProviderExit(operator, sp)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCompleteStorageProviderExit, msg.Type())
	// A valid Operator takes priority over StorageProvider as the signer.
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)
	require.Contains(t, string(bz), sp.String())
}

func TestMsgCompleteStorageProviderExit_GetSigners_FallsBackToStorageProvider(t *testing.T) {
	sp := sample.RandAccAddress()
	msg := &MsgCompleteStorageProviderExit{
		StorageProvider: sp.String(),
		Operator:        "invalid_address",
	}
	// An unparseable Operator falls back to the (valid) StorageProvider.
	require.Equal(t, []sdk.AccAddress{sp}, msg.GetSigners())
}

func TestMsgCompleteStorageProviderExit_ValidateRuntime(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCompleteStorageProviderExit
		err  error
	}{
		{
			name: "valid operator",
			msg:  MsgCompleteStorageProviderExit{Operator: sample.RandAccAddressHex()},
		},
		{
			name: "invalid operator",
			msg:  MsgCompleteStorageProviderExit{Operator: "invalid_address"},
			err:  sdkerrors.ErrInvalidAddress,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateRuntime(sdk.Context{})
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}
