package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

func TestMsgCancelMigrateBucket_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCancelMigrateBucket
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgCancelMigrateBucket{
				Operator:   "invalid_address",
				BucketName: testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgCancelMigrateBucket{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
			},
		}, {
			name: "invalid bucket name",
			msg: MsgCancelMigrateBucket{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "TestBucket",
			},
			err: gnfderrors.ErrInvalidBucketName,
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

func TestNewMsgCancelMigrateBucket(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgCancelMigrateBucket(operator, testBucketName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCancelMigrateBucket, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid_address"
	require.Panics(t, func() { bad.GetSigners() })
}
