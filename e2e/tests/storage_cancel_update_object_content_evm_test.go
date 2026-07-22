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

// TestStorageCancelUpdateObjectContentEvmFlow drives x/storage's
// update-then-cancel object-content flow through the storage precompile's
// updateObjectContent and cancelUpdateObjectContent methods: updating an
// already-sealed object's content only *starts* a pending update (the
// object's IsUpdating flag flips true, actual content swap requires a
// BLS-signed reseal from the secondary SPs, out of scope for a live test --
// see TestStorageDelegateCreateObjectEvmFlow's package doc), and canceling
// that pending update reverts IsUpdating back to false with no content
// change, mirroring TestStorageBillCancelCreateObjectEvmFlow's
// create/cancel shape one level further into the object's lifecycle.
func TestStorageCancelUpdateObjectContentEvmFlow(t *testing.T) {
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
	createMethod := storage.GetAbiMethod(storage.DelegateCreateObjectMethodName)
	createArgs, err := createMethod.Inputs.Pack(
		ownerAddr.String(), bucketName, objectName,
		uint64(0), "application/octet-stream", uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		b64Checksums, uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, precompile.Address(),
		append(append([]byte{}, createMethod.ID...), createArgs...))

	before, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.OBJECT_STATUS_SEALED, before.ObjectInfo.ObjectStatus, "updateObjectContent requires an already-sealed object")

	_, newB64Checksums := threeChecksums()
	updateContentMethod := storage.GetAbiMethod(storage.UpdateObjectContentMethodName)
	updateContentArgs, err := updateContentMethod.Inputs.Pack(bucketName, objectName, uint64(2048), "application/octet-stream", newB64Checksums)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, updateContentMethod.ID...), updateContentArgs...))

	during, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.True(t, during.ObjectInfo.IsUpdating, "updateObjectContent should start a pending update, not apply it immediately")
	require.Zero(t, during.ObjectInfo.PayloadSize, "the visible payload size must not change until the update is sealed")

	cancelMethod := storage.GetAbiMethod(storage.CancelUpdateObjectContentMethodName)
	cancelArgs, err := cancelMethod.Inputs.Pack(bucketName, objectName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, cancelMethod.ID...), cancelArgs...))

	after, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.False(t, after.ObjectInfo.IsUpdating, "cancelUpdateObjectContent must revert the pending-update flag")
	require.Equal(t, storagetypes.OBJECT_STATUS_SEALED, after.ObjectInfo.ObjectStatus)
}
