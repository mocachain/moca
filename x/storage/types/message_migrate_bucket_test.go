package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/types/common"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

func TestMsgMigrateBucket_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgMigrateBucket
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgMigrateBucket{
				Operator:   "invalid_address",
				BucketName: "bucketname",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgMigrateBucket{
				Operator:             sample.RandAccAddressHex(),
				BucketName:           "bucketname",
				DstPrimarySpId:       1,
				DstPrimarySpApproval: &common.Approval{ExpiredHeight: 10, Sig: []byte("XXXTentacion")},
			},
		}, {
			name: "invalid bucket name",
			msg: MsgMigrateBucket{
				Operator:             sample.RandAccAddressHex(),
				BucketName:           "TestBucket",
				DstPrimarySpId:       1,
				DstPrimarySpApproval: &common.Approval{ExpiredHeight: 10, Sig: []byte("XXXTentacion")},
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "missing dst primary sp id",
			msg: MsgMigrateBucket{
				Operator:             sample.RandAccAddressHex(),
				BucketName:           "bucketname",
				DstPrimarySpApproval: &common.Approval{ExpiredHeight: 10, Sig: []byte("XXXTentacion")},
			},
			err: gnfderrors.ErrInvalidMessage,
		}, {
			name: "nil approval",
			msg: MsgMigrateBucket{
				Operator:       sample.RandAccAddressHex(),
				BucketName:     "bucketname",
				DstPrimarySpId: 1,
			},
			err: ErrInvalidApproval,
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

func TestNewMsgMigrateBucket(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgMigrateBucket(operator, "bucketname", 7)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, "bucketname", msg.BucketName)
	require.Equal(t, uint32(7), msg.DstPrimarySpId)
	require.NotNil(t, msg.DstPrimarySpApproval)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgMigrateBucket, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	msg.DstPrimarySpApproval.Sig = []byte("sig-one")
	otherSig := *msg
	otherApproval := *msg.DstPrimarySpApproval
	otherApproval.Sig = []byte("sig-two")
	otherSig.DstPrimarySpApproval = &otherApproval
	requireApprovalBytesStripSignature(t, msg, &otherSig)

	bad := *msg
	bad.Operator = "invalid_address"
	require.Panics(t, func() { bad.GetSigners() })
}
