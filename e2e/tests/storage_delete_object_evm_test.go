package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageDeleteObjectEvmFlow drives x/storage's object deletion through
// the storage precompile's deleteObject method: the bucket owner deletes an
// existing (empty, already-sealed) object.
func TestStorageDeleteObjectEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	fundMoca(t, ctx, client, chainID, crypto.PubkeyToAddress(ownerKey.PublicKey), fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	objectName := storageutils.GenRandomObjectName()
	_, b64Checksums := threeChecksums()
	delegateMethod := storage.GetAbiMethod(storage.DelegateCreateObjectMethodName)
	delegateArgs, err := delegateMethod.Inputs.Pack(
		crypto.PubkeyToAddress(ownerKey.PublicKey).String(), bucketName, objectName,
		uint64(0), "application/octet-stream", uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		b64Checksums, uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, precompile.Address(),
		append(append([]byte{}, delegateMethod.ID...), delegateArgs...))

	_, err = storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err, "object should exist right after creation")

	deleteMethod := storage.GetAbiMethod(storage.DeleteObjectMethodName)
	deleteArgs, err := deleteMethod.Inputs.Pack(bucketName, objectName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, deleteMethod.ID...), deleteArgs...))

	_, err = storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.Error(t, err, "deleted object should no longer exist")
}
