package upgrades

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// ReassignGovernancePayerBuckets moves the payer of every bucket that still
// names the payment governance account as its payment address over to the
// bucket's own owner: the governance account never pays, and every rate
// change that would make it do so is now rejected, but that guard only
// covers assignments made from here on. Bucket creation never checked
// payment-account ownership before this release, so live state can already
// hold buckets whose payer is the governance account.
//
// A sealed object's charge is already folded into the bucket's ongoing rate
// (InternalBucketInfo.TotalChargeSize), which UpdateBucketInfoAndCharge moves
// to the new payer as part of the payer switch below, so it needs no
// per-object handling. An unsealed object is different: its fee is locked
// against the bucket's CURRENT payer (see lockObjectStoreFee in
// x/storage/keeper/payment.go), and unlocking always reads the bucket's
// payment address at the time of the call, so unlocking it after the payer
// switch would credit the new payer for a lock the old one funded. Each
// unsealed object is therefore canceled first, while the payer is still the
// governance account, through the same path a user canceling their own
// upload takes (CancelCreateObject), with the bucket owner as operator: an
// object's Owner is always set to its bucket's Owner at creation
// (CreateObject, CopyObject) and never changes afterward -- bucket ownership
// itself is immutable, the bucket NFT contract is non-transferable -- so
// VerifyObjectPermission's owner clause always grants the bucket owner
// ACTION_DELETE_OBJECT on every object in it.
//
// The walk runs under the forced stream-record context, the same one an
// end-of-block settlement uses: a new payer that cannot afford its share is
// force-settled and frozen by the existing mechanism rather than aborting
// the reassignment.
//
// Bucket ids, and each bucket's unsealed object names, are collected before
// any of them are mutated, so neither walk depends on iteration order.
// Returns the number of buckets reassigned and objects canceled.
func ReassignGovernancePayerBuckets(ctx sdk.Context, k storagekeeper.Keeper) (reassignedCount, canceledCount int, err error) {
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

		if k.GetLockedObjectCount(ctx, bucketInfo.Id) != 0 {
			type unsealedObject struct {
				name       string
				sourceType storagetypes.SourceType
			}
			var unsealed []unsealedObject
			k.IterateBucketObjects(ctx, bucketInfo.BucketName, func(objectInfo storagetypes.ObjectInfo) bool {
				if objectInfo.ObjectStatus == storagetypes.OBJECT_STATUS_CREATED {
					unsealed = append(unsealed, unsealedObject{objectInfo.ObjectName, objectInfo.SourceType})
				}
				return false
			})

			for _, obj := range unsealed {
				cancelErr := k.CancelCreateObject(forcedCtx, owner, bucketInfo.BucketName, obj.name,
					storagetypes.CancelCreateObjectOptions{SourceType: obj.sourceType})
				if cancelErr != nil {
					return reassignedCount, canceledCount, fmt.Errorf(
						"cancel unsealed object %s in bucket %s: %w", obj.name, bucketInfo.BucketName, cancelErr)
				}
				canceledCount++
			}
		}

		internalBucketInfo, found := k.GetInternalBucketInfo(ctx, bucketInfo.Id)
		if !found {
			// Every bucket is created together with its internal record, so this
			// should not happen; skip and log rather than aborting the rest of
			// the upgrade over one bucket's inconsistent state.
			ctx.Logger().Error("storage: left a governance-payer bucket without internal bucket info for a later pass",
				"bucket_id", bucketInfo.Id.String(), "bucket_name", bucketInfo.BucketName)
			continue
		}

		if updateErr := k.UpdateBucketInfoAndCharge(forcedCtx, bucketInfo, internalBucketInfo, owner.String(), bucketInfo.ChargedReadQuota); updateErr != nil {
			return reassignedCount, canceledCount, fmt.Errorf(
				"reassign governance payer for bucket %s: %w", bucketInfo.Id.String(), updateErr)
		}
		k.StoreBucketInfo(ctx, bucketInfo)
		k.SetInternalBucketInfo(ctx, bucketInfo.Id, internalBucketInfo)
		reassignedCount++
	}

	ctx.Logger().Info("storage: reassigned governance-payer buckets to their owners",
		"reassigned", reassignedCount, "objects_canceled", canceledCount)
	return reassignedCount, canceledCount, nil
}
