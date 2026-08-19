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

// TestStorageDiscontinueObjectEvmFlow drives x/storage's discontinue-object
// flow through the storage precompile's discontinueObject method: only the
// bucket's primary SP (by GC address) may discontinue one of its objects,
// identified by object ID (not name), flipping it to
// OBJECT_STATUS_DISCONTINUED immediately -- the same shape as
// TestStorageDiscontinueBucketEvmFlow, one level down the resource tree.
func TestStorageDiscontinueObjectEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)
	sp0Export, ok := loadSPExport(t)["sp0"]
	require.True(t, ok)
	gcKey := mustHexKey(t, sp0Export.GcPrivateKey)
	fundMoca(t, ctx, client, chainID, crypto.PubkeyToAddress(gcKey.PublicKey), fundingAmountMOCA)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	objectName := storageutils.GenRandomObjectName()
	_, b64Checksums := threeChecksums()
	createMethod := storage.GetAbiMethod(storage.DelegateCreateObjectMethodName)
	createArgs, err := createMethod.Inputs.Pack(
		ownerAddr.String(), bucketName, objectName,
		uint64(0), "application/octet-stream", uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		b64Checksums, uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, precompile.Address(),
		append(append([]byte{}, createMethod.ID...), createArgs...))

	headResp, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	objectID := headResp.ObjectInfo.Id.BigInt()

	discontinueMethod := storage.GetAbiMethod(storage.DiscontinueObjectMethodName)
	discontinueArgs, err := discontinueMethod.Inputs.Pack(bucketName, []*big.Int{objectID}, "policy violation")
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, gcKey, precompile.Address(),
		append(append([]byte{}, discontinueMethod.ID...), discontinueArgs...))

	after, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.OBJECT_STATUS_DISCONTINUED, after.ObjectInfo.ObjectStatus)
}
