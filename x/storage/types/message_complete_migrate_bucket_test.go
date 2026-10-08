package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func TestMsgCompleteMigrateBucket_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCompleteMigrateBucket
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgCompleteMigrateBucket{
				Operator:   "invalid_address",
				BucketName: "bucketname",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "bucketname",
				GlobalVirtualGroupFamilyId: 1,
				GvgMappings:                []*GVGMapping{{1, 2, []byte("xxxxxxxxxxx")}},
			},
		}, {
			name: "invalid bucket name",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "TestBucket",
				GlobalVirtualGroupFamilyId: 1,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "missing global virtual group family id",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "bucketname",
				GlobalVirtualGroupFamilyId: vgtypes.NoSpecifiedFamilyID,
			},
			err: gnfderrors.ErrInvalidMessage,
		}, {
			name: "zero src gvg id",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "bucketname",
				GlobalVirtualGroupFamilyId: 1,
				GvgMappings:                []*GVGMapping{{0, 2, []byte("xxxxxxxxxxx")}},
			},
			err: ErrInvalidGlobalVirtualGroup,
		}, {
			name: "zero dst gvg id",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "bucketname",
				GlobalVirtualGroupFamilyId: 1,
				GvgMappings:                []*GVGMapping{{1, 0, []byte("xxxxxxxxxxx")}},
			},
			err: ErrInvalidGlobalVirtualGroup,
		}, {
			name: "src equals dst gvg id",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "bucketname",
				GlobalVirtualGroupFamilyId: 1,
				GvgMappings:                []*GVGMapping{{5, 5, []byte("xxxxxxxxxxx")}},
			},
			err: ErrInvalidGlobalVirtualGroup,
		}, {
			name: "missing secondary sp bls signature",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "bucketname",
				GlobalVirtualGroupFamilyId: 1,
				GvgMappings:                []*GVGMapping{{1, 2, nil}},
			},
			err: gnfderrors.ErrInvalidBlsSignature,
		}, {
			name: "duplicate src gvg id",
			msg: MsgCompleteMigrateBucket{
				Operator:                   sample.RandAccAddressHex(),
				BucketName:                 "bucketname",
				GlobalVirtualGroupFamilyId: 1,
				GvgMappings: []*GVGMapping{
					{1, 2, []byte("xxxxxxxxxxx")},
					{1, 3, []byte("yyyyyyyyyyy")},
				},
			},
			err: ErrInvalidGlobalVirtualGroup,
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

func TestNewMsgCompleteMigrateBucket(t *testing.T) {
	operator := sample.RandAccAddress()
	mappings := []*GVGMapping{{1, 2, []byte("xxxxxxxxxxx")}}
	msg := NewMsgCompleteMigrateBucket(operator, "bucketname", 9, mappings)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, "bucketname", msg.BucketName)
	require.Equal(t, uint32(9), msg.GlobalVirtualGroupFamilyId)
	require.Equal(t, mappings, msg.GvgMappings)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCompleteMigrateBucket, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid_address"
	require.Panics(t, func() { bad.GetSigners() })
}
