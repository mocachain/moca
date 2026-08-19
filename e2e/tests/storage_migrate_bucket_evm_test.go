package tests

import (
	"context"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestStorageMigrateBucketCancelEvmFlow drives x/storage's bucket-migration
// request through the storage precompile's migrateBucket and
// cancelMigrateBucket methods: the owner requests migration to a second SP
// (signed the same Keccak256(GetApprovalBytes()) way as
// createBucket/copyObject), the bucket flips to BUCKET_STATUS_MIGRATING,
// and canceling reverts it. completeMigrateBucket needs a BLS-signed GVG
// mapping confirmation from the destination's secondary SPs (the same
// infrastructure gap that blocks sealObject for non-trivial objects, see
// TestStorageCancelUpdateObjectContentEvmFlow) so it's out of scope here.
func TestStorageMigrateBucketCancelEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	spClient := sptypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)
	sp1Export, ok := loadSPExport(t)["sp1"]
	require.True(t, ok)
	sp1ApprovalKey := mustHexKey(t, sp1Export.ApprovalPrivateKey)

	spsResp, err := spClient.StorageProviders(ctx, &sptypes.QueryStorageProvidersRequest{Pagination: &query.PageRequest{Limit: math.MaxUint64}})
	require.NoError(t, err)
	var dstSPID uint32
	for _, s := range spsResp.Sps {
		if strings.EqualFold(s.OperatorAddress, sp1Export.OperatorAddress) {
			dstSPID = s.Id
		}
	}
	require.NotZero(t, dstSPID, "sp1 not found on-chain by operator address")

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	// A non-zero charged read quota is required: migrateBucket checks the
	// bucket's own payment-address stream record isn't frozen, and a
	// zero-quota bucket never gets a stream record at all (treated the same
	// as frozen).
	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 100)

	fakeMsg := storagetypes.NewMsgMigrateBucket(ownerAddr.Bytes(), bucketName, dstSPID)
	fakeMsg.DstPrimarySpApproval.ExpiredHeight = math.MaxUint
	approvalDigest := crypto.Keccak256(fakeMsg.GetApprovalBytes())
	approvalSig, err := crypto.Sign(approvalDigest, sp1ApprovalKey)
	require.NoError(t, err)

	migrateMethod := storage.GetAbiMethod(storage.MigrateBucketMethodName)
	migrateArgs, err := migrateMethod.Inputs.Pack(
		bucketName, dstSPID,
		storage.Approval{ExpiredHeight: math.MaxUint, GlobalVirtualGroupFamilyId: 0, Sig: approvalSig},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, migrateMethod.ID...), migrateArgs...))

	during, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.BUCKET_STATUS_MIGRATING, during.BucketInfo.BucketStatus)

	cancelMethod := storage.GetAbiMethod(storage.CancelMigrateBucketMethodName)
	cancelArgs, err := cancelMethod.Inputs.Pack(bucketName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, cancelMethod.ID...), cancelArgs...))

	after, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.BUCKET_STATUS_CREATED, after.BucketInfo.BucketStatus, "canceling must revert the bucket back out of migrating status")
}

// TestStorageMigrateBucketRejectEvmFlow mirrors
// TestStorageMigrateBucketCancelEvmFlow but resolves the pending migration
// through the storage precompile's rejectMigrateBucket method instead: only
// the destination SP's own operator address (not the bucket owner, not any
// other SP) may reject a migration targeting it.
func TestStorageMigrateBucketRejectEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	spClient := sptypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)
	sp1Export, ok := loadSPExport(t)["sp1"]
	require.True(t, ok)
	sp1OperatorKey := mustHexKey(t, sp1Export.OperatorPrivateKey)
	sp1ApprovalKey := mustHexKey(t, sp1Export.ApprovalPrivateKey)
	fundMoca(t, ctx, client, chainID, crypto.PubkeyToAddress(sp1OperatorKey.PublicKey), fundingAmountMOCA)

	spsResp, err := spClient.StorageProviders(ctx, &sptypes.QueryStorageProvidersRequest{Pagination: &query.PageRequest{Limit: math.MaxUint64}})
	require.NoError(t, err)
	var dstSPID uint32
	for _, s := range spsResp.Sps {
		if strings.EqualFold(s.OperatorAddress, sp1Export.OperatorAddress) {
			dstSPID = s.Id
		}
	}
	require.NotZero(t, dstSPID, "sp1 not found on-chain by operator address")

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 100)

	fakeMsg := storagetypes.NewMsgMigrateBucket(ownerAddr.Bytes(), bucketName, dstSPID)
	fakeMsg.DstPrimarySpApproval.ExpiredHeight = math.MaxUint
	approvalDigest := crypto.Keccak256(fakeMsg.GetApprovalBytes())
	approvalSig, err := crypto.Sign(approvalDigest, sp1ApprovalKey)
	require.NoError(t, err)

	migrateMethod := storage.GetAbiMethod(storage.MigrateBucketMethodName)
	migrateArgs, err := migrateMethod.Inputs.Pack(
		bucketName, dstSPID,
		storage.Approval{ExpiredHeight: math.MaxUint, GlobalVirtualGroupFamilyId: 0, Sig: approvalSig},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, migrateMethod.ID...), migrateArgs...))

	rejectMethod := storage.GetAbiMethod(storage.RejectMigrateBucketMethodName)
	rejectArgs, err := rejectMethod.Inputs.Pack(bucketName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, sp1OperatorKey, precompile.Address(),
		append(append([]byte{}, rejectMethod.ID...), rejectArgs...))

	after, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.BUCKET_STATUS_CREATED, after.BucketInfo.BucketStatus, "the destination SP rejecting must revert the bucket back out of migrating status")
}
