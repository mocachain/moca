package keeper

import (
	"encoding/hex"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v12/types/common"
	"github.com/evmos/evmos/v12/x/storage/types"
)

var _ sdk.CrossChainApplication = &BucketApp{}

type BucketApp struct {
	storageKeeper types.StorageKeeper
}

func NewBucketApp(keeper types.StorageKeeper) *BucketApp {
	return &BucketApp{
		storageKeeper: keeper,
	}
}

func (app *BucketApp) ExecuteAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	pack, err := types.DeserializeCrossChainPackage(payload, types.BucketChannelId, sdk.AckCrossChainPackageType)
	if err != nil {
		app.storageKeeper.Logger(ctx).Error("deserialize bucket cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
		panic("deserialize bucket cross chain package error")
	}

	var operationType uint8
	var result sdk.ExecuteResult
	switch p := pack.(type) {
	case *types.MirrorBucketAckPackage:
		operationType = types.OperationMirrorBucket
		result = app.handleMirrorBucketAckPackage(ctx, appCtx, p)
	case *types.CreateBucketAckPackage:
		operationType = types.OperationCreateBucket
		result = app.handleCreateBucketAckPackage(ctx, appCtx, p)
	case *types.DeleteBucketAckPackage:
		operationType = types.OperationDeleteBucket
		result = app.handleDeleteBucketAckPackage(ctx, appCtx, p)
	default:
		panic("unknown cross chain ack package type")
	}

	if len(result.Payload) != 0 {
		wrapPayload := types.CrossChainPackage{
			OperationType: operationType,
			Package:       result.Payload,
		}
		result.Payload = wrapPayload.MustSerialize()
	}

	return result
}

// ExecuteFailAckPackage processes failed acknowledgment cross-chain packages.
//
// IMPORTANT: This function uses explicit branching on OperationType to distinguish
// V1 and V2 package formats. V1 and V2 CreateBucket operations are handled separately:
// - OperationCreateBucket (0x02): V1 format without GlobalVirtualGroupFamilyId
// - OperationCreateBucketV2 (0x82): V2 format with GlobalVirtualGroupFamilyId
//
// The high-bit flag in OperationType (0x80) ensures packages cannot be misinterpreted.
// Fallback deserialization (try V2, then V1) is explicitly prohibited as it can cause
// silent data corruption where V1 payloads are incorrectly parsed as V2.
//
// Unknown OperationType values will trigger panic as a safety measure.
func (app *BucketApp) ExecuteFailAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	rawPack, err := types.DeserializeRawCrossChainPackage(payload)
	if err != nil {
		app.storageKeeper.Logger(ctx).Error("deserialize raw cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
		panic("deserialize raw cross chain package error")
	}

	var operationType uint8
	var result sdk.ExecuteResult
	switch rawPack.OperationType {
	case types.OperationMirrorBucket:
		p, derr := types.DeserializeMirrorBucketSynPackage(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize mirror bucket failack package error", "error", derr.Error())
			panic("deserialize mirror bucket failack package error")
		}
		operationType = types.OperationMirrorBucket
		result = app.handleMirrorBucketFailAckPackage(ctx, appCtx, p.(*types.MirrorBucketSynPackage))
	case types.OperationCreateBucket: // V1: 0x02 (legacy format without GlobalVirtualGroupFamilyId)
		p, derr := types.DeserializeCreateBucketSynPackage(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize create bucket (v1) failack package error", "error", derr.Error())
			panic("deserialize create bucket (v1) failack package error")
		}
		operationType = types.OperationCreateBucket
		result = app.handleCreateBucketFailAckPackage(ctx, appCtx, p.(*types.CreateBucketSynPackage))
	case types.OperationCreateBucketV2: // V2: 0x82 (includes GlobalVirtualGroupFamilyId)
		p, derr := types.DeserializeCreateBucketSynPackageV2(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize create bucket (v2) failack package error", "error", derr.Error())
			panic("deserialize create bucket (v2) failack package error")
		}
		operationType = types.OperationCreateBucketV2
		result = app.handleCreateBucketFailAckPackageV2(ctx, appCtx, p.(*types.CreateBucketSynPackageV2))
	case types.OperationDeleteBucket:
		p, derr := types.DeserializeDeleteBucketSynPackage(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize delete bucket failack package error", "error", derr.Error())
			panic("deserialize delete bucket failack package error")
		}
		operationType = types.OperationDeleteBucket
		result = app.handleDeleteBucketFailAckPackage(ctx, appCtx, p.(*types.DeleteBucketSynPackage))
	default:
		panic("unknown cross chain ack package type")
	}

	// Wrap response payload with OperationType to maintain V1/V2 consistency.
	// The response OperationType matches the request (V2 request → V2 response).
	if len(result.Payload) != 0 {
		wrapPayload := types.CrossChainPackage{
			OperationType: operationType,
			Package:       result.Payload,
		}
		result.Payload = wrapPayload.MustSerialize()
	}

	return result
}

