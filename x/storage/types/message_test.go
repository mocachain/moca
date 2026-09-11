package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/types/common"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
	"github.com/mocachain/moca/v2/x/permission/types"
)

var (
	testBucketName                      = "testbucket"
	testObjectName                      = "testobject"
	testGroupName                       = "testgroup"
	testInvalidBucketNameWithLongLength = [68]byte{}
	// farFutureTime is after types.MaxTimeStamp, used to trigger the group-member
	// "expiration time too big" validation branch.
	farFutureTime = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	testBucketGRN = types2.NewBucketGRN(testBucketName).String()
)

// requireSignBytes asserts that msg.GetSignBytes() returns non-empty, deterministically
// sorted JSON that round-trips through the module codec back to an identical message. This
// proves the sign bytes actually encode the message content rather than being a stub value.
func requireSignBytes(t *testing.T, msg interface {
	proto.Message
	GetSignBytes() []byte
},
) {
	t.Helper()
	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)
	require.True(t, json.Valid(bz))
	require.Equal(t, bz, sdk.MustSortJSON(bz), "GetSignBytes output must already be sorted JSON")

	// Round-trip through a freshly allocated instance (never proto.Clone/Equal on msg itself):
	// gogoproto's generic reflection-based Merge/Equal panics ("merger not found for type:
	// big.Word") on messages carrying a populated cosmossdk.io/math Int/Uint field, since it
	// tries to walk math.Int's unexported big.Int internals instead of using the type's own
	// Marshal/Unmarshal. Re-marshaling a freshly unmarshaled instance and comparing bytes
	// proves the same round-trip fidelity without ever invoking that codepath.
	fresh, ok := reflect.New(reflect.TypeOf(msg).Elem()).Interface().(proto.Message)
	require.True(t, ok)
	require.NoError(t, ModuleCdc.UnmarshalJSON(bz, fresh))
	freshBz := sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(fresh))
	require.Equal(t, bz, freshBz, "sign bytes must round-trip back to identical content")
}

// requireApprovalBytesStripSignature asserts that two messages which differ only in their
// approval signature produce identical GetApprovalBytes() output (proving the signature is
// zeroed before signing) while still producing different GetSignBytes() output (proving the
// signature is not simply dropped from the message).
func requireApprovalBytesStripSignature(t *testing.T, withSig, withOtherSig interface {
	GetApprovalBytes() []byte
	GetSignBytes() []byte
},
) {
	t.Helper()
	require.Equal(t, withSig.GetApprovalBytes(), withOtherSig.GetApprovalBytes())
	require.NotEqual(t, withSig.GetSignBytes(), withOtherSig.GetSignBytes())
}

