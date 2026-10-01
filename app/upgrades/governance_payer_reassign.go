package upgrades

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// ReassignGovernancePayerBuckets cancels every create/update still locked against a governance-payer
// bucket's objects and moves the bucket's payer to its owner; sealed charges move with the switch.
func ReassignGovernancePayerBuckets(ctx sdk.Context, k storagekeeper.Keeper) (reassignedCount, canceledCreateCount, canceledUpdateCount int, err error) {
	var candidateIDs []sdkmath.Uint
	k.IterateBucketInfos(ctx, func(bucketInfo storagetypes.BucketInfo) bool {
		payer, addrErr := sdk.AccAddressFromHexUnsafe(bucketInfo.PaymentAddress)
		if addrErr == nil && payer.Equals(paymenttypes.GovernanceAddress) {
			candidateIDs = append(candidateIDs, bucketInfo.Id)
		}
		return false
	})

	forcedCtx := ctx.WithValue(paymenttypes.ForceUpdateStreamRecordKey, true)

	for _, bucketID := range candidateIDs {
		bucketInfo, found := k.GetBucketInfoById(ctx, bucketID)
		if !found {
			continue // gone by the time it is revisited; nothing to reassign
		}
		owner := sdk.MustAccAddressFromHex(bucketInfo.Owner)

		// The locked-object counter can read zero on state older than the counter
		// itself, so every bucket is scanned regardless of its count.
		type unsealedObject struct {
			name       string
			sourceType storagetypes.SourceType
		}
		var unsealed []unsealedObject
		var updating []string
		k.IterateBucketObjects(ctx, bucketInfo.BucketName, func(objectInfo storagetypes.ObjectInfo) bool {
			switch {
			case objectInfo.ObjectStatus == storagetypes.OBJECT_STATUS_CREATED:
				unsealed = append(unsealed, unsealedObject{objectInfo.ObjectName, objectInfo.SourceType})
			case objectInfo.ObjectStatus == storagetypes.OBJECT_STATUS_SEALED && objectInfo.IsUpdating:
				updating = append(updating, objectInfo.ObjectName)
			}
			return false
		})

		for _, obj := range unsealed {
			cancelErr := k.CancelCreateObject(forcedCtx, owner, bucketInfo.BucketName, obj.name,
				storagetypes.CancelCreateObjectOptions{SourceType: obj.sourceType})
			if cancelErr != nil {
				return reassignedCount, canceledCreateCount, canceledUpdateCount, fmt.Errorf(
					"cancel unsealed object %s in bucket %s: %w", obj.name, bucketInfo.BucketName, cancelErr)
			}
			canceledCreateCount++
		}

		for _, name := range updating {
			cancelErr := k.CancelUpdateObjectContent(forcedCtx, owner, bucketInfo.BucketName, name)
			if cancelErr != nil {
				return reassignedCount, canceledCreateCount, canceledUpdateCount, fmt.Errorf(
					"cancel in-progress update %s in bucket %s: %w", name, bucketInfo.BucketName, cancelErr)
			}
			canceledUpdateCount++
		}

		internalBucketInfo, found := k.GetInternalBucketInfo(ctx, bucketInfo.Id)
		if !found {
			// A bucket is always created with its internal record; skip and log
			// rather than abort the upgrade over one inconsistent bucket.
			ctx.Logger().Error("storage: left a governance-payer bucket without internal bucket info for a later pass",
				"bucket_id", bucketInfo.Id.String(), "bucket_name", bucketInfo.BucketName)
			continue
		}

		if updateErr := k.UpdateBucketInfoAndCharge(forcedCtx, bucketInfo, internalBucketInfo, owner.String(), bucketInfo.ChargedReadQuota); updateErr != nil {
			return reassignedCount, canceledCreateCount, canceledUpdateCount, fmt.Errorf(
				"reassign governance payer for bucket %s: %w", bucketInfo.Id.String(), updateErr)
		}
		k.StoreBucketInfo(ctx, bucketInfo)
		k.SetInternalBucketInfo(ctx, bucketInfo.Id, internalBucketInfo)
		reassignedCount++
	}

	ctx.Logger().Info("storage: reassigned governance-payer buckets to their owners",
		"reassigned", reassignedCount, "creates_canceled", canceledCreateCount, "updates_canceled", canceledUpdateCount)
	return reassignedCount, canceledCreateCount, canceledUpdateCount, nil
}
