package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/storage/types"
)

// TestRegisterCodec checks that every concrete message RegisterCodec wires up
// round-trips through legacy amino JSON under its documented wire name --
// that name is part of the amino/Ledger wire contract, so a typo in it would
// otherwise go completely unnoticed.
func TestRegisterCodec(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	types.RegisterCodec(cdc)

	tests := []struct {
		wantType string
		msg      sdk.Msg
	}{
		{"storage/CreateBucket", &types.MsgCreateBucket{}},
		{"storage/DeleteBucket", &types.MsgDeleteBucket{}},
		{"storage/CreateObject", &types.MsgCreateObject{}},
		{"storage/SealObject", &types.MsgSealObject{}},
		{"storage/RejectSealObject", &types.MsgRejectSealObject{}},
		{"storage/DeleteObject", &types.MsgDeleteObject{}},
		{"storage/CreateGroup", &types.MsgCreateGroup{}},
		{"storage/DeleteGroup", &types.MsgDeleteGroup{}},
		{"storage/UpdateGroupMember", &types.MsgUpdateGroupMember{}},
		{"storage/UpdateGroupExtra", &types.MsgUpdateGroupExtra{}},
		{"storage/LeaveGroup", &types.MsgLeaveGroup{}},
		{"storage/CopyObject", &types.MsgCopyObject{}},
		{"storage/UpdateBucketInfo", &types.MsgUpdateBucketInfo{}},
		{"storage/CancelCreateObject", &types.MsgCancelCreateObject{}},
		{"storage/DeletePolicy", &types.MsgDeletePolicy{}},
		{"storage/MigrateBucket", &types.MsgMigrateBucket{}},
		{"storage/CompleteMigrateBucket", &types.MsgCompleteMigrateBucket{}},
		{"storage/CancelMigrateBucket", &types.MsgCancelMigrateBucket{}},
		{"storage/RejectMigrateBucket", &types.MsgRejectMigrateBucket{}},
		{"storage/SetBucketFlowRateLimit", &types.MsgSetBucketFlowRateLimit{}},
	}
	for _, tt := range tests {
		t.Run(tt.wantType, func(t *testing.T) {
			bz, err := cdc.MarshalJSON(tt.msg)
			require.NoError(t, err)
			require.Contains(t, string(bz), `"type":"`+tt.wantType+`"`)
		})
	}
}

// TestRegisterInterfaces checks that every message RegisterInterfaces wires
// up as an sdk.Msg implementation resolves back to its own concrete type
// through a fresh registry, and that a message which was never registered
// stays unresolvable.
func TestRegisterInterfaces(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)

	msgs := []sdk.Msg{
		&types.MsgCreateBucket{},
		&types.MsgDeleteBucket{},
		&types.MsgUpdateBucketInfo{},
		&types.MsgDiscontinueBucket{},
		&types.MsgCreateObject{},
		&types.MsgSealObject{},
		&types.MsgRejectSealObject{},
		&types.MsgCopyObject{},
		&types.MsgDeleteObject{},
		&types.MsgCancelCreateObject{},
		&types.MsgDiscontinueObject{},
		&types.MsgUpdateObjectInfo{},
		&types.MsgCreateGroup{},
		&types.MsgDeleteGroup{},
		&types.MsgUpdateGroupMember{},
		&types.MsgUpdateGroupExtra{},
		&types.MsgLeaveGroup{},
		&types.MsgPutPolicy{},
		&types.MsgDeletePolicy{},
		&types.MsgUpdateParams{},
		&types.MsgMigrateBucket{},
		&types.MsgCompleteMigrateBucket{},
		&types.MsgCancelMigrateBucket{},
		&types.MsgRejectMigrateBucket{},
		&types.MsgUpdateObjectContent{},
		&types.MsgCancelUpdateObjectContent{},
		&types.MsgDelegateCreateObject{},
		&types.MsgToggleSPAsDelegatedAgent{},
		&types.MsgSealObjectV2{},
		&types.MsgDelegateUpdateObjectContent{},
		&types.MsgSetBucketFlowRateLimit{},
	}
	for _, msg := range msgs {
		typeURL := cdctypes.MsgTypeURL(msg)
		t.Run(typeURL, func(t *testing.T) {
			resolved, err := registry.Resolve(typeURL)
			require.NoError(t, err)
			require.IsType(t, msg, resolved)
		})
	}

	// A message from a different module's service was never handed to this
	// registry, so it must stay unresolvable -- Resolve should not fall back
	// to any global registry.
	_, err := registry.Resolve(cdctypes.MsgTypeURL(&banktypes.MsgSend{}))
	require.Error(t, err, "a message this registry never saw must stay unresolvable")
}
