package keeper

import (
	// "encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v12/x/storage/types"
)

var _ sdk.CrossChainApplication = &MocaVCApp{}

type MocaVCApp struct {
	storageKeeper types.StorageKeeper
}

func NewMocaVCApp(keeper types.StorageKeeper) *MocaVCApp {
	return &MocaVCApp{
		storageKeeper: keeper,
	}
}

func (app *MocaVCApp) ExecuteAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	// pack, err := types.DeserializeCrossChainPackage(payload, types.MocaVCChannelId, sdk.AckCrossChainPackageType)
	// if err != nil {
	// 	app.storageKeeper.Logger(ctx).Error("deserialize mocavc cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
	// 	panic("deserialize mocavc cross chain package error")
	// }

	// var operationType uint8
	// var result sdk.ExecuteResult
	// switch p := pack.(type) {
	// case *types.MocaVCAckPackage:
	// 	operationType = types.OperationMirrorGroup
	// 	result = app.handleMocaVCAckPackage(ctx, appCtx, p)
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

func (app *MocaVCApp) ExecuteFailAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	// pack, err := types.DeserializeCrossChainPackage(payload, types.MocaVCChannelId, sdk.FailAckCrossChainPackageType)
	// if err != nil {
	// 	app.storageKeeper.Logger(ctx).Error("deserialize mocavc cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
	// 	panic("deserialize mocavc cross chain package error")
	// }

	// var operationType uint8
	// var result sdk.ExecuteResult
	// switch p := pack.(type) {
	// case *types.MocaVCSynPackage:
	// 	operationType = types.OperationMirrorGroup
	// 	result = app.handleMocaVCFailAckPackage(ctx, appCtx, p)
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

func (app *MocaVCApp) ExecuteSynPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	var result sdk.ExecuteResult
	return result
}
