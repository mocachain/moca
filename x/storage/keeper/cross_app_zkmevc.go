package keeper

import (
	// "encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v12/x/storage/types"
)

var _ sdk.CrossChainApplication = &ZkmeVCApp{}

type ZkmeVCApp struct {
	storageKeeper types.StorageKeeper
}

func NewZkmeVCApp(keeper types.StorageKeeper) *ZkmeVCApp {
	return &ZkmeVCApp{
		storageKeeper: keeper,
	}
}

func (app *ZkmeVCApp) ExecuteAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	// pack, err := types.DeserializeCrossChainPackage(payload, types.ZkmeVCChannelId, sdk.AckCrossChainPackageType)
	// if err != nil {
	// 	app.storageKeeper.Logger(ctx).Error("deserialize zkmevc cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
	// 	panic("deserialize zkmevc cross chain package error")
	// }

	// var operationType uint8
	// var result sdk.ExecuteResult
	// switch p := pack.(type) {
	// case *types.ZkmeVCAckPackage:
	// 	operationType = types.OperationMirrorGroup
	// 	result = app.handleZkmeVCAckPackage(ctx, appCtx, p)
	// default:
	// 	panic("unknown cross chain ack package type")
	// }

	// if len(result.Payload) != 0 {
	// 	wrapPayload := types.CrossChainPackage{
	// 		OperationType: operationType,
	// 		Package:       result.Payload,
	// 	}
	// 	result.Payload = wrapPayload.MustSerialize()
	// }

	var result sdk.ExecuteResult
	return result
}

func (app *ZkmeVCApp) ExecuteFailAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	// pack, err := types.DeserializeCrossChainPackage(payload, types.ZkmeVCChannelId, sdk.FailAckCrossChainPackageType)
	// if err != nil {
	// 	app.storageKeeper.Logger(ctx).Error("deserialize zkmevc cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
	// 	panic("deserialize zkmevc cross chain package error")
	// }

	// var operationType uint8
	// var result sdk.ExecuteResult
	// switch p := pack.(type) {
	// case *types.ZkmeVCSynPackage:
	// 	operationType = types.OperationMirrorGroup
	// 	result = app.handleZkmeVCFailAckPackage(ctx, appCtx, p)
	// default:
	// 	panic("unknown cross chain ack package type")
	// }

	// if len(result.Payload) != 0 {
	// 	wrapPayload := types.CrossChainPackage{
	// 		OperationType: operationType,
	// 		Package:       result.Payload,
	// 	}
	// 	result.Payload = wrapPayload.MustSerialize()
	// }

	var result sdk.ExecuteResult
	return result
}

func (app *ZkmeVCApp) ExecuteSynPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	var result sdk.ExecuteResult
	return result
}
