package storage

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	cmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	mocacommon "github.com/mocachain/moca/v2/types/common"
	permTypes "github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// CreateBucketMethodName is the ABI name for the createBucket transaction.
	CreateBucketMethodName = "createBucket"
	// DeleteBucketMethodName is the ABI name for the deleteBucket transaction.
	DeleteBucketMethodName = "deleteBucket"
	// DiscontinueBucketMethodName is the ABI name for the discontinueBucket transaction.
	DiscontinueBucketMethodName = "discontinueBucket"
	// MigrateBucketMethodName is the ABI name for the migrateBucket transaction.
	MigrateBucketMethodName = "migrateBucket"
	// CompleteMigrateBucketMethodName is the ABI name for the completeMigrateBucket transaction.
	CompleteMigrateBucketMethodName = "completeMigrateBucket"
	// RejectMigrateBucketMethodName is the ABI name for the rejectMigrateBucket transaction.
	RejectMigrateBucketMethodName = "rejectMigrateBucket"
	// CancelMigrateBucketMethodName is the ABI name for the cancelMigrateBucket transaction.
	CancelMigrateBucketMethodName = "cancelMigrateBucket"
	// SetBucketFlowRateLimitMethodName is the ABI name for the setBucketFlowRateLimit transaction.
	SetBucketFlowRateLimitMethodName = "setBucketFlowRateLimit"
	// CreateObjectMethodName is the ABI name for the createObject transaction.
	CreateObjectMethodName = "createObject"
	// CopyObjectMethodName is the ABI name for the copyObject transaction.
	CopyObjectMethodName = "copyObject"
	// DeleteObjectMethodName is the ABI name for the deleteObject transaction.
	DeleteObjectMethodName = "deleteObject"
	// CancelCreateObjectMethodName is the ABI name for the cancelCreateObject transaction.
	CancelCreateObjectMethodName = "cancelCreateObject"
	// SealObjectMethodName is the ABI name for the sealObject transaction.
	SealObjectMethodName = "sealObject"
	// SealObjectV2MethodName is the ABI name for the sealObjectV2 transaction.
	SealObjectV2MethodName = "sealObjectV2"
	// RejectSealObjectMethodName is the ABI name for the rejectSealObject transaction.
	RejectSealObjectMethodName = "rejectSealObject"
	// DelegateCreateObjectMethodName is the ABI name for the delegateCreateObject transaction.
	DelegateCreateObjectMethodName = "delegateCreateObject"
	// DelegateUpdateObjectContentMethodName is the ABI name for the delegateUpdateObjectContent transaction.
	DelegateUpdateObjectContentMethodName = "delegateUpdateObjectContent"
	// UpdateObjectInfoMethodName is the ABI name for the updateObjectInfo transaction.
	UpdateObjectInfoMethodName = "updateObjectInfo"
	// UpdateObjectContentMethodName is the ABI name for the updateObjectContent transaction.
	UpdateObjectContentMethodName = "updateObjectContent"
	// DiscontinueObjectMethodName is the ABI name for the discontinueObject transaction.
	DiscontinueObjectMethodName = "discontinueObject"
	// CreateGroupMethodName is the ABI name for the createGroup transaction.
	CreateGroupMethodName = "createGroup"
	// UpdateGroupMethodName is the ABI name for the updateGroup transaction.
	UpdateGroupMethodName = "updateGroup"
	// UpdateGroupExtraMethodName is the ABI name for the updateGroupExtra transaction.
	UpdateGroupExtraMethodName = "updateGroupExtra"
	// DeleteGroupMethodName is the ABI name for the deleteGroup transaction.
	DeleteGroupMethodName = "deleteGroup"
	// LeaveGroupMethodName is the ABI name for the leaveGroup transaction.
	LeaveGroupMethodName = "leaveGroup"
	// RenewGroupMemberMethodName is the ABI name for the renewGroupMember transaction.
	RenewGroupMemberMethodName = "renewGroupMember"
	// SetTagMethodName is the ABI name for the setTag transaction.
	SetTagMethodName = "setTag"
	// UpdateBucketInfoMethodName is the ABI name for the updateBucketInfo transaction.
	UpdateBucketInfoMethodName = "updateBucketInfo"
	// PutPolicyMethodName is the ABI name for the putPolicy transaction.
	PutPolicyMethodName = "putPolicy"
	// DeletePolicyMethodName is the ABI name for the deletePolicy transaction.
	DeletePolicyMethodName = "deletePolicy"
	// ToggleSPAsDelegatedAgentMethodName is the ABI name for the toggleSPAsDelegatedAgent transaction.
	ToggleSPAsDelegatedAgentMethodName = "toggleSPAsDelegatedAgent"
	// CancelUpdateObjectContentMethodName is the ABI name for the cancelUpdateObjectContent transaction.
	CancelUpdateObjectContentMethodName = "cancelUpdateObjectContent"
)

