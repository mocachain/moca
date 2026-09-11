package types

import (
	"strings"
	"testing"

	"cosmossdk.io/math"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/types/common"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

func TestMsgCreateObject_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCreateObject
		err  error
	}{
		{
			name: "normal",
			msg: MsgCreateObject{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        testObjectName,
				PayloadSize:       1024,
				Visibility:        VISIBILITY_TYPE_PRIVATE,
				ContentType:       "content-type",
				PrimarySpApproval: &common.Approval{},
				ExpectChecksums:   [][]byte{sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum()},
			},
		}, {
			name: "invalid object name",
			msg: MsgCreateObject{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        "",
				PayloadSize:       1024,
				Visibility:        VISIBILITY_TYPE_PRIVATE,
				ContentType:       "content-type",
				PrimarySpApproval: &common.Approval{},
				ExpectChecksums:   [][]byte{sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum()},
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "invalid object name",
			msg: MsgCreateObject{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        "../object",
				PayloadSize:       1024,
				Visibility:        VISIBILITY_TYPE_PRIVATE,
				ContentType:       "content-type",
				PrimarySpApproval: &common.Approval{},
				ExpectChecksums:   [][]byte{sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum()},
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "invalid object name",
			msg: MsgCreateObject{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        "//object",
				PayloadSize:       1024,
				Visibility:        VISIBILITY_TYPE_PRIVATE,
				ContentType:       "content-type",
				PrimarySpApproval: &common.Approval{},
				ExpectChecksums:   [][]byte{sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum()},
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "invalid creator address",
			msg: MsgCreateObject{
				Creator:           "invalid address",
				BucketName:        testBucketName,
				ObjectName:        testObjectName,
				Visibility:        VISIBILITY_TYPE_PRIVATE,
				PrimarySpApproval: &common.Approval{},
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "nil approval",
			msg: MsgCreateObject{
				Creator:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_PRIVATE,
			},
			err: ErrInvalidApproval,
		}, {
			name: "invalid bucket name",
			msg: MsgCreateObject{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        "TestBucket",
				ObjectName:        testObjectName,
				Visibility:        VISIBILITY_TYPE_PRIVATE,
				PrimarySpApproval: &common.Approval{},
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid checksums",
			msg: MsgCreateObject{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        testObjectName,
				Visibility:        VISIBILITY_TYPE_PRIVATE,
				PrimarySpApproval: &common.Approval{},
				ExpectChecksums:   [][]byte{[]byte("too-short")},
			},
			err: gnfderrors.ErrInvalidChecksum,
		}, {
			name: "unspecified visibility",
			msg: MsgCreateObject{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        testObjectName,
				Visibility:        VISIBILITY_TYPE_UNSPECIFIED,
				PrimarySpApproval: &common.Approval{},
			},
			err: ErrInvalidVisibility,
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

func TestMsgCancelCreateObject_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCancelCreateObject
		err  error
	}{
		{
			name: "basic",
			msg: MsgCancelCreateObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
		}, {
			name: "invalid operator address",
			msg: MsgCancelCreateObject{
				Operator:   "invalid address",
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgCancelCreateObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "TestBucket",
				ObjectName: testObjectName,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgCancelCreateObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
			},
			err: gnfderrors.ErrInvalidObjectName,
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

func TestMsgDeleteObject_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDeleteObject
		err  error
	}{
		{
			name: "normal",
			msg: MsgDeleteObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
		}, {
			name: "invalid operator address",
			msg: MsgDeleteObject{
				Operator:   "invalid address",
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgDeleteObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "TestBucket",
				ObjectName: testObjectName,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgDeleteObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
			},
			err: gnfderrors.ErrInvalidObjectName,
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

func TestMsgCopyObject_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCopyObject
		err  error
	}{
		{
			name: "valid address",
			msg: MsgCopyObject{
				Operator:      sample.RandAccAddressHex(),
				SrcBucketName: testBucketName,
				SrcObjectName: testObjectName,
				DstBucketName: "dst" + testBucketName,
				DstObjectName: "dst" + testObjectName,
				DstPrimarySpApproval: &common.Approval{
					ExpiredHeight: 100,
					Sig:           []byte("xxx"),
				},
			},
		},
		{
			name: "invalid address",
			msg: MsgCopyObject{
				Operator:      "invalid address",
				SrcBucketName: testBucketName,
				SrcObjectName: testObjectName,
				DstBucketName: "dst" + testBucketName,
				DstObjectName: "dst" + testObjectName,
				DstPrimarySpApproval: &common.Approval{
					ExpiredHeight: 100,
					Sig:           []byte("xxx"),
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "empty approval",
			msg: MsgCopyObject{
				Operator:             sample.RandAccAddressHex(),
				SrcBucketName:        testBucketName,
				SrcObjectName:        testObjectName,
				DstBucketName:        "dst" + testBucketName,
				DstObjectName:        "dst" + testObjectName,
				DstPrimarySpApproval: nil,
			},
			err: ErrInvalidApproval,
		},
		{
			name: "invalid src bucket name",
			msg: MsgCopyObject{
				Operator:      sample.RandAccAddressHex(),
				SrcBucketName: "1.1.1.1",
				SrcObjectName: testObjectName,
				DstBucketName: "dst" + testBucketName,
				DstObjectName: "dst" + testObjectName,
				DstPrimarySpApproval: &common.Approval{
					ExpiredHeight: 100,
					Sig:           []byte("xxx"),
				},
			},
			err: gnfderrors.ErrInvalidBucketName,
		},
		{
			name: "invalid src object name",
			msg: MsgCopyObject{
				Operator:      sample.RandAccAddressHex(),
				SrcBucketName: testBucketName,
				SrcObjectName: "",
				DstBucketName: "dst" + testBucketName,
				DstObjectName: "dst" + testObjectName,
				DstPrimarySpApproval: &common.Approval{
					ExpiredHeight: 100,
					Sig:           []byte("xxx"),
				},
			},
			err: gnfderrors.ErrInvalidObjectName,
		},
		{
			name: "invalid dest bucket name",
			msg: MsgCopyObject{
				Operator:      sample.RandAccAddressHex(),
				SrcBucketName: testBucketName,
				SrcObjectName: testObjectName,
				DstBucketName: "1.1.1.1",
				DstObjectName: "dst" + testObjectName,
				DstPrimarySpApproval: &common.Approval{
					ExpiredHeight: 100,
					Sig:           []byte("xxx"),
				},
			},
			err: gnfderrors.ErrInvalidBucketName,
		},
		{
			name: "invalid dest object name",
			msg: MsgCopyObject{
				Operator:      sample.RandAccAddressHex(),
				SrcBucketName: testBucketName,
				SrcObjectName: testObjectName,
				DstBucketName: "dst" + testBucketName,
				DstObjectName: "",
				DstPrimarySpApproval: &common.Approval{
					ExpiredHeight: 100,
					Sig:           []byte("xxx"),
				},
			},
			err: gnfderrors.ErrInvalidObjectName,
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

func TestMsgSealObject_ValidateBasic(t *testing.T) {
	checksums := [][]byte{sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum()}
	blsSignDoc := NewSecondarySpSealObjectSignDoc("moca_5151-1", 1, math.NewUint(1), GenerateHash(checksums)).GetSignBytes()
	blsPrivKey, _ := bls.GenerateBlsKey()
	aggSig, _ := blsPrivKey.Sign(blsSignDoc, votepool.DST)
	aggSigBts, _ := aggSig.Marshal()
	tests := []struct {
		name string
		msg  MsgSealObject
		err  error
	}{
		{
			name: "normal",
			msg: MsgSealObject{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
			},
		},
		{
			name: "invalid address",
			msg: MsgSealObject{
				Operator:                    "invalid address",
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid bucket name",
			msg: MsgSealObject{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  "1.1.1.1",
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
			},
			err: gnfderrors.ErrInvalidBucketName,
		},
		{
			name: "invalid object name",
			msg: MsgSealObject{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  "",
				SecondarySpBlsAggSignatures: aggSigBts,
			},
			err: gnfderrors.ErrInvalidObjectName,
		},
		{
			name: "invalid signature",
			msg: MsgSealObject{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: []byte("invalid signature"),
			},
			err: gnfderrors.ErrInvalidBlsSignature,
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

func TestMsgRejectSealObject_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgRejectSealObject
		err  error
	}{
		{
			name: "normal",
			msg: MsgRejectSealObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
		},
		{
			name: "invalid address",
			msg: MsgRejectSealObject{
				Operator:   "invalid address",
				BucketName: "1.1.1.1",
				ObjectName: testObjectName,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid bucket name",
			msg: MsgRejectSealObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "1.1.1.1",
				ObjectName: testObjectName,
			},
			err: gnfderrors.ErrInvalidBucketName,
		},
		{
			name: "invalid object name",
			msg: MsgRejectSealObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
			},
			err: gnfderrors.ErrInvalidObjectName,
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

func TestMsgUpdateObjectInfo_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgUpdateObjectInfo
		err  error
	}{
		{
			name: "normal",
			msg: MsgUpdateObjectInfo{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_INHERIT,
			},
		},
		{
			name: "invalid address",
			msg: MsgUpdateObjectInfo{
				Operator:   "invalid address",
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_INHERIT,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid bucket name",
			msg: MsgUpdateObjectInfo{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "1.1.1.1",
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_INHERIT,
			},
			err: gnfderrors.ErrInvalidBucketName,
		},
		{
			name: "invalid bucket name",
			msg: MsgUpdateObjectInfo{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
				Visibility: VISIBILITY_TYPE_INHERIT,
			},
			err: gnfderrors.ErrInvalidObjectName,
		},
		{
			name: "invalid visibility",
			msg: MsgUpdateObjectInfo{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_UNSPECIFIED,
			},
			err: ErrInvalidVisibility,
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

func TestMsgDiscontinueObject_ValidateBasic(t *testing.T) {
	invalidObjectIDs := [MaxDiscontinueObjects + 1]Uint{}
	tests := []struct {
		name string
		msg  MsgDiscontinueObject
		err  error
	}{
		{
			name: "normal",
			msg: MsgDiscontinueObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectIds:  []Uint{math.NewUint(1)},
				Reason:     "valid reason",
			},
		},
		{
			name: "invalid address",
			msg: MsgDiscontinueObject{
				Operator:   "invalid address",
				BucketName: testBucketName,
				ObjectIds:  []Uint{math.NewUint(1)},
				Reason:     "valid reason",
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid bucket name",
			msg: MsgDiscontinueObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "1.11.1.1",
				ObjectIds:  []Uint{math.NewUint(1)},
				Reason:     "valid reason",
			},
			err: gnfderrors.ErrInvalidBucketName,
		},
		{
			name: "invalid object ids",
			msg: MsgDiscontinueObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectIds:  nil,
				Reason:     "valid reason",
			},
			err: ErrInvalidObjectIDs,
		},
		{
			name: "invalid object ids",
			msg: MsgDiscontinueObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectIds:  invalidObjectIDs[:],
				Reason:     "valid reason",
			},
			err: ErrInvalidObjectIDs,
		},
		{
			name: "invalid reason",
			msg: MsgDiscontinueObject{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectIds:  []Uint{math.NewUint(1)},
				Reason:     strings.Repeat("s", MaxDiscontinueReasonLen+1),
			},
			err: ErrInvalidReason,
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

// sampleBlsSignature returns a well-formed secondary-SP BLS aggregate signature over
// checksums, of the exact length ValidateBasic requires.
func sampleBlsSignature(t *testing.T, checksums [][]byte) []byte {
	t.Helper()
	blsSignDoc := NewSecondarySpSealObjectSignDoc("moca_5151-1", 1, math.NewUint(1), GenerateHash(checksums)).GetSignBytes()
	blsPrivKey, err := bls.GenerateBlsKey()
	require.NoError(t, err)
	aggSig, err := blsPrivKey.Sign(blsSignDoc, votepool.DST)
	require.NoError(t, err)
	aggSigBts, err := aggSig.Marshal()
	require.NoError(t, err)
	return aggSigBts
}

func TestMsgSealObjectV2_ValidateBasic(t *testing.T) {
	checksums := [][]byte{sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum(), sample.Checksum()}
	aggSigBts := sampleBlsSignature(t, checksums)
	tests := []struct {
		name string
		msg  MsgSealObjectV2
		err  error
	}{
		{
			name: "normal without checksums",
			msg: MsgSealObjectV2{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
			},
		}, {
			name: "normal with checksums",
			msg: MsgSealObjectV2{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
				ExpectChecksums:             checksums,
			},
		}, {
			name: "invalid address",
			msg: MsgSealObjectV2{
				Operator:                    "invalid address",
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgSealObjectV2{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  "1.1.1.1",
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgSealObjectV2{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  "",
				SecondarySpBlsAggSignatures: aggSigBts,
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "invalid signature",
			msg: MsgSealObjectV2{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: []byte("invalid signature"),
			},
			err: gnfderrors.ErrInvalidBlsSignature,
		}, {
			name: "invalid checksums",
			msg: MsgSealObjectV2{
				Operator:                    sample.RandAccAddressHex(),
				BucketName:                  testBucketName,
				ObjectName:                  testObjectName,
				SecondarySpBlsAggSignatures: aggSigBts,
				ExpectChecksums:             [][]byte{[]byte("too-short")},
			},
			err: gnfderrors.ErrInvalidChecksum,
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

func TestMsgUpdateObjectContent_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgUpdateObjectContent
		err  error
	}{
		{
			name: "normal",
			msg: MsgUpdateObjectContent{
				Operator:        sample.RandAccAddressHex(),
				BucketName:      testBucketName,
				ObjectName:      testObjectName,
				PayloadSize:     1024,
				ExpectChecksums: [][]byte{sample.Checksum(), sample.Checksum()},
			},
		}, {
			name: "invalid operator address",
			msg: MsgUpdateObjectContent{
				Operator:   "invalid address",
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "TestBucket",
				ObjectName: testObjectName,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "invalid checksums",
			msg: MsgUpdateObjectContent{
				Operator:        sample.RandAccAddressHex(),
				BucketName:      testBucketName,
				ObjectName:      testObjectName,
				ExpectChecksums: [][]byte{[]byte("too-short")},
			},
			err: gnfderrors.ErrInvalidChecksum,
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

func TestMsgCancelUpdateObjectContent_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCancelUpdateObjectContent
		err  error
	}{
		{
			name: "normal",
			msg: MsgCancelUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
		}, {
			name: "invalid operator address",
			msg: MsgCancelUpdateObjectContent{
				Operator:   "invalid address",
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgCancelUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "TestBucket",
				ObjectName: testObjectName,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgCancelUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
			},
			err: gnfderrors.ErrInvalidObjectName,
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

func TestMsgDelegateCreateObject_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDelegateCreateObject
		err  error
	}{
		{
			name: "normal",
			msg: MsgDelegateCreateObject{
				Operator:   sample.RandAccAddressHex(),
				Creator:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_PRIVATE,
			},
		}, {
			name: "invalid operator address",
			msg: MsgDelegateCreateObject{
				Operator:   "invalid address",
				Creator:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_PRIVATE,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid creator address",
			msg: MsgDelegateCreateObject{
				Operator:   sample.RandAccAddressHex(),
				Creator:    "invalid address",
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_PRIVATE,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgDelegateCreateObject{
				Operator:   sample.RandAccAddressHex(),
				Creator:    sample.RandAccAddressHex(),
				BucketName: "TestBucket",
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_PRIVATE,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgDelegateCreateObject{
				Operator:   sample.RandAccAddressHex(),
				Creator:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
				Visibility: VISIBILITY_TYPE_PRIVATE,
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "invalid checksums",
			msg: MsgDelegateCreateObject{
				Operator:        sample.RandAccAddressHex(),
				Creator:         sample.RandAccAddressHex(),
				BucketName:      testBucketName,
				ObjectName:      testObjectName,
				Visibility:      VISIBILITY_TYPE_PRIVATE,
				ExpectChecksums: [][]byte{[]byte("too-short")},
			},
			err: gnfderrors.ErrInvalidChecksum,
		}, {
			name: "unspecified visibility",
			msg: MsgDelegateCreateObject{
				Operator:   sample.RandAccAddressHex(),
				Creator:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
				Visibility: VISIBILITY_TYPE_UNSPECIFIED,
			},
			err: ErrInvalidVisibility,
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

func TestMsgDelegateUpdateObjectContent_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDelegateUpdateObjectContent
		err  error
	}{
		{
			name: "normal",
			msg: MsgDelegateUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				Updater:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
		}, {
			name: "invalid operator address",
			msg: MsgDelegateUpdateObjectContent{
				Operator:   "invalid address",
				Updater:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid updater address",
			msg: MsgDelegateUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				Updater:    "invalid address",
				BucketName: testBucketName,
				ObjectName: testObjectName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgDelegateUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				Updater:    sample.RandAccAddressHex(),
				BucketName: "TestBucket",
				ObjectName: testObjectName,
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgDelegateUpdateObjectContent{
				Operator:   sample.RandAccAddressHex(),
				Updater:    sample.RandAccAddressHex(),
				BucketName: testBucketName,
				ObjectName: "",
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "invalid checksums",
			msg: MsgDelegateUpdateObjectContent{
				Operator:        sample.RandAccAddressHex(),
				Updater:         sample.RandAccAddressHex(),
				BucketName:      testBucketName,
				ObjectName:      testObjectName,
				ExpectChecksums: [][]byte{[]byte("too-short")},
			},
			err: gnfderrors.ErrInvalidChecksum,
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

// --- Boilerplate coverage: NewMsgXxx / Route / Type / GetSigners / GetSignBytes ---

func TestNewMsgCreateObject(t *testing.T) {
	creator := sample.RandAccAddress()
	checksums := [][]byte{sample.Checksum()}
	sig := []byte("sig-one")
	msg := NewMsgCreateObject(creator, testBucketName, testObjectName, 1024, VISIBILITY_TYPE_PRIVATE, checksums, "content-type", REDUNDANCY_REPLICA_TYPE, 100, sig)

	require.Equal(t, creator.String(), msg.Creator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, uint64(1024), msg.PayloadSize)
	require.Equal(t, VISIBILITY_TYPE_PRIVATE, msg.Visibility)
	require.Equal(t, "content-type", msg.ContentType)
	require.Equal(t, REDUNDANCY_REPLICA_TYPE, msg.RedundancyType)
	require.Equal(t, checksums, msg.ExpectChecksums)
	require.Equal(t, uint64(100), msg.PrimarySpApproval.ExpiredHeight)
	require.Equal(t, sig, msg.PrimarySpApproval.Sig)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCreateObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{creator}, msg.GetSigners())
	requireSignBytes(t, msg)

	otherSig := NewMsgCreateObject(creator, testBucketName, testObjectName, 1024, VISIBILITY_TYPE_PRIVATE, checksums, "content-type", REDUNDANCY_REPLICA_TYPE, 100, []byte("sig-two"))
	requireApprovalBytesStripSignature(t, msg, otherSig)

	bad := *msg
	bad.Creator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgCancelCreateObject(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgCancelCreateObject(operator, testBucketName, testObjectName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCancelCreateObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDeleteObject(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgDeleteObject(operator, testBucketName, testObjectName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDeleteObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgCopyObject(t *testing.T) {
	operator := sample.RandAccAddress()
	sig := []byte("sig-one")
	msg := NewMsgCopyObject(operator, testBucketName, "dst"+testBucketName, testObjectName, "dst"+testObjectName, 100, sig)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.SrcBucketName)
	require.Equal(t, "dst"+testBucketName, msg.DstBucketName)
	require.Equal(t, testObjectName, msg.SrcObjectName)
	require.Equal(t, "dst"+testObjectName, msg.DstObjectName)
	require.Equal(t, uint64(100), msg.DstPrimarySpApproval.ExpiredHeight)
	require.Equal(t, sig, msg.DstPrimarySpApproval.Sig)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCopyObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	otherSig := NewMsgCopyObject(operator, testBucketName, "dst"+testBucketName, testObjectName, "dst"+testObjectName, 100, []byte("sig-two"))
	requireApprovalBytesStripSignature(t, msg, otherSig)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgSealObject(t *testing.T) {
	operator := sample.RandAccAddress()
	checksums := [][]byte{sample.Checksum()}
	aggSigBts := sampleBlsSignature(t, checksums)
	msg := NewMsgSealObject(operator, testBucketName, testObjectName, 7, aggSigBts)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, uint32(7), msg.GlobalVirtualGroupId)
	require.Equal(t, aggSigBts, msg.SecondarySpBlsAggSignatures)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgSealObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgSealObjectV2(t *testing.T) {
	operator := sample.RandAccAddress()
	checksums := [][]byte{sample.Checksum()}
	aggSigBts := sampleBlsSignature(t, checksums)
	msg := NewMsgSealObjectV2(operator, testBucketName, testObjectName, 7, aggSigBts, checksums)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, uint32(7), msg.GlobalVirtualGroupId)
	require.Equal(t, aggSigBts, msg.SecondarySpBlsAggSignatures)
	require.Equal(t, checksums, msg.ExpectChecksums)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgSealObjectV2, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgRejectUnsealedObject(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgRejectUnsealedObject(operator, testBucketName, testObjectName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgRejectSealObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgUpdateObjectInfo(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgUpdateObjectInfo(operator, testBucketName, testObjectName, VISIBILITY_TYPE_PRIVATE)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, VISIBILITY_TYPE_PRIVATE, msg.Visibility)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgUpdateObjectInfo, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDiscontinueObject(t *testing.T) {
	operator := sample.RandAccAddress()
	objectIDs := []Uint{math.NewUint(1), math.NewUint(2)}
	msg := NewMsgDiscontinueObject(operator, testBucketName, objectIDs, "  padded reason  ")

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, objectIDs, msg.ObjectIds)
	require.Equal(t, "padded reason", msg.Reason)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDiscontinueObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgUpdateObjectContent(t *testing.T) {
	operator := sample.RandAccAddress()
	checksums := [][]byte{sample.Checksum()}
	msg := NewMsgUpdateObjectContent(operator, testBucketName, testObjectName, 2048, checksums)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, uint64(2048), msg.PayloadSize)
	require.Equal(t, checksums, msg.ExpectChecksums)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgUpdateObjectContent, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgCancelUpdateObjectContent(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgCancelUpdateObjectContent(operator, testBucketName, testObjectName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCancelUpdateObjectContent, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDelegateCreateObject(t *testing.T) {
	operator := sample.RandAccAddress()
	creator := sample.RandAccAddress()
	checksums := [][]byte{sample.Checksum()}
	msg := NewMsgDelegateCreateObject(operator, creator, testBucketName, testObjectName, 1024, VISIBILITY_TYPE_PRIVATE, checksums, "content-type", REDUNDANCY_REPLICA_TYPE)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, creator.String(), msg.Creator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, uint64(1024), msg.PayloadSize)
	require.Equal(t, VISIBILITY_TYPE_PRIVATE, msg.Visibility)
	require.Equal(t, checksums, msg.ExpectChecksums)
	require.Equal(t, "content-type", msg.ContentType)
	require.Equal(t, REDUNDANCY_REPLICA_TYPE, msg.RedundancyType)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDelegateCreateObject, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDelegateUpdateObjectContent(t *testing.T) {
	operator := sample.RandAccAddress()
	updater := sample.RandAccAddress()
	checksums := [][]byte{sample.Checksum()}
	msg := NewMsgDelegateUpdateObjectContent(operator, updater, testBucketName, testObjectName, 2048, checksums)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, updater.String(), msg.Updater)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.Equal(t, uint64(2048), msg.PayloadSize)
	require.Equal(t, checksums, msg.ExpectChecksums)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDelegateUpdateObjectContent, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}
