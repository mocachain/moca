package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageBillDeleteBucketWithReadQuotaEvmFlow drives x/storage's bucket
// deletion billing through the storage precompile's deleteBucket method,
// exercising the same functional path as the retired suite's
// TestStorageBill_DeleteBucket_WithReadQuota: a charged read quota alone
// (no objects at all) already creates an ongoing billing rate, and deleting
// the bucket must fully unwind it back to exactly zero.
func TestStorageBillDeleteBucketWithReadQuotaEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	paymentClient := paymenttypes.NewQueryClient(conn)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PUBLIC_READ, 100)

	beforeDelete := getStreamRecord(t, ctx, paymentClient, ownerAddr.String())
	require.False(t, beforeDelete.NetflowRate.IsZero(), "a charged read quota alone should already create a billing rate")

	deleteMethod := storage.GetAbiMethod(storage.DeleteBucketMethodName)
	deleteArgs, err := deleteMethod.Inputs.Pack(bucketName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, deleteMethod.ID...), deleteArgs...))

	_, err = storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.Error(t, err, "deleted bucket should no longer exist")

	afterDelete := getStreamRecord(t, ctx, paymentClient, ownerAddr.String())
	require.True(t, afterDelete.NetflowRate.IsZero(), "deleting the bucket must fully unwind the read-quota-only rate back to zero")
}