// CreateBucket creates a new bucket owned by the caller and mirrors the bucket-NFT
// mint as an ERC721 Transfer log on the bucket token contract.
func (p Precompile) CreateBucket(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CreateBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgCreateBucket{
		Creator:          contract.Caller().String(),
		BucketName:       input.BucketName,
		Visibility:       storagetypes.VisibilityType(input.Visibility),
		PaymentAddress:   input.PaymentAddress.String(),
		PrimarySpAddress: input.PrimarySpAddress.String(),
		PrimarySpApproval: &mocacommon.Approval{
			ExpiredHeight:              input.PrimarySpApproval.ExpiredHeight,
			GlobalVirtualGroupFamilyId: input.PrimarySpApproval.GlobalVirtualGroupFamilyId,
			Sig:                        input.PrimarySpApproval.Sig,
		},
		ChargedReadQuota: input.ChargedReadQuota,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	res, err := p.storageMsgServer.CreateBucket(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := p.EmitCreateBucketEvent(evm, contract.Caller(), input.PaymentAddress, input.PrimarySpAddress, res.BucketId.BigInt()); err != nil {
		return nil, err
	}

	bucketInfo, found := p.storageKeeper.GetBucketInfo(ctx, input.BucketName)
	if found {
		if err := p.EmitBucketTransferEvent(evm, bucketInfo.Owner, bucketInfo.Id.BigInt()); err != nil {
			return nil, err
		}
	}

	return method.Outputs.Pack(true)
}

// UpdateBucketInfo updates a bucket's visibility, payment address and read quota.
func (p Precompile) UpdateBucketInfo(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input UpdateBucketInfoArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if input.BucketName == "" {
		return nil, errors.New("empty bucket name")
	}
	if input.ChargedReadQuota.Int64() != -1 && !input.ChargedReadQuota.IsUint64() {
		return nil, errors.New("charged read quota is invalid")
	}

	msg := &storagetypes.MsgUpdateBucketInfo{
		Operator:       contract.Caller().String(),
		BucketName:     input.BucketName,
		Visibility:     storagetypes.VisibilityType(input.Visibility),
		PaymentAddress: input.PaymentAddress.String(),
	}
	if input.PaymentAddress == (common.Address{}) {
		msg.PaymentAddress = ""
	}
	if input.ChargedReadQuota.Int64() == -1 {
		msg.ChargedReadQuota = nil
	} else {
		msg.ChargedReadQuota = &mocacommon.UInt64Value{Value: input.ChargedReadQuota.Uint64()}
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.UpdateBucketInfo(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitUpdateBucketInfoEvent(evm, contract.Caller(), input.BucketName, input.PaymentAddress, input.Visibility); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DeleteBucket deletes a bucket owned by the caller.
func (p Precompile) DeleteBucket(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DeleteBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgDeleteBucket{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DeleteBucket(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDeleteBucketEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DiscontinueBucket discontinues a bucket with a reason.
func (p Precompile) DiscontinueBucket(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DiscontinueBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgDiscontinueBucket{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		Reason:     strings.TrimSpace(input.Reason),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DiscontinueBucket(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDiscontinueBucketEvent(evm, contract.Caller(), input.BucketName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// MigrateBucket migrates a bucket to a destination primary storage provider.
func (p Precompile) MigrateBucket(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input MigrateBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgMigrateBucket{
		Operator:       contract.Caller().String(),
		BucketName:     input.BucketName,
		DstPrimarySpId: input.DstPrimarySpID,
		DstPrimarySpApproval: &mocacommon.Approval{
			ExpiredHeight: input.DstPrimarySpApproval.ExpiredHeight,
			Sig:           input.DstPrimarySpApproval.Sig,
		},
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.MigrateBucket(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitMigrateBucketEvent(evm, contract.Caller(), input.BucketName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CompleteMigrateBucket completes a bucket migration with the new gvg mappings.
func (p Precompile) CompleteMigrateBucket(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CompleteMigrateBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	gvgMappings := make([]*storagetypes.GVGMapping, 0)
	for _, gvgMapping := range input.GvgMappings {
		gvgMappings = append(gvgMappings, &storagetypes.GVGMapping{
			SrcGlobalVirtualGroupId: gvgMapping.SrcGlobalVirtualGroupId,
			DstGlobalVirtualGroupId: gvgMapping.DstGlobalVirtualGroupId,
			SecondarySpBlsSignature: gvgMapping.SecondarySpBlsSignature,
		})
	}

	msg := &storagetypes.MsgCompleteMigrateBucket{
		Operator:                   contract.Caller().String(),
		BucketName:                 input.BucketName,
		GlobalVirtualGroupFamilyId: input.GvgFamilyID,
		GvgMappings:                gvgMappings,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.CompleteMigrateBucket(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCompleteMigrateBucketEvent(evm, contract.Caller(), input.BucketName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// RejectMigrateBucket rejects a pending bucket migration.
func (p Precompile) RejectMigrateBucket(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input RejectMigrateBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgRejectMigrateBucket{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.RejectMigrateBucket(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitRejectMigrateBucketEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CancelMigrateBucket cancels a pending bucket migration.
func (p Precompile) CancelMigrateBucket(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CancelMigrateBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgCancelMigrateBucket{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.CancelMigrateBucket(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCancelMigrateBucketEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// SetBucketFlowRateLimit sets the payment flow rate limit for a bucket.
func (p Precompile) SetBucketFlowRateLimit(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input SetBucketFlowRateLimitArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgSetBucketFlowRateLimit{
		Operator:       contract.Caller().String(),
		BucketName:     input.BucketName,
		BucketOwner:    input.BucketOwner,
		PaymentAddress: input.PaymentAddress,
		FlowRateLimit:  cmath.NewIntFromBigInt(input.FlowRateLimit),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.SetBucketFlowRateLimit(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSetBucketFlowRateLimitEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CreateObject creates a new object in a bucket owned/authorized to the caller.
func (p Precompile) CreateObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CreateObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	expectChecksums := make([][]byte, 0)
	for _, checksum := range input.ExpectChecksums {
		checksumBytes, err := base64.StdEncoding.DecodeString(checksum)
		if err != nil {
			return nil, err
		}
		expectChecksums = append(expectChecksums, checksumBytes)
	}

	msg := &storagetypes.MsgCreateObject{
		Creator:     contract.Caller().String(),
		BucketName:  input.BucketName,
		ObjectName:  input.ObjectName,
		PayloadSize: input.PayloadSize,
		Visibility:  storagetypes.VisibilityType(input.Visibility),
		ContentType: input.ContentType,
		PrimarySpApproval: &mocacommon.Approval{
			ExpiredHeight:              input.PrimarySpApproval.ExpiredHeight,
			GlobalVirtualGroupFamilyId: input.PrimarySpApproval.GlobalVirtualGroupFamilyId,
			Sig:                        input.PrimarySpApproval.Sig,
		},
		ExpectChecksums: expectChecksums,
		RedundancyType:  storagetypes.RedundancyType(input.RedundancyType),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	res, err := p.storageMsgServer.CreateObject(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := p.EmitCreateObjectEvent(evm, contract.Caller(), res.ObjectId.BigInt()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CopyObject copies an object into a destination bucket.
func (p Precompile) CopyObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CopyObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgCopyObject{
		Operator:      contract.Caller().String(),
		SrcBucketName: input.SrcBucketName,
		DstBucketName: input.DstBucketName,
		SrcObjectName: input.SrcObjectName,
		DstObjectName: input.DstObjectName,
		DstPrimarySpApproval: &mocacommon.Approval{
			ExpiredHeight: input.DstPrimarySpApproval.ExpiredHeight,
			Sig:           input.DstPrimarySpApproval.Sig,
		},
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.CopyObject(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCopyObjectEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DeleteObject deletes an object from a bucket.
func (p Precompile) DeleteObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DeleteObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgDeleteObject{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DeleteObject(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDeleteObjectEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CancelCreateObject cancels an in-progress object creation.
func (p Precompile) CancelCreateObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CancelCreateObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgCancelCreateObject{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.CancelCreateObject(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCancelCreateObjectEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// SealObject seals an object and mirrors the object-NFT mint as an ERC721 Transfer log.
func (p Precompile) SealObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input SealObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	secondarySpBlsAggSignatures, err := base64.StdEncoding.DecodeString(input.SecondarySpBlsAggSignatures)
	if err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgSealObject{
		Operator:                    contract.Caller().String(),
		BucketName:                  input.BucketName,
		ObjectName:                  input.ObjectName,
		GlobalVirtualGroupId:        input.GlobalVirtualGroupID,
		SecondarySpBlsAggSignatures: secondarySpBlsAggSignatures,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err = p.storageMsgServer.SealObject(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSealObjectEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	objectInfo, found := p.storageKeeper.GetObjectInfo(ctx, input.BucketName, input.ObjectName)
	if found {
		if err := p.EmitObjectTransferEvent(evm, objectInfo.Owner, objectInfo.Id.BigInt()); err != nil {
			return nil, err
		}
	}

	return method.Outputs.Pack(true)
}

// SealObjectV2 seals an object with expected checksums.
func (p Precompile) SealObjectV2(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input SealObjectV2Args
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	secondarySpBlsAggSignatures, err := base64.StdEncoding.DecodeString(input.SecondarySpBlsAggSignatures)
	if err != nil {
		return nil, err
	}

	expectChecksums := make([][]byte, 0)
	for _, checksum := range input.ExpectChecksums {
		checksumBytes, err := base64.StdEncoding.DecodeString(checksum)
		if err != nil {
			return nil, err
		}
		expectChecksums = append(expectChecksums, checksumBytes)
	}

	msg := &storagetypes.MsgSealObjectV2{
		Operator:                    contract.Caller().String(),
		BucketName:                  input.BucketName,
		ObjectName:                  input.ObjectName,
		GlobalVirtualGroupId:        input.GlobalVirtualGroupID,
		SecondarySpBlsAggSignatures: secondarySpBlsAggSignatures,
		ExpectChecksums:             expectChecksums,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err = p.storageMsgServer.SealObjectV2(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSealObjectV2Event(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// RejectSealObject rejects the sealing of an object.
func (p Precompile) RejectSealObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input RejectSealObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgRejectSealObject{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.RejectSealObject(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitRejectSealObjectEvent(evm, contract.Caller(), input.ObjectName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DelegateCreateObject creates an object on behalf of a creator via a delegated agent.
func (p Precompile) DelegateCreateObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DelegateCreateObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	expectChecksums := make([][]byte, 0)
	for _, checksum := range input.ExpectChecksums {
		checksumBytes, err := base64.StdEncoding.DecodeString(checksum)
		if err != nil {
			return nil, err
		}
		expectChecksums = append(expectChecksums, checksumBytes)
	}

	msg := &storagetypes.MsgDelegateCreateObject{
		Operator:        contract.Caller().String(),
		Creator:         input.Creator,
		BucketName:      input.BucketName,
		ObjectName:      input.ObjectName,
		PayloadSize:     input.PayloadSize,
		ContentType:     input.ContentType,
		Visibility:      storagetypes.VisibilityType(input.Visibility),
		ExpectChecksums: expectChecksums,
		RedundancyType:  storagetypes.RedundancyType(input.RedundancyType),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DelegateCreateObject(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDelegateCreateObjectEvent(evm, contract.Caller(), input.ObjectName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DelegateUpdateObjectContent updates an object's content via a delegated agent.
func (p Precompile) DelegateUpdateObjectContent(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DelegateUpdateObjectContentArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	expectChecksums := make([][]byte, 0)
	for _, checksum := range input.ExpectChecksums {
		checksumBytes, err := base64.StdEncoding.DecodeString(checksum)
		if err != nil {
			return nil, err
		}
		expectChecksums = append(expectChecksums, checksumBytes)
	}

	msg := &storagetypes.MsgDelegateUpdateObjectContent{
		Operator:        contract.Caller().String(),
		Updater:         input.Updater,
		BucketName:      input.BucketName,
		ObjectName:      input.ObjectName,
		PayloadSize:     input.PayloadSize,
		ContentType:     input.ContentType,
		ExpectChecksums: expectChecksums,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DelegateUpdateObjectContent(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDelegateUpdateObjectContentEvent(evm, contract.Caller(), input.ObjectName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// UpdateObjectInfo updates an object's visibility.
func (p Precompile) UpdateObjectInfo(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input UpdateObjectInfoArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgUpdateObjectInfo{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
		Visibility: storagetypes.VisibilityType(input.Visibility),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.UpdateObjectInfo(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitUpdateObjectInfoEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// UpdateObjectContent updates an object's content with expected checksums.
func (p Precompile) UpdateObjectContent(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input UpdateObjectContentArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	expectChecksums := make([][]byte, 0)
	for _, checksum := range input.ExpectChecksums {
		checksumBytes, err := base64.StdEncoding.DecodeString(checksum)
		if err != nil {
			return nil, err
		}
		expectChecksums = append(expectChecksums, checksumBytes)
	}

	msg := &storagetypes.MsgUpdateObjectContent{
		Operator:        contract.Caller().String(),
		BucketName:      input.BucketName,
		ObjectName:      input.ObjectName,
		PayloadSize:     input.PayloadSize,
		ContentType:     input.ContentType,
		ExpectChecksums: expectChecksums,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.UpdateObjectContent(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitUpdateObjectContentEvent(evm, contract.Caller(), input.ObjectName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DiscontinueObject discontinues a set of objects in a bucket with a reason.
func (p Precompile) DiscontinueObject(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DiscontinueObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	objectIDs := make([]storagetypes.Uint, 0)
	for _, id := range input.ObjectIDs {
		if id.Cmp(big.NewInt(0)) < 0 {
			return nil, fmt.Errorf("object id should not be negative")
		}
		objectIDs = append(objectIDs, cmath.NewUintFromBigInt(id))
	}

	msg := &storagetypes.MsgDiscontinueObject{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		ObjectIds:  objectIDs,
		Reason:     strings.TrimSpace(input.Reason),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DiscontinueObject(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDiscontinueObjectEvent(evm, contract.Caller(), input.BucketName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CreateGroup creates a new group owned by the caller and mirrors the group-NFT
// mint as an ERC721 Transfer log on the group token contract.
func (p Precompile) CreateGroup(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CreateGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgCreateGroup{
		Creator:   contract.Caller().String(),
		GroupName: input.GroupName,
		Extra:     input.Extra,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	res, err := p.storageMsgServer.CreateGroup(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := p.EmitCreateGroupEvent(evm, contract.Caller(), res.GroupId.BigInt()); err != nil {
		return nil, err
	}

	address := sdk.MustAccAddressFromHex(contract.Caller().String())
	groupInfo, found := p.storageKeeper.GetGroupInfo(ctx, address, input.GroupName)
	if found {
		if err := p.EmitGroupTransferEvent(evm, groupInfo.Owner, groupInfo.Id.BigInt()); err != nil {
			return nil, err
		}
	}

	return method.Outputs.Pack(true)
}

// UpdateGroup adds and removes members of a group.
func (p Precompile) UpdateGroup(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input UpdateGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if input.GroupName == "" {
		return nil, errors.New("group name is empty")
	}
	if len(input.MembersToAdd) == 0 && len(input.MembersToDelete) == 0 {
		return nil, errors.New("no update member")
	}
	if input.ExpirationTime != nil && len(input.MembersToAdd) != len(input.ExpirationTime) {
		return nil, errors.New("please provide expirationTime for every new add member")
	}

	membersToAdd := make([]*storagetypes.MsgGroupMember, 0)
	for i, members := range input.MembersToAdd {
		var exp time.Time
		if input.ExpirationTime[i] != 0 {
			exp = time.Unix(input.ExpirationTime[i], 0)
		} else {
			exp = storagetypes.MaxTimeStamp
		}
		membersToAdd = append(membersToAdd, &storagetypes.MsgGroupMember{
			Member:         members.String(),
			ExpirationTime: &exp,
		})
	}
	var membersToDelete []string
	for _, members := range input.MembersToDelete {
		membersToDelete = append(membersToDelete, members.String())
	}

	msg := &storagetypes.MsgUpdateGroupMember{
		Operator:        contract.Caller().String(),
		GroupOwner:      input.GroupOwner.String(),
		GroupName:       input.GroupName,
		MembersToAdd:    membersToAdd,
		MembersToDelete: membersToDelete,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.UpdateGroupMember(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitUpdateGroupEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// UpdateGroupExtra updates a group's extra metadata.
func (p Precompile) UpdateGroupExtra(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input UpdateGroupExtraArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgUpdateGroupExtra{
		Operator:   contract.Caller().String(),
		GroupOwner: input.GroupOwner.String(),
		GroupName:  input.GroupName,
		Extra:      input.Extra,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.UpdateGroupExtra(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitUpdateGroupExtraEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DeleteGroup deletes a group owned by the caller.
func (p Precompile) DeleteGroup(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DeleteGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgDeleteGroup{
		Operator:  contract.Caller().String(),
		GroupName: input.GroupName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DeleteGroup(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDeleteGroupEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// LeaveGroup removes the caller from a group.
func (p Precompile) LeaveGroup(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input LeaveGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgLeaveGroup{
		Member:     contract.Caller().String(),
		GroupOwner: input.GroupOwner.String(),
		GroupName:  input.GroupName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.LeaveGroup(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitLeaveGroupEvent(evm, contract.Caller(), input.GroupName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// RenewGroupMember renews the membership expiration of a set of group members.
func (p Precompile) RenewGroupMember(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input RenewGroupMemberArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if input.GroupName == "" {
		return nil, errors.New("group name is empty")
	}
	if len(input.Members) == 0 {
		return nil, errors.New("no renew member")
	}
	if input.ExpirationTime != nil && len(input.Members) != len(input.ExpirationTime) {
		return nil, errors.New("please provide expirationTime for every renew member")
	}

	membersToRenew := make([]*storagetypes.MsgGroupMember, 0)
	for i, members := range input.Members {
		var exp time.Time
		if input.ExpirationTime[i] != 0 {
			exp = time.Unix(input.ExpirationTime[i], 0)
		} else {
			exp = storagetypes.MaxTimeStamp
		}
		membersToRenew = append(membersToRenew, &storagetypes.MsgGroupMember{
			Member:         members.String(),
			ExpirationTime: &exp,
		})
	}

	msg := &storagetypes.MsgRenewGroupMember{
		Operator:   contract.Caller().String(),
		GroupOwner: input.GroupOwner.String(),
		GroupName:  input.GroupName,
		Members:    membersToRenew,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.RenewGroupMember(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitRenewGroupMemberEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// SetTag sets the resource tags for a bucket, object or group resource.
func (p Precompile) SetTag(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input SetTagArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if input.Tags == nil {
		return nil, errors.New("invalid tags parameter")
	}

	var tags storagetypes.ResourceTags
	for _, tag := range input.Tags {
		tags.Tags = append(tags.Tags, storagetypes.ResourceTags_Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}

	msg := &storagetypes.MsgSetTag{
		Operator: contract.Caller().String(),
		Resource: input.Resource,
		Tags:     &tags,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.SetTag(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSetTagEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// PutPolicy attaches an access-control policy to a resource for a principal.
func (p Precompile) PutPolicy(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input PutPolicyArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	statements := make([]*permTypes.Statement, 0)
	for _, statement := range input.Statements {
		actions := make([]permTypes.ActionType, 0)
		for _, action := range statement.Actions {
			actions = append(actions, permTypes.ActionType(action))
		}
		s := &permTypes.Statement{
			Effect:    permTypes.Effect(statement.Effect),
			Actions:   actions,
			Resources: statement.Resources,
		}
		if statement.ExpirationTime != 0 {
			tm := time.Unix(statement.ExpirationTime, 0)
			s.ExpirationTime = &tm
		}
		if statement.LimitSize != 0 {
			s.LimitSize = &mocacommon.UInt64Value{Value: statement.LimitSize}
		}
		statements = append(statements, s)
	}

	var tmptr *time.Time
	if input.ExpirationTime != 0 {
		tm := time.Unix(input.ExpirationTime, 0)
		tmptr = &tm
	}

	msg := &storagetypes.MsgPutPolicy{
		Operator:       contract.Caller().String(),
		Principal:      &permTypes.Principal{Type: permTypes.PrincipalType(input.Principal.PrincipalType), Value: input.Principal.Value},
		Resource:       input.Resource,
		Statements:     statements,
		ExpirationTime: tmptr,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.PutPolicy(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitPutPolicyEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DeletePolicy deletes the access-control policy of a principal on a resource.
func (p Precompile) DeletePolicy(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input DeletePolicyArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgDeletePolicy{
		Operator:  contract.Caller().String(),
		Principal: &permTypes.Principal{Type: permTypes.PrincipalType(input.Principal.PrincipalType), Value: input.Principal.Value},
		Resource:  input.Resource,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.DeletePolicy(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDeletePolicyEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// ToggleSPAsDelegatedAgent toggles whether the SP acts as a delegated agent for a bucket.
func (p Precompile) ToggleSPAsDelegatedAgent(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input ToggleSPAsDelegatedAgentArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgToggleSPAsDelegatedAgent{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.ToggleSPAsDelegatedAgent(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitToggleSPAsDelegatedAgentEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CancelUpdateObjectContent cancels an in-progress object content update.
func (p Precompile) CancelUpdateObjectContent(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input CancelUpdateObjectContentArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &storagetypes.MsgCancelUpdateObjectContent{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.storageMsgServer.CancelUpdateObjectContent(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCancelUpdateObjectContentEvent(evm, contract.Caller(), input.ObjectName); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
