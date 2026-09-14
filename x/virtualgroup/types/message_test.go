package types

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/types/common"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

// errInvalidAuthorityAddress is the ValidateBasic error text for an invalid
// authority address, shared with message_storage_provider_forced_exit_test.go.
const errInvalidAuthorityAddress = "invalid authority address"

// Sub-test names shared across TestMessageRouteAndType, TestMessageGetSignBytes,
// and TestMessageGetSigners below, so the identical labels used across those
// three tables aren't flagged as duplicated string literals.
const (
	caseCreateGlobalVirtualGroup = "create global virtual group"
	caseDeleteGlobalVirtualGroup = "delete global virtual group"
	caseSwapOut                  = "swap out"
	caseReserveSwapIn            = "reserve swap in"
	caseCompleteSwapIn           = "complete swap in"
)

func TestMsgDeposit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDeposit
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgDeposit{
				StorageProvider:      "invalid_address",
				GlobalVirtualGroupId: 1,
				Deposit: types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(1),
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid deposit amount",
			msg: MsgDeposit{
				StorageProvider:      sample.RandAccAddressHex(),
				GlobalVirtualGroupId: 1,
				Deposit: types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(0),
				},
			},
			err: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "valid case",
			msg: *NewMsgDeposit(
				sample.RandAccAddress(),
				1,
				types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(1),
				},
			),
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

func TestMsgWithdraw_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgWithdraw
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgWithdraw{
				StorageProvider:      "invalid_address",
				GlobalVirtualGroupId: 1,
				Withdraw: types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(1),
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid deposit amount",
			msg: MsgWithdraw{
				StorageProvider:      sample.RandAccAddressHex(),
				GlobalVirtualGroupId: 1,
				Withdraw: types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(0),
				},
			},
			err: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "valid case",
			msg: *NewMsgWithdraw(
				sample.RandAccAddress(),
				1,
				types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(1),
				},
			),
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