// ExecuteSynPackage processes synchronous cross-chain packages.
//
// IMPORTANT: This function uses explicit branching on OperationType to distinguish
// V1 and V2 package formats. V1 and V2 CreateBucket operations are handled separately:
// - OperationCreateBucket (0x02): V1 format without GlobalVirtualGroupFamilyId
// - OperationCreateBucketV2 (0x82): V2 format with GlobalVirtualGroupFamilyId
//
// The high-bit flag in OperationType (0x80) ensures packages cannot be misinterpreted.
// Fallback deserialization (try V2, then V1) is explicitly prohibited as it can cause
// silent data corruption where V1 payloads are incorrectly parsed as V2.
//
// Unknown OperationType values will trigger panic as a safety measure.
func (app *BucketApp) ExecuteSynPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	rawPack, err := types.DeserializeRawCrossChainPackage(payload)
	if err != nil {
		app.storageKeeper.Logger(ctx).Error("deserialize raw cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
		panic("deserialize raw cross chain package error")
	}

	var operationType uint8
	var result sdk.ExecuteResult
	switch rawPack.OperationType {
	case types.OperationMirrorBucket:
		p, derr := types.DeserializeMirrorBucketSynPackage(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize mirror bucket syn package error", "error", derr.Error())
			panic("deserialize mirror bucket syn package error")
		}
		operationType = types.OperationMirrorBucket
		result = app.handleMirrorBucketSynPackage(ctx, appCtx, p.(*types.MirrorBucketSynPackage))
	case types.OperationCreateBucket: // V1: 0x02 (legacy format without GlobalVirtualGroupFamilyId)
		p, derr := types.DeserializeCreateBucketSynPackage(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize create bucket (v1) syn package error", "error", derr.Error())
			panic("deserialize create bucket (v1) syn package error")
		}
		operationType = types.OperationCreateBucket
		result = app.handleCreateBucketSynPackage(ctx, appCtx, p.(*types.CreateBucketSynPackage))
	case types.OperationCreateBucketV2: // V2: 0x82 (includes GlobalVirtualGroupFamilyId)
		p, derr := types.DeserializeCreateBucketSynPackageV2(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize create bucket (v2) syn package error", "error", derr.Error())
			panic("deserialize create bucket (v2) syn package error")
		}
		operationType = types.OperationCreateBucketV2
		result = app.handleCreateBucketSynPackageV2(ctx, appCtx, p.(*types.CreateBucketSynPackageV2))
	case types.OperationDeleteBucket:
		p, derr := types.DeserializeDeleteBucketSynPackage(rawPack.Package)
		if derr != nil {
			app.storageKeeper.Logger(ctx).Error("deserialize delete bucket syn package error", "error", derr.Error())
			panic("deserialize delete bucket syn package error")
		}
		operationType = types.OperationDeleteBucket
		result = app.handleDeleteBucketSynPackage(ctx, appCtx, p.(*types.DeleteBucketSynPackage))
	default:
		panic("unknown cross chain ack package type")
	}

	// Wrap response payload with OperationType to maintain V1/V2 consistency.
	// The response OperationType matches the request (V2 request → V2 response).
	if len(result.Payload) != 0 {
		wrapPayload := types.CrossChainPackage{
			OperationType: operationType,
			Package:       result.Payload,
		}
		result.Payload = wrapPayload.MustSerialize()
	}

	return result
}