func TestMsgCreateBucket_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCreateBucket
		err  error
	}{
		{
			name: "normal",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  sample.RandAccAddressHex(),
				PrimarySpApproval: &common.Approval{},
			},
		}, {
			name: "invalid bucket name",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        "TestBucket",
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  sample.RandAccAddressHex(),
				PrimarySpApproval: &common.Approval{},
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid bucket name",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        "Test-Bucket",
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  sample.RandAccAddressHex(),
				PrimarySpApproval: &common.Approval{},
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid bucket name",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        "ss",
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  sample.RandAccAddressHex(),
				PrimarySpApproval: &common.Approval{},
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid bucket name",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        string(testInvalidBucketNameWithLongLength[:]),
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  sample.RandAccAddressHex(),
				PrimarySpApproval: &common.Approval{},
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid creator address",
			msg: MsgCreateBucket{
				Creator:           "invalid address",
				BucketName:        testBucketName,
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  sample.RandAccAddressHex(),
				PrimarySpApproval: &common.Approval{},
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid primary sp address",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  "invalid address",
				PrimarySpApproval: &common.Approval{},
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "nil approval",
			msg: MsgCreateBucket{
				Creator:          sample.RandAccAddressHex(),
				BucketName:       testBucketName,
				Visibility:       VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:   sample.RandAccAddressHex(),
				PrimarySpAddress: sample.RandAccAddressHex(),
			},
			err: ErrInvalidApproval,
		}, {
			name: "invalid payment address",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				Visibility:        VISIBILITY_TYPE_PUBLIC_READ,
				PaymentAddress:    "invalid address",
				PrimarySpAddress:  sample.RandAccAddressHex(),
				PrimarySpApproval: &common.Approval{},
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "unspecified visibility",
			msg: MsgCreateBucket{
				Creator:           sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				Visibility:        VISIBILITY_TYPE_UNSPECIFIED,
				PaymentAddress:    sample.RandAccAddressHex(),
				PrimarySpAddress:  sample.RandAccAddressHex(),
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

func TestMsgDeleteBucket_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDeleteBucket
		err  error
	}{
		{
			name: "normal",
			msg: MsgDeleteBucket{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
			},
		}, {
			name: "invalid bucket name",
			msg: MsgDeleteBucket{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "testBucket",
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid operator address",
			msg: MsgDeleteBucket{
				Operator:   "invalid address",
				BucketName: testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
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

func TestMsgUpdateBucketInfo_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgUpdateBucketInfo
		err  error
	}{
		{
			name: "basic",
			msg: MsgUpdateBucketInfo{
				Operator:         sample.RandAccAddressHex(),
				BucketName:       testBucketName,
				PaymentAddress:   sample.RandAccAddressHex(),
				ChargedReadQuota: &common.UInt64Value{Value: 10000},
			},
		}, {
			name: "invalid operator address",
			msg: MsgUpdateBucketInfo{
				Operator:   "invalid address",
				BucketName: testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgUpdateBucketInfo{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "TestBucket",
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid visibility",
			msg: MsgUpdateBucketInfo{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				Visibility: VisibilityType(100),
			},
			err: ErrInvalidVisibility,
		}, {
			name: "invalid payment address",
			msg: MsgUpdateBucketInfo{
				Operator:       sample.RandAccAddressHex(),
				BucketName:     testBucketName,
				PaymentAddress: "invalid address",
			},
			err: sdkerrors.ErrInvalidAddress,
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

func TestMsgCreateGroup_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCreateGroup
		err  error
	}{
		{
			name: "normal",
			msg: MsgCreateGroup{
				Creator:   sample.RandAccAddressHex(),
				GroupName: testGroupName,
			},
		}, {
			name: "invalid creator address",
			msg: MsgCreateGroup{
				Creator:   "invalid address",
				GroupName: testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group name",
			msg: MsgCreateGroup{
				Creator:   sample.RandAccAddressHex(),
				GroupName: "a",
			},
			err: gnfderrors.ErrInvalidGroupName,
		}, {
			name: "extra field is too long",
			msg: MsgCreateGroup{
				Creator:   sample.RandAccAddressHex(),
				GroupName: testGroupName,
				Extra:     strings.Repeat("abcdefg", 80),
			},
			err: gnfderrors.ErrInvalidParameter,
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

func TestMsgDeleteGroup_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDeleteGroup
		err  error
	}{
		{
			name: "normal",
			msg: MsgDeleteGroup{
				Operator:  sample.RandAccAddressHex(),
				GroupName: testGroupName,
			},
		}, {
			name: "invalid operator address",
			msg: MsgDeleteGroup{
				Operator:  "invalid address",
				GroupName: testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group name",
			msg: MsgDeleteGroup{
				Operator:  sample.RandAccAddressHex(),
				GroupName: "a",
			},
			err: gnfderrors.ErrInvalidGroupName,
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

func TestMsgLeaveGroup_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgLeaveGroup
		err  error
	}{
		{
			name: "normal",
			msg: MsgLeaveGroup{
				Member:     sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
			},
		}, {
			name: "invalid member address",
			msg: MsgLeaveGroup{
				Member:     "invalid address",
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group owner address",
			msg: MsgLeaveGroup{
				Member:     sample.RandAccAddressHex(),
				GroupOwner: "invalid address",
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group name",
			msg: MsgLeaveGroup{
				Member:     sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  "a",
			},
			err: gnfderrors.ErrInvalidGroupName,
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

func TestMsgUpdateGroupMember_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgUpdateGroupMember
		err  error
	}{
		{
			name: "normal",
			msg: MsgUpdateGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				MembersToAdd: []*MsgGroupMember{
					{
						Member: sample.RandAccAddressHex(),
					},
					{
						Member: sample.RandAccAddressHex(),
					},
				},
				MembersToDelete: []string{sample.RandAccAddressHex(), sample.RandAccAddressHex()},
			},
		}, {
			name: "invalid operator address",
			msg: MsgUpdateGroupMember{
				Operator:   "invalid address",
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group owner address",
			msg: MsgUpdateGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: "invalid address",
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group name",
			msg: MsgUpdateGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  "a",
			},
			err: gnfderrors.ErrInvalidGroupName,
		}, {
			name: "too many members in a single update",
			msg: MsgUpdateGroupMember{
				Operator:        sample.RandAccAddressHex(),
				GroupOwner:      sample.RandAccAddressHex(),
				GroupName:       testGroupName,
				MembersToDelete: make([]string, MaxGroupMemberLimitOnce+1),
			},
			err: gnfderrors.ErrInvalidParameter,
		}, {
			name: "invalid member to add address",
			msg: MsgUpdateGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				MembersToAdd: []*MsgGroupMember{
					{Member: "invalid address"},
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "member to add expiration time too far in the future",
			msg: MsgUpdateGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				MembersToAdd: []*MsgGroupMember{
					{Member: sample.RandAccAddressHex(), ExpirationTime: &farFutureTime},
				},
			},
			err: gnfderrors.ErrInvalidParameter,
		}, {
			name: "invalid member to delete address",
			msg: MsgUpdateGroupMember{
				Operator:        sample.RandAccAddressHex(),
				GroupOwner:      sample.RandAccAddressHex(),
				GroupName:       testGroupName,
				MembersToDelete: []string{"invalid address"},
			},
			err: sdkerrors.ErrInvalidAddress,
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

func TestMsgUpdateGroupExtra_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgUpdateGroupExtra
		err  error
	}{
		{
			name: "normal",
			msg: MsgUpdateGroupExtra{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				Extra:      "testExtra",
			},
		},
		{
			name: "extra field is too long",
			msg: MsgUpdateGroupExtra{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				Extra:      strings.Repeat("abcdefg", 80),
			},
			err: gnfderrors.ErrInvalidParameter,
		},
		{
			name: "invalid operator address",
			msg: MsgUpdateGroupExtra{
				Operator:   "invalid address",
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid group owner address",
			msg: MsgUpdateGroupExtra{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: "invalid address",
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid group name",
			msg: MsgUpdateGroupExtra{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  "a",
			},
			err: gnfderrors.ErrInvalidGroupName,
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

func TestMsgPutPolicy_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgPutPolicy
		err  error
	}{
		{
			name: "normal",
			msg: MsgPutPolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  types2.NewBucketGRN(testBucketName).String(),
				Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
				Statements: []*types.Statement{{
					Effect:  types.EFFECT_ALLOW,
					Actions: []types.ActionType{types.ACTION_DELETE_BUCKET},
				}},
			},
		},
		{
			name: "bucket object action without resources",
			msg: MsgPutPolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  types2.NewBucketGRN(testBucketName).String(),
				Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
				Statements: []*types.Statement{{
					Effect:  types.EFFECT_DENY,
					Actions: []types.ActionType{types.ACTION_GET_OBJECT},
				}},
			},
			err: types.ErrInvalidStatement,
		},
		{
			name: "invalid operator address",
			msg: MsgPutPolicy{
				Operator:  "invalid address",
				Resource:  types2.NewBucketGRN(testBucketName).String(),
				Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid resource",
			msg: MsgPutPolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  "not-a-grn",
				Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
			},
			err: gnfderrors.ErrInvalidGRN,
		},
		{
			name: "nil principal",
			msg: MsgPutPolicy{
				Operator: sample.RandAccAddressHex(),
				Resource: types2.NewBucketGRN(testBucketName).String(),
			},
			err: gnfderrors.ErrInvalidPrincipal,
		},
		{
			name: "group principal cannot be granted permission on another group",
			msg: MsgPutPolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  types2.NewGroupGRN(sdk.MustAccAddressFromHex(sample.RandAccAddressHex()), testGroupName).String(),
				Principal: types.NewPrincipalWithGroupID(math.NewUint(1)),
			},
			err: gnfderrors.ErrInvalidPrincipal,
		},
		{
			name: "principal fails its own ValidateBasic",
			msg: MsgPutPolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  types2.NewBucketGRN(testBucketName).String(),
				Principal: &types.Principal{Type: types.PRINCIPAL_TYPE_UNSPECIFIED},
			},
			err: types.ErrInvalidPrincipal,
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

func TestMsgDeletePolicy_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDeletePolicy
		err  error
	}{
		{
			name: "valid address",
			msg: MsgDeletePolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  types2.NewBucketGRN(testBucketName).String(),
				Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
			},
		}, {
			name: "invalid operator address",
			msg: MsgDeletePolicy{
				Operator:  "invalid address",
				Resource:  types2.NewBucketGRN(testBucketName).String(),
				Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid resource",
			msg: MsgDeletePolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  "not-a-grn",
				Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
			},
			err: gnfderrors.ErrInvalidGRN,
		}, {
			name: "nil principal",
			msg: MsgDeletePolicy{
				Operator: sample.RandAccAddressHex(),
				Resource: types2.NewBucketGRN(testBucketName).String(),
			},
			err: gnfderrors.ErrInvalidPrincipal,
		}, {
			name: "group principal cannot be granted permission on another group",
			msg: MsgDeletePolicy{
				Operator:  sample.RandAccAddressHex(),
				Resource:  types2.NewGroupGRN(sdk.MustAccAddressFromHex(sample.RandAccAddressHex()), testGroupName).String(),
				Principal: types.NewPrincipalWithGroupID(math.NewUint(1)),
			},
			err: gnfderrors.ErrInvalidPrincipal,
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

func TestMsgDeletePolicy_ValidateRuntime(t *testing.T) {
	t.Run("valid principal", func(t *testing.T) {
		msg := MsgDeletePolicy{
			Operator:  sample.RandAccAddressHex(),
			Resource:  types2.NewBucketGRN(testBucketName).String(),
			Principal: types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex())),
		}
		require.NoError(t, msg.ValidateRuntime(sdk.Context{}))
	})
	t.Run("principal fails its own ValidateBasic", func(t *testing.T) {
		msg := MsgDeletePolicy{
			Operator:  sample.RandAccAddressHex(),
			Resource:  types2.NewBucketGRN(testBucketName).String(),
			Principal: &types.Principal{Type: types.PRINCIPAL_TYPE_UNSPECIFIED},
		}
		require.ErrorIs(t, msg.ValidateRuntime(sdk.Context{}), types.ErrInvalidPrincipal)
	})
}

func TestMsgRenewGroupMember_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgRenewGroupMember
		err  error
	}{
		{
			name: "normal",
			msg: MsgRenewGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				Members: []*MsgGroupMember{
					{
						Member: sample.RandAccAddressHex(),
					},
					{
						Member: sample.RandAccAddressHex(),
					},
				},
			},
		}, {
			name: "invalid operator address",
			msg: MsgRenewGroupMember{
				Operator:   "invalid address",
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group owner address",
			msg: MsgRenewGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: "invalid address",
				GroupName:  testGroupName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid group name",
			msg: MsgRenewGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  "a",
			},
			err: gnfderrors.ErrInvalidGroupName,
		}, {
			name: "too many members in a single renewal",
			msg: MsgRenewGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				Members:    make([]*MsgGroupMember, MaxGroupMemberLimitOnce+1),
			},
			err: gnfderrors.ErrInvalidParameter,
		}, {
			name: "invalid member address",
			msg: MsgRenewGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				Members: []*MsgGroupMember{
					{Member: "invalid address"},
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "member expiration time too far in the future",
			msg: MsgRenewGroupMember{
				Operator:   sample.RandAccAddressHex(),
				GroupOwner: sample.RandAccAddressHex(),
				GroupName:  testGroupName,
				Members: []*MsgGroupMember{
					{Member: sample.RandAccAddressHex(), ExpirationTime: &farFutureTime},
				},
			},
			err: gnfderrors.ErrInvalidParameter,
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

func TestMsgToggleSPAsDelegatedAgent_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgToggleSPAsDelegatedAgent
		err  error
	}{
		{
			name: "normal",
			msg: MsgToggleSPAsDelegatedAgent{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
			},
		}, {
			name: "invalid operator address",
			msg: MsgToggleSPAsDelegatedAgent{
				Operator:   "invalid address",
				BucketName: testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgToggleSPAsDelegatedAgent{
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

func TestMsgDiscontinueBucket_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDiscontinueBucket
		err  error
	}{
		{
			name: "normal",
			msg: MsgDiscontinueBucket{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
				Reason:     "valid reason",
			},
		}, {
			name: "invalid operator address",
			msg: MsgDiscontinueBucket{
				Operator:   "invalid address",
				BucketName: testBucketName,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgDiscontinueBucket{
				Operator:   sample.RandAccAddressHex(),
				BucketName: "TestBucket",
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "reason too long",
			msg: MsgDiscontinueBucket{
				Operator:   sample.RandAccAddressHex(),
				BucketName: testBucketName,
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

func TestMsgSetTag_ValidateBasic(t *testing.T) {
	tooManyTags := &ResourceTags{}
	for i := 0; i < MaxTagCount+1; i++ {
		tooManyTags.Tags = append(tooManyTags.Tags, ResourceTags_Tag{Key: fmt.Sprintf("key%d", i), Value: "value"})
	}

	tests := []struct {
		name string
		msg  MsgSetTag
		err  error
	}{
		{
			name: "normal",
			msg: MsgSetTag{
				Operator: sample.RandAccAddressHex(),
				Resource: testBucketGRN,
				Tags: &ResourceTags{Tags: []ResourceTags_Tag{
					{Key: "env", Value: "prod"},
					{Key: "team", Value: "storage"},
				}},
			},
		}, {
			name: "no tags is valid",
			msg: MsgSetTag{
				Operator: sample.RandAccAddressHex(),
				Resource: testBucketGRN,
			},
		}, {
			name: "invalid operator address",
			msg: MsgSetTag{
				Operator: "invalid address",
				Resource: testBucketGRN,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid resource",
			msg: MsgSetTag{
				Operator: sample.RandAccAddressHex(),
				Resource: "not-a-grn",
			},
			err: gnfderrors.ErrInvalidGRN,
		}, {
			name: "too many tags",
			msg: MsgSetTag{
				Operator: sample.RandAccAddressHex(),
				Resource: testBucketGRN,
				Tags:     tooManyTags,
			},
			err: gnfderrors.ErrInvalidParameter,
		}, {
			name: "tag key too long",
			msg: MsgSetTag{
				Operator: sample.RandAccAddressHex(),
				Resource: testBucketGRN,
				Tags: &ResourceTags{Tags: []ResourceTags_Tag{
					{Key: strings.Repeat("k", MaxTagKeyLength+1), Value: "v"},
				}},
			},
			err: gnfderrors.ErrInvalidParameter,
		}, {
			name: "tag value too long",
			msg: MsgSetTag{
				Operator: sample.RandAccAddressHex(),
				Resource: testBucketGRN,
				Tags: &ResourceTags{Tags: []ResourceTags_Tag{
					{Key: "k", Value: strings.Repeat("v", MaxTagValueLength+1)},
				}},
			},
			err: gnfderrors.ErrInvalidParameter,
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

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	// Unlike every other message in this file, MsgUpdateParams wraps the raw
	// AccAddressFromHexUnsafe error directly instead of sdkerrors.ErrInvalidAddress, so the
	// invalid-address case is asserted by message content rather than by sentinel.
	t.Run("normal", func(t *testing.T) {
		msg := MsgUpdateParams{
			Authority: sample.RandAccAddressHex(),
			Params:    DefaultParams(),
		}
		require.NoError(t, msg.ValidateBasic())
	})
	t.Run("invalid authority address", func(t *testing.T) {
		msg := MsgUpdateParams{
			Authority: "invalid address",
			Params:    DefaultParams(),
		}
		require.ErrorContains(t, msg.ValidateBasic(), "invalid authority address")
	})
}

func TestMsgUpdateParams_ValidateBasic_DelegatesToParamsValidate(t *testing.T) {
	msg := MsgUpdateParams{
		Authority: sample.RandAccAddressHex(),
		Params:    Params{},
	}
	require.Error(t, msg.ValidateBasic())
}

func TestMsgUpdateParams_GetSignersAndSignBytes(t *testing.T) {
	authority := sample.RandAccAddress()
	msg := MsgUpdateParams{Authority: authority.String(), Params: DefaultParams()}
	require.Equal(t, []sdk.AccAddress{authority}, msg.GetSigners())
	requireSignBytes(t, &msg)
}

func TestMsgPutPolicy_ValidateRuntime(t *testing.T) {
	t.Run("valid statements", func(t *testing.T) {
		msg := MsgPutPolicy{
			Operator: sample.RandAccAddressHex(),
			Resource: testBucketGRN,
			Statements: []*types.Statement{{
				Effect:  types.EFFECT_ALLOW,
				Actions: []types.ActionType{types.ACTION_DELETE_BUCKET},
			}},
		}
		require.NoError(t, msg.ValidateRuntime(sdk.Context{}))
	})
	t.Run("statement fails its own ValidateRuntime", func(t *testing.T) {
		msg := MsgPutPolicy{
			Operator: sample.RandAccAddressHex(),
			Resource: types2.NewObjectGRN(testBucketName, testObjectName).String(),
			Statements: []*types.Statement{{
				Effect:    types.EFFECT_ALLOW,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{"sub/*"},
			}},
		}
		require.ErrorIs(t, msg.ValidateRuntime(sdk.Context{}), types.ErrInvalidStatement)
	})
}

// --- Boilerplate coverage: NewMsgXxx / Route / Type / GetSigners / GetSignBytes ---

func TestNewMsgCreateBucket(t *testing.T) {
	creator := sample.RandAccAddress()
	primarySP := sample.RandAccAddress()
	paymentAddr := sample.RandAccAddress()
	sig := []byte("sig-one")
	msg := NewMsgCreateBucket(creator, testBucketName, VISIBILITY_TYPE_PRIVATE, primarySP, paymentAddr, 100, sig, 1024)

	require.Equal(t, creator.String(), msg.Creator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, VISIBILITY_TYPE_PRIVATE, msg.Visibility)
	require.Equal(t, paymentAddr.String(), msg.PaymentAddress)
	require.Equal(t, primarySP.String(), msg.PrimarySpAddress)
	require.Equal(t, uint64(100), msg.PrimarySpApproval.ExpiredHeight)
	require.Equal(t, sig, msg.PrimarySpApproval.Sig)
	require.Equal(t, uint64(1024), msg.ChargedReadQuota)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCreateBucket, msg.Type())
	require.Equal(t, []sdk.AccAddress{creator}, msg.GetSigners())
	requireSignBytes(t, msg)

	otherSig := NewMsgCreateBucket(creator, testBucketName, VISIBILITY_TYPE_PRIVATE, primarySP, paymentAddr, 100, []byte("sig-two"), 1024)
	requireApprovalBytesStripSignature(t, msg, otherSig)

	bad := *msg
	bad.Creator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDeleteBucket(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgDeleteBucket(operator, testBucketName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDeleteBucket, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgUpdateBucketInfo(t *testing.T) {
	operator := sample.RandAccAddress()
	paymentAcc := sample.RandAccAddress()
	quota := uint64(555)
	msg := NewMsgUpdateBucketInfo(operator, testBucketName, &quota, paymentAcc, VISIBILITY_TYPE_PRIVATE)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, paymentAcc.String(), msg.PaymentAddress)
	require.Equal(t, VISIBILITY_TYPE_PRIVATE, msg.Visibility)
	require.Equal(t, quota, msg.ChargedReadQuota.GetValue())

	msgNoOptionals := NewMsgUpdateBucketInfo(operator, testBucketName, nil, nil, VISIBILITY_TYPE_PRIVATE)
	require.Empty(t, msgNoOptionals.PaymentAddress)
	require.Nil(t, msgNoOptionals.ChargedReadQuota)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgUpdateBucketInfo, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgToggleSPAsDelegatedAgent(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgToggleSPAsDelegatedAgent(operator, testBucketName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgToggleSPAsDelegatedAgent, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDiscontinueBucket(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgDiscontinueBucket(operator, testBucketName, "  padded reason  ")

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, "padded reason", msg.Reason)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDiscontinueBucket, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgCreateGroup(t *testing.T) {
	creator := sample.RandAccAddress()
	msg := NewMsgCreateGroup(creator, testGroupName, "extra-info")

	require.Equal(t, creator.String(), msg.Creator)
	require.Equal(t, testGroupName, msg.GroupName)
	require.Equal(t, "extra-info", msg.Extra)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgCreateGroup, msg.Type())
	require.Equal(t, []sdk.AccAddress{creator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Creator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDeleteGroup(t *testing.T) {
	operator := sample.RandAccAddress()
	msg := NewMsgDeleteGroup(operator, testGroupName)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testGroupName, msg.GroupName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDeleteGroup, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgLeaveGroup(t *testing.T) {
	member := sample.RandAccAddress()
	groupOwner := sample.RandAccAddress()
	msg := NewMsgLeaveGroup(member, groupOwner, testGroupName)

	require.Equal(t, member.String(), msg.Member)
	require.Equal(t, groupOwner.String(), msg.GroupOwner)
	require.Equal(t, testGroupName, msg.GroupName)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgLeaveGroup, msg.Type())
	require.Equal(t, []sdk.AccAddress{member}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Member = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgUpdateGroupMember(t *testing.T) {
	operator := sample.RandAccAddress()
	groupOwner := sample.RandAccAddress()
	memberToAdd := &MsgGroupMember{Member: sample.RandAccAddressHex()}
	memberToDelete := sample.RandAccAddress()
	msg := NewMsgUpdateGroupMember(operator, groupOwner, testGroupName, []*MsgGroupMember{memberToAdd}, []sdk.AccAddress{memberToDelete})

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, groupOwner.String(), msg.GroupOwner)
	require.Equal(t, testGroupName, msg.GroupName)
	require.Equal(t, []*MsgGroupMember{memberToAdd}, msg.MembersToAdd)
	require.Equal(t, []string{memberToDelete.String()}, msg.MembersToDelete)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgUpdateGroupMember, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgUpdateGroupExtra(t *testing.T) {
	operator := sample.RandAccAddress()
	groupOwner := sample.RandAccAddress()
	msg := NewMsgUpdateGroupExtra(operator, groupOwner, testGroupName, "extra-info")

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, groupOwner.String(), msg.GroupOwner)
	require.Equal(t, testGroupName, msg.GroupName)
	require.Equal(t, "extra-info", msg.Extra)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgUpdateGroupExtra, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgRenewGroupMember(t *testing.T) {
	operator := sample.RandAccAddress()
	groupOwner := sample.RandAccAddress()
	member := &MsgGroupMember{Member: sample.RandAccAddressHex()}
	msg := NewMsgRenewGroupMember(operator, groupOwner, testGroupName, []*MsgGroupMember{member})

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, groupOwner.String(), msg.GroupOwner)
	require.Equal(t, testGroupName, msg.GroupName)
	require.Equal(t, []*MsgGroupMember{member}, msg.Members)
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgRenewGroupMember, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgPutPolicy(t *testing.T) {
	operator := sample.RandAccAddress()
	principal := types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex()))
	statements := []*types.Statement{{
		Effect:  types.EFFECT_ALLOW,
		Actions: []types.ActionType{types.ACTION_DELETE_BUCKET},
	}}
	expiry := time.Now().Add(time.Hour).UTC()
	msg := NewMsgPutPolicy(operator, testBucketGRN, principal, statements, &expiry)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketGRN, msg.Resource)
	require.Equal(t, principal, msg.Principal)
	require.Equal(t, statements, msg.Statements)
	require.Equal(t, &expiry, msg.ExpirationTime)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgPutPolicy, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgDeletePolicy(t *testing.T) {
	operator := sample.RandAccAddress()
	principal := types.NewPrincipalWithAccount(sdk.MustAccAddressFromHex(sample.RandAccAddressHex()))
	msg := NewMsgDeletePolicy(operator, testBucketGRN, principal)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketGRN, msg.Resource)
	require.Equal(t, principal, msg.Principal)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgDeletePolicy, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}

func TestNewMsgSetTag(t *testing.T) {
	operator := sample.RandAccAddress()
	tags := &ResourceTags{Tags: []ResourceTags_Tag{{Key: "env", Value: "prod"}}}
	msg := NewMsgSetTag(operator, testBucketGRN, tags)

	require.Equal(t, operator.String(), msg.Operator)
	require.Equal(t, testBucketGRN, msg.Resource)
	require.Equal(t, tags, msg.Tags)

	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgSetTag, msg.Type())
	require.Equal(t, []sdk.AccAddress{operator}, msg.GetSigners())
	requireSignBytes(t, msg)

	bad := *msg
	bad.Operator = "invalid address"
	require.Panics(t, func() { bad.GetSigners() })
}