func TestMsgSwapOut_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgSwapOut
		err  error
	}{
		{
			name: "valid case",
			msg: MsgSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupIds:      []uint32{1, 2, 3},
				SuccessorSpId:              100,
				SuccessorSpApproval: &common.Approval{
					ExpiredHeight:              100,
					GlobalVirtualGroupFamilyId: 1,
					Sig:                        []byte("sig"),
				},
			},
		},
		{
			name: "valid case",
			msg: MsgSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupIds:      []uint32{},
				SuccessorSpId:              100,
				SuccessorSpApproval: &common.Approval{
					ExpiredHeight:              100,
					GlobalVirtualGroupFamilyId: 1,
					Sig:                        []byte("sig"),
				},
			},
		},
		{
			name: "invalid address",
			msg: MsgSwapOut{
				StorageProvider:            "invalid address",
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupIds:      []uint32{},
				SuccessorSpId:              100,
				SuccessorSpApproval: &common.Approval{
					ExpiredHeight:              100,
					GlobalVirtualGroupFamilyId: 1,
					Sig:                        []byte("sig"),
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid virtual group family",
			msg: MsgSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupIds:      []uint32{1},
				SuccessorSpId:              100,
				SuccessorSpApproval: &common.Approval{
					ExpiredHeight:              100,
					GlobalVirtualGroupFamilyId: 1,
					Sig:                        []byte("sig"),
				},
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid virtual group family",
			msg: MsgSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupIds:      []uint32{},
				SuccessorSpId:              100,
				SuccessorSpApproval: &common.Approval{
					ExpiredHeight:              100,
					GlobalVirtualGroupFamilyId: 1,
					Sig:                        []byte("sig"),
				},
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid successor sp id",
			msg: MsgSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupIds:      []uint32{},
				SuccessorSpId:              0,
				SuccessorSpApproval: &common.Approval{
					ExpiredHeight:              100,
					GlobalVirtualGroupFamilyId: 1,
					Sig:                        []byte("sig"),
				},
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid successor sp approval",
			msg: MsgSwapOut{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupIds:      []uint32{},
				SuccessorSpId:              1,
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

func TestMsgCreateGlobalVirtualGroup_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCreateGlobalVirtualGroup
		err  error
	}{
		{
			name: "valid case",
			msg: *NewMsgCreateGlobalVirtualGroup(
				sample.RandAccAddress(),
				1,
				[]uint32{2, 3, 4},
				types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(1),
				},
			),
		},
		{
			name: "invalid address",
			msg: MsgCreateGlobalVirtualGroup{
				StorageProvider: "invalid_address",
				FamilyId:        1,
				SecondarySpIds:  []uint32{2, 3, 4},
				Deposit: types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(1),
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid deposit coin",
			msg: MsgCreateGlobalVirtualGroup{
				StorageProvider: "invalid_address",
				FamilyId:        1,
				SecondarySpIds:  []uint32{2, 3, 4},
				Deposit: types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(0),
				},
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid deposit amount",
			msg: MsgCreateGlobalVirtualGroup{
				StorageProvider: sample.RandAccAddressHex(),
				FamilyId:        1,
				SecondarySpIds:  []uint32{2, 3, 4},
				Deposit: types.Coin{
					Denom:  "denom",
					Amount: math.NewInt(0),
				},
			},
			err: sdkerrors.ErrInvalidRequest,
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

func TestMsgDeleteGlobalVirtualGroup_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgDeleteGlobalVirtualGroup
		err  error
	}{
		{
			name: "valid case",
			msg: *NewMsgDeleteGlobalVirtualGroup(
				sample.RandAccAddress(),
				1,
			),
		},
		{
			name: "invalid address",
			msg: MsgDeleteGlobalVirtualGroup{
				StorageProvider:      "invalid_address",
				GlobalVirtualGroupId: 1,
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

func TestMsgSettle_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgSettle
		err  error
	}{
		{
			name: "valid case",
			msg: *NewMsgSettle(
				sample.RandAccAddress(),
				1,
				[]uint32{1, 2, 3, 4},
			),
		},
		{
			name: "invalid address",
			msg: MsgSettle{
				StorageProvider:            "invalid_address",
				GlobalVirtualGroupFamilyId: 1,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid gvg ids",
			msg: MsgSettle{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
			},
			err: ErrInvalidGVGCount,
		},
		{
			name: "invalid gvg ids",
			msg: MsgSettle{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupIds:      []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			},
			err: ErrInvalidGVGCount,
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

func TestMsgReserveSwapIn_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgReserveSwapIn
		err  error
	}{
		{
			name: "valid case",
			msg: MsgReserveSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				TargetSpId:                 1,
				GlobalVirtualGroupId:       0,
			},
		},
		{
			name: "valid case",
			msg: MsgReserveSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				TargetSpId:                 1,
				GlobalVirtualGroupId:       1,
			},
		},
		{
			name: "invalid address",
			msg: MsgReserveSwapIn{
				StorageProvider:            "invalid_address",
				GlobalVirtualGroupFamilyId: 0,
				TargetSpId:                 1,
				GlobalVirtualGroupId:       1,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid virtual group family",
			msg: MsgReserveSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				TargetSpId:                 1,
				GlobalVirtualGroupId:       0,
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid virtual group",
			msg: MsgReserveSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				TargetSpId:                 1,
				GlobalVirtualGroupId:       1,
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid successor sp id",
			msg: MsgReserveSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupId:       0,
				TargetSpId:                 0,
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

func TestMsgCompleteSwapIn_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCompleteSwapIn
		err  error
	}{
		{
			name: "valid case",
			msg: MsgCompleteSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupId:       0,
			},
		},
		{
			name: "valid case",
			msg: MsgCompleteSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupId:       1,
			},
		},
		{
			name: "invalid address",
			msg: MsgCompleteSwapIn{
				StorageProvider:            "invalid_address",
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupId:       1,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid virtual group family",
			msg: MsgCompleteSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupId:       0,
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid virtual group",
			msg: MsgCompleteSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupId:       1,
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

func TestMsgCancelSwapIn_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgCancelSwapIn
		err  error
	}{
		{
			name: "valid case",
			msg: MsgCancelSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupId:       0,
			},
		},
		{
			name: "valid case",
			msg: MsgCancelSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupId:       1,
			},
		},
		{
			name: "invalid address",
			msg: MsgCancelSwapIn{
				StorageProvider:            "invalid_address",
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupId:       1,
			},
			err: sdkerrors.ErrInvalidAddress,
		},
		{
			name: "invalid virtual group family",
			msg: MsgCancelSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 0,
				GlobalVirtualGroupId:       0,
			},
			err: gnfderrors.ErrInvalidMessage,
		},
		{
			name: "invalid virtual group",
			msg: MsgCancelSwapIn{
				StorageProvider:            sample.RandAccAddressHex(),
				GlobalVirtualGroupFamilyId: 1,
				GlobalVirtualGroupId:       1,
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

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	tests := []struct {
		name   string
		msg    MsgUpdateParams
		errMsg string
	}{
		{
			name: "valid case",
			msg: MsgUpdateParams{
				Authority: sample.RandAccAddressHex(),
				Params:    DefaultParams(),
			},
		},
		{
			name: "invalid authority",
			msg: MsgUpdateParams{
				Authority: "invalid_address",
				Params:    DefaultParams(),
			},
			errMsg: errInvalidAuthorityAddress,
		},
		{
			name: "invalid params",
			msg: MsgUpdateParams{
				Authority: sample.RandAccAddressHex(),
				Params:    Params{},
			},
			errMsg: "deposit denom cannot be blank",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.errMsg != "" {
				require.ErrorContains(t, err, tt.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestMessageRouteAndType covers the trivial Route/Type accessors shared by
// every legacy-amino message in this file; none of them branch, so a single
// call each is enough to exercise the statement and pin the routing string.
func TestMessageRouteAndType(t *testing.T) {
	tests := []struct {
		name     string
		routeFn  func() string
		typeFn   func() string
		wantType string
	}{
		{caseCreateGlobalVirtualGroup, (&MsgCreateGlobalVirtualGroup{}).Route, (&MsgCreateGlobalVirtualGroup{}).Type, TypeMsgCreateGlobalVirtualGroup},
		{caseDeleteGlobalVirtualGroup, (&MsgDeleteGlobalVirtualGroup{}).Route, (&MsgDeleteGlobalVirtualGroup{}).Type, TypeMsgDeleteGlobalVirtualGroup},
		{TypeMsgDeposit, (&MsgDeposit{}).Route, (&MsgDeposit{}).Type, TypeMsgDeposit},
		{TypeMsgWithdraw, (&MsgWithdraw{}).Route, (&MsgWithdraw{}).Type, TypeMsgWithdraw},
		{caseSwapOut, (&MsgSwapOut{}).Route, (&MsgSwapOut{}).Type, TypeMsgSwapOut},
		{TypeMsgSettle, (&MsgSettle{}).Route, (&MsgSettle{}).Type, TypeMsgSettle},
		{caseReserveSwapIn, (&MsgReserveSwapIn{}).Route, (&MsgReserveSwapIn{}).Type, TypeMsgReserveSwapIn},
		{"cancel swap in", (&MsgCancelSwapIn{}).Route, (&MsgCancelSwapIn{}).Type, TypeMsgCancelSwapIn},
		{caseCompleteSwapIn, (&MsgCompleteSwapIn{}).Route, (&MsgCompleteSwapIn{}).Type, TypeMsgCompleteSwapIn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, RouterKey, tt.routeFn())
			require.Equal(t, tt.wantType, tt.typeFn())
		})
	}
}

// TestMessageGetSignBytes covers GetSignBytes for every message type that
// defines it in this file (MsgCancelSwapIn has none). A real assertion on
// content, not just non-empty bytes, so a broken marshal path fails loudly.
func TestMessageGetSignBytes(t *testing.T) {
	addr := sample.RandAccAddressHex()

	tests := []struct {
		name string
		msg  interface{ GetSignBytes() []byte }
	}{
		{caseCreateGlobalVirtualGroup, &MsgCreateGlobalVirtualGroup{StorageProvider: addr}},
		{caseDeleteGlobalVirtualGroup, &MsgDeleteGlobalVirtualGroup{StorageProvider: addr}},
		{TypeMsgDeposit, &MsgDeposit{StorageProvider: addr}},
		{TypeMsgWithdraw, &MsgWithdraw{StorageProvider: addr}},
		{caseSwapOut, &MsgSwapOut{StorageProvider: addr}},
		{"update params", &MsgUpdateParams{Authority: addr, Params: DefaultParams()}},
		{TypeMsgSettle, &MsgSettle{StorageProvider: addr}},
		{caseReserveSwapIn, &MsgReserveSwapIn{StorageProvider: addr}},
		{caseCompleteSwapIn, &MsgCompleteSwapIn{StorageProvider: addr}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bz := tt.msg.GetSignBytes()
			require.NotEmpty(t, bz)
			require.Contains(t, string(bz), addr)
		})
	}
}

// TestMessageGetSigners covers the happy path of every GetSigners in this
// file. The panic-on-parse-error branches a few of them have are only
// reachable with an address that ValidateBasic would already have rejected,
// so they are intentionally left uncovered as dead defensive code.
func TestMessageGetSigners(t *testing.T) {
	acc := sample.RandAccAddress()
	addr := acc.String()

	tests := []struct {
		name string
		fn   func() []types.AccAddress
	}{
		{caseCreateGlobalVirtualGroup, (&MsgCreateGlobalVirtualGroup{StorageProvider: addr}).GetSigners},
		{caseDeleteGlobalVirtualGroup, (&MsgDeleteGlobalVirtualGroup{StorageProvider: addr}).GetSigners},
		{TypeMsgDeposit, (&MsgDeposit{StorageProvider: addr}).GetSigners},
		{TypeMsgWithdraw, (&MsgWithdraw{StorageProvider: addr}).GetSigners},
		{caseSwapOut, (&MsgSwapOut{StorageProvider: addr}).GetSigners},
		{"update params", (&MsgUpdateParams{Authority: addr}).GetSigners},
		{TypeMsgSettle, (&MsgSettle{StorageProvider: addr}).GetSigners},
		{caseReserveSwapIn, (&MsgReserveSwapIn{StorageProvider: addr}).GetSigners},
		{"cancel swap in", (&MsgCancelSwapIn{StorageProvider: addr}).GetSigners},
		{caseCompleteSwapIn, (&MsgCompleteSwapIn{StorageProvider: addr}).GetSigners},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, []types.AccAddress{acc}, tt.fn())
		})
	}
}

func TestMsgSwapOut_GetApprovalBytes(t *testing.T) {
	msg := &MsgSwapOut{
		StorageProvider: sample.RandAccAddressHex(),
		SuccessorSpApproval: &common.Approval{
			ExpiredHeight: 100,
			Sig:           []byte("sig"),
		},
	}
	signedBytes := msg.GetSignBytes()

	approvalBytes := msg.GetApprovalBytes()
	require.NotEmpty(t, approvalBytes)
	// GetApprovalBytes must hash the approval with the signature blanked out,
	// so it has to differ from the fully-signed bytes...
	require.NotEqual(t, signedBytes, approvalBytes)
	// ...and it must do so on a clone, leaving the original message alone.
	require.Equal(t, []byte("sig"), msg.SuccessorSpApproval.Sig)

	msg.SuccessorSpApproval.Sig = nil
	require.Equal(t, msg.GetSignBytes(), approvalBytes)
}