func (app *BucketApp) handleMirrorBucketAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, ackPackage *types.MirrorBucketAckPackage) sdk.ExecuteResult {
	bucketInfo, found := app.storageKeeper.GetBucketInfoById(ctx, math.NewUintFromBigInt(ackPackage.Id))
	if !found {
		app.storageKeeper.Logger(ctx).Error("bucket does not exist", "bucket id", ackPackage.Id.String())
		return sdk.ExecuteResult{
			Err: types.ErrNoSuchBucket,
		}
	}

	// update bucket
	if ackPackage.Status == types.StatusSuccess {
		sourceType, err := app.storageKeeper.GetSourceTypeByChainId(ctx, appCtx.SrcChainId)
		if err != nil {
			return sdk.ExecuteResult{
				Err: err,
			}
		}

		bucketInfo.SourceType = sourceType
		app.storageKeeper.SetBucketInfo(ctx, bucketInfo)
	}

	if err := ctx.EventManager().EmitTypedEvents(&types.EventMirrorBucketResult{
		Status:      uint32(ackPackage.Status),
		BucketName:  bucketInfo.BucketName,
		BucketId:    bucketInfo.Id,
		DestChainId: uint32(appCtx.SrcChainId),
	}); err != nil {
		return sdk.ExecuteResult{
			Err: err,
		}
	}

	return sdk.ExecuteResult{}
}

func (app *BucketApp) handleMirrorBucketFailAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, mirrorBucketPackage *types.MirrorBucketSynPackage) sdk.ExecuteResult {
	bucketInfo, found := app.storageKeeper.GetBucketInfoById(ctx, math.NewUintFromBigInt(mirrorBucketPackage.Id))
	if !found {
		app.storageKeeper.Logger(ctx).Error("bucket does not exist", "bucket id", mirrorBucketPackage.Id.String())
		return sdk.ExecuteResult{
			Err: types.ErrNoSuchBucket,
		}
	}

	bucketInfo.SourceType = types.SOURCE_TYPE_ORIGIN
	app.storageKeeper.SetBucketInfo(ctx, bucketInfo)

	if err := ctx.EventManager().EmitTypedEvents(&types.EventMirrorBucketResult{
		Status:      uint32(types.StatusFail),
		BucketName:  bucketInfo.BucketName,
		BucketId:    bucketInfo.Id,
		DestChainId: uint32(appCtx.SrcChainId),
	}); err != nil {
		return sdk.ExecuteResult{
			Err: err,
		}
	}

	return sdk.ExecuteResult{}
}

//nolint:unparam
func (app *BucketApp) handleMirrorBucketSynPackage(ctx sdk.Context, _ *sdk.CrossChainAppContext, _ *types.MirrorBucketSynPackage) sdk.ExecuteResult {
	app.storageKeeper.Logger(ctx).Error("received mirror bucket syn package ")

	return sdk.ExecuteResult{}
}

//nolint:unparam
func (app *BucketApp) handleCreateBucketAckPackage(ctx sdk.Context, _ *sdk.CrossChainAppContext, _ *types.CreateBucketAckPackage) sdk.ExecuteResult {
	app.storageKeeper.Logger(ctx).Error("received create bucket ack package ")

	return sdk.ExecuteResult{}
}

//nolint:unparam
func (app *BucketApp) handleCreateBucketFailAckPackage(ctx sdk.Context, _ *sdk.CrossChainAppContext, _ *types.CreateBucketSynPackage) sdk.ExecuteResult {
	app.storageKeeper.Logger(ctx).Error("received create bucket fail ack package ")

	return sdk.ExecuteResult{}
}

