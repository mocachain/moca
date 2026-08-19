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

// TestStorageUpdateObjectInfoEvmFlow drives x/storage's object visibility
// update through the storage precompile's updateObjectInfo method: the
// owner flips an existing object from PRIVATE to PUBLIC_READ.
func TestStorageUpdateObjectInfoEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	objectName := storageutils.GenRandomObjectName()
	_, b64Checksums := threeChecksums()
	delegateMethod := storage.GetAbiMethod(storage.DelegateCreateObjectMethodName)
	delegateArgs, err := delegateMethod.Inputs.Pack(
		ownerAddr.String(), bucketName, objectName,
		uint64(0), "application/octet-stream", uint8(storagetypes.VISIBILITY_TYPE_PRIVATE),
		b64Checksums, uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, precompile.Address(),
		append(append([]byte{}, delegateMethod.ID...), delegateArgs...))

	before, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.VISIBILITY_TYPE_PRIVATE, before.ObjectInfo.Visibility)

	updateMethod := storage.GetAbiMethod(storage.UpdateObjectInfoMethodName)
	updateArgs, err := updateMethod.Inputs.Pack(bucketName, objectName, uint8(storagetypes.VISIBILITY_TYPE_PUBLIC_READ))
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, updateMethod.ID...), updateArgs...))

	after, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.VISIBILITY_TYPE_PUBLIC_READ, after.ObjectInfo.Visibility)
}
