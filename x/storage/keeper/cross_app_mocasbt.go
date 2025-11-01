package keeper

import (
	// "encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/evmos/evmos/v12/x/storage/types"
)

var _ sdk.CrossChainApplication = &MocaSBTApp{}

type MocaSBTApp struct {
	storageKeeper types.StorageKeeper
}

func NewMocaSBTApp(keeper types.StorageKeeper) *MocaSBTApp {
	return &MocaSBTApp{
		storageKeeper: keeper,
	}
}

func (app *MocaSBTApp) ExecuteAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	// pack, err := types.DeserializeCrossChainPackage(payload, types.MocaSBTChannelId, sdk.AckCrossChainPackageType)
	// if err != nil {
	// 	app.storageKeeper.Logger(ctx).Error("deserialize mocasbt cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
	// 	panic("deserialize mocasbt cross chain package error")
	// }

	// var operationType uint8
	// var result sdk.ExecuteResult
	// switch p := pack.(type) {
	// case *types.MocaSBTAckPackage:
	// 	operationType = types.OperationMirrorGroup
	// 	result = app.handleMocaSBTAckPackage(ctx, appCtx, p)
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

func (app *MocaSBTApp) ExecuteFailAckPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	// pack, err := types.DeserializeCrossChainPackage(payload, types.MocaSBTChannelId, sdk.FailAckCrossChainPackageType)
	// if err != nil {
	// 	app.storageKeeper.Logger(ctx).Error("deserialize mocasbt cross chain package error", "payload", hex.EncodeToString(payload), "error", err.Error())
	// 	panic("deserialize mocasbt cross chain package error")
	// }

	// var operationType uint8
	// var result sdk.ExecuteResult
	// switch p := pack.(type) {
	// case *types.MocaSBTSynPackage:
	// 	operationType = types.OperationMirrorGroup
	// 	result = app.handleMocaSBTFailAckPackage(ctx, appCtx, p)
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

func (app *MocaSBTApp) ExecuteSynPackage(ctx sdk.Context, appCtx *sdk.CrossChainAppContext, payload []byte) sdk.ExecuteResult {
	var result sdk.ExecuteResult
	return result
}