//nolint:unparam
func (app *BucketApp) handleCreateBucketFailAckPackageV2(ctx sdk.Context, _ *sdk.CrossChainAppContext, _ *types.CreateBucketSynPackageV2) sdk.ExecuteResult {
	app.storageKeeper.Logger(ctx).Error("received create bucket fail ack package ")

	return sdk.ExecuteResult{}
}

func (app *BucketApp) handleCreateBucketSynPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, createBucketPackage *types.CreateBucketSynPackage) sdk.ExecuteResult {
	err := createBucketPackage.ValidateBasic()
	if err != nil {
		return sdk.ExecuteResult{
			Payload: types.CreateBucketAckPackage{
				Status:    types.StatusFail,
				Creator:   createBucketPackage.Creator,
				ExtraData: createBucketPackage.ExtraData,
			}.MustSerialize(),
			Err: err,
		}
	}
	app.storageKeeper.Logger(ctx).Info("process create bucket syn package", "bucket name", createBucketPackage.BucketName)

	sourceType, err := app.storageKeeper.GetSourceTypeByChainId(ctx, appCtx.SrcChainId)
	if err != nil {
		return sdk.ExecuteResult{
			Err: err,
		}
	}

	bucketID, err := app.storageKeeper.CreateBucket(ctx,
		createBucketPackage.Creator,
		createBucketPackage.BucketName,
		createBucketPackage.PrimarySpAddress,
		&types.CreateBucketOptions{
			Visibility:       types.VisibilityType(createBucketPackage.Visibility),
			SourceType:       sourceType,
			ChargedReadQuota: createBucketPackage.ChargedReadQuota,
			PaymentAddress:   createBucketPackage.PaymentAddress.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight: createBucketPackage.PrimarySpApprovalExpiredHeight,
				Sig:           createBucketPackage.PrimarySpApprovalSignature,
			},
			ApprovalMsgBytes: createBucketPackage.GetApprovalBytes(),
		},
	)
	if err != nil {
		return sdk.ExecuteResult{
			Payload: types.CreateBucketAckPackage{
				Status:    types.StatusFail,
				Creator:   createBucketPackage.Creator,
				ExtraData: createBucketPackage.ExtraData,
			}.MustSerialize(),
			Err: err,
		}
	}

	return sdk.ExecuteResult{
		Payload: types.CreateBucketAckPackage{
			Status:    types.StatusSuccess,
			Id:        bucketID.BigInt(),
			Creator:   createBucketPackage.Creator,
			ExtraData: createBucketPackage.ExtraData,
		}.MustSerialize(),
	}
}

func (app *BucketApp) handleCreateBucketSynPackageV2(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, createBucketPackageV2 *types.CreateBucketSynPackageV2) sdk.ExecuteResult {
	err := createBucketPackageV2.ValidateBasic()
	if err != nil {
		return sdk.ExecuteResult{
			Payload: types.CreateBucketAckPackage{
				Status:    types.StatusFail,
				Creator:   createBucketPackageV2.Creator,
				ExtraData: createBucketPackageV2.ExtraData,
			}.MustSerialize(),
			Err: err,
		}
	}
	app.storageKeeper.Logger(ctx).Info("process create bucket syn package v2", "bucket name", createBucketPackageV2.BucketName)

	sourceType, err := app.storageKeeper.GetSourceTypeByChainId(ctx, appCtx.SrcChainId)
	if err != nil {
		return sdk.ExecuteResult{
			Err: err,
		}
	}

	bucketID, err := app.storageKeeper.CreateBucket(ctx,
		createBucketPackageV2.Creator,
		createBucketPackageV2.BucketName,
		createBucketPackageV2.PrimarySpAddress,
		&types.CreateBucketOptions{
			Visibility:       types.VisibilityType(createBucketPackageV2.Visibility),
			SourceType:       sourceType,
			ChargedReadQuota: createBucketPackageV2.ChargedReadQuota,
			PaymentAddress:   createBucketPackageV2.PaymentAddress.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              createBucketPackageV2.PrimarySpApprovalExpiredHeight,
				GlobalVirtualGroupFamilyId: createBucketPackageV2.GlobalVirtualGroupFamilyId,
				Sig:                        createBucketPackageV2.PrimarySpApprovalSignature,
			},
			ApprovalMsgBytes: createBucketPackageV2.GetApprovalBytes(),
		},
	)
	if err != nil {
		return sdk.ExecuteResult{
			Payload: types.CreateBucketAckPackage{
				Status:    types.StatusFail,
				Creator:   createBucketPackageV2.Creator,
				ExtraData: createBucketPackageV2.ExtraData,
			}.MustSerialize(),
			Err: err,
		}
	}

	return sdk.ExecuteResult{
		Payload: types.CreateBucketAckPackage{
			Status:    types.StatusSuccess,
			Id:        bucketID.BigInt(),
			Creator:   createBucketPackageV2.Creator,
			ExtraData: createBucketPackageV2.ExtraData,
		}.MustSerialize(),
	}
}

