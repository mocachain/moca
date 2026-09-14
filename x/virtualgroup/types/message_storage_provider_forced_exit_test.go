package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestMsgStorageProviderForcedExit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name      string
		msg       MsgStorageProviderForcedExit
		expErr    bool
		expErrMsg string
	}{
		{
			name: "invalid authority",
			msg: MsgStorageProviderForcedExit{
				Authority: errInvalidAuthorityAddress,
			},
			expErr:    true,
			expErrMsg: errInvalidAuthorityAddress,
		},
		{
			name: "invalid address",
			msg: MsgStorageProviderForcedExit{
				Authority:       "0xaE4F00015B40eE402a7f05E46757c18Df86E49E1",
				StorageProvider: "invalid_address",
			},
			expErr:    true,
			expErrMsg: "invalid address",
		},
		{
			name: "valid address",
			msg:  *NewMsgStorageProviderForcedExit(sample.RandAccAddress().String(), sample.RandAccAddress()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.expErr {
				require.Contains(t, err.Error(), tt.expErrMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgStorageProviderForcedExit_RouteTypeSignBytesSigners(t *testing.T) {
	authority := sample.RandAccAddress()
	msg := NewMsgStorageProviderForcedExit(authority.String(), sample.RandAccAddress())

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgStorageProviderForcedExit, msg.Type())
	require.Equal(t, []sdk.AccAddress{authority}, msg.GetSigners())

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)
	require.Contains(t, string(bz), authority.String())
}
