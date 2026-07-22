package tests

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageCopyObjectEvmFlow drives x/storage's object copy through the
// storage precompile's copyObject method: the destination bucket's primary
// SP signs an approval (the same Keccak256(GetApprovalBytes()) pattern used
// by createBucket/createObject) and the owner copies an existing object
// into a second, already-existing bucket.
func TestStorageCopyObjectEvmFlow(t *testing.T) {
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

	srcBucket := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)
	dstBucket := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	srcObjectName := storageutils.GenRandomObjectName()
	_, b64Checksums := threeChecksums()
	delegateMethod := storage.GetAbiMethod(storage.DelegateCreateObjectMethodName)
	delegateArgs, err := delegateMethod.Inputs.Pack(
		ownerAddr.String(), srcBucket, srcObjectName,
		uint64(0), "application/octet-stream", uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		b64Checksums, uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, precompile.Address(),
		append(append([]byte{}, delegateMethod.ID...), delegateArgs...))

	dstObjectName := storageutils.GenRandomObjectName()
	fakeMsg := storagetypes.NewMsgCopyObject(ownerAddr.Bytes(), srcBucket, dstBucket, srcObjectName, dstObjectName, math.MaxUint, nil)
	approvalDigest := crypto.Keccak256(fakeMsg.GetApprovalBytes())
	approvalSig, err := crypto.Sign(approvalDigest, sp.ApprovalKey)
	require.NoError(t, err)

	copyMethod := storage.GetAbiMethod(storage.CopyObjectMethodName)
	copyArgs, err := copyMethod.Inputs.Pack(
		srcBucket, dstBucket, srcObjectName, dstObjectName,
		storage.Approval{ExpiredHeight: math.MaxUint, GlobalVirtualGroupFamilyId: 0, Sig: approvalSig},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, copyMethod.ID...), copyArgs...))

	headResp, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: dstBucket, ObjectName: dstObjectName})
	require.NoError(t, err)
	require.Equal(t, ownerAddr.String(), headResp.ObjectInfo.Owner)
}