//nolint:unparam
func (app *BucketApp) handleDeleteBucketAckPackage(ctx sdk.Context, _ *sdk.CrossChainAppContext, _ *types.DeleteBucketAckPackage) sdk.ExecuteResult {
	app.storageKeeper.Logger(ctx).Error("received delete bucket ack package ")

	return sdk.ExecuteResult{}
}

//nolint:unparam
func (app *BucketApp) handleDeleteBucketFailAckPackage(ctx sdk.Context, _ *sdk.CrossChainAppContext, _ *types.DeleteBucketSynPackage) sdk.ExecuteResult {
	app.storageKeeper.Logger(ctx).Error("received delete bucket fail ack package ")

	return sdk.ExecuteResult{}
}

func (app *BucketApp) handleDeleteBucketSynPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, deleteBucketPackage *types.DeleteBucketSynPackage) sdk.ExecuteResult {
	err := deleteBucketPackage.ValidateBasic()
	if err != nil {
		return sdk.ExecuteResult{
			Payload: types.DeleteBucketAckPackage{
				Status:    types.StatusFail,
				Id:        deleteBucketPackage.Id,
				ExtraData: deleteBucketPackage.ExtraData,
			}.MustSerialize(),
			Err: err,
		}
	}

	app.storageKeeper.Logger(ctx).Info("process delete group syn package", "bucket id", deleteBucketPackage.Id.String())

	bucketInfo, found := app.storageKeeper.GetBucketInfoById(ctx, math.NewUintFromBigInt(deleteBucketPackage.Id))
	if !found {
		app.storageKeeper.Logger(ctx).Error("bucket does not exist", "bucket id", deleteBucketPackage.Id.String())
		return sdk.ExecuteResult{
			Payload: types.DeleteBucketAckPackage{
				Status:    types.StatusFail,
				Id:        deleteBucketPackage.Id,
				ExtraData: deleteBucketPackage.ExtraData,
			}.MustSerialize(),
			Err: types.ErrNoSuchBucket,
		}
	}

	sourceType, err := app.storageKeeper.GetSourceTypeByChainId(ctx, appCtx.SrcChainId)
	if err != nil {
		return sdk.ExecuteResult{
			Err: err,
		}
	}

	err = app.storageKeeper.DeleteBucket(ctx,
		deleteBucketPackage.Operator,
		bucketInfo.BucketName,
		types.DeleteBucketOptions{
			SourceType: sourceType,
		},
	)
	if err != nil {
		return sdk.ExecuteResult{
			Payload: types.DeleteBucketAckPackage{
				Status:    types.StatusFail,
				Id:        deleteBucketPackage.Id,
				ExtraData: deleteBucketPackage.ExtraData,
			}.MustSerialize(),
			Err: err,
		}
	}
	return sdk.ExecuteResult{
		Payload: types.DeleteBucketAckPackage{
			Status:    types.StatusSuccess,
			Id:        bucketInfo.Id.BigInt(),
			ExtraData: deleteBucketPackage.ExtraData,
		}.MustSerialize(),
	}
}
