package tests

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// threeChecksums returns 3 raw checksums (1 primary + 2 secondary, matching
// this genesis's EC scheme), and their base64 encoding for the ABI call --
// the same bytes must be used to compute the SP approval signature and to
// submit createObject, since GetSignBytes hashes the whole message.
func threeChecksums() (raw [][]byte, b64 []string) {
	raw = make([][]byte, 3)
	b64 = make([]string, 3)
	for i := range raw {
		sum := sha256.Sum256([]byte{byte(i)})
		raw[i] = sum[:]
		b64[i] = base64.StdEncoding.EncodeToString(sum[:])
	}
	return raw, b64
}

// TestStorageBillCancelCreateObjectEvmFlow drives x/storage's create/cancel
// object lock-fee accounting through the storage precompile's createObject
// and cancelCreateObject methods, exercising the same functional path as the
// retired suite's TestStorageBill_CancelCreateObject: canceling a pending
// (never-sealed) object creation is a full, penalty-free refund -- the
// locked fee returns exactly to StaticBalance and no netflow rate changes.
func TestStorageBillCancelCreateObjectEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	paymentClient := paymenttypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	baseline := getStreamRecord(t, ctx, paymentClient, ownerAddr.String())

	// Create an object -- this locks a fee but starts no stream (unsealed).
	objectName := storageutils.GenRandomObjectName()
	rawChecksums, b64Checksums := threeChecksums()
	fakeMsg := storagetypes.NewMsgCreateObject(
		ownerAddr.Bytes(), bucketName, objectName, 1024*1024, storagetypes.VISIBILITY_TYPE_INHERIT,
		rawChecksums, "application/octet-stream", storagetypes.REDUNDANCY_EC_TYPE, 0, nil,
	)
	approvalDigest := crypto.Keccak256(fakeMsg.GetApprovalBytes())
	approvalSig, err := crypto.Sign(approvalDigest, sp.ApprovalKey)
	require.NoError(t, err)

	createMethod := storage.GetAbiMethod(storage.CreateObjectMethodName)
	createArgs, err := createMethod.Inputs.Pack(
		bucketName,
		objectName,
		uint64(1024*1024),
		uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		"application/octet-stream",
		storage.Approval{ExpiredHeight: 0, GlobalVirtualGroupFamilyId: 0, Sig: approvalSig},
		b64Checksums,
		uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, createMethod.ID...), createArgs...))

	_, err = storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.NoError(t, err)

	afterCreate := getStreamRecord(t, ctx, paymentClient, ownerAddr.String())
	require.True(t, afterCreate.NetflowRate.Equal(baseline.NetflowRate), "an unsealed create must not start any stream")
	lockedFee := afterCreate.LockBalance.Sub(baseline.LockBalance)
	require.True(t, lockedFee.IsPositive(), "creating an object should lock a positive fee")

	// Cancel the pending creation.
	cancelMethod := storage.GetAbiMethod(storage.CancelCreateObjectMethodName)
	cancelArgs, err := cancelMethod.Inputs.Pack(bucketName, objectName)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, cancelMethod.ID...), cancelArgs...))

	_, err = storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	require.Error(t, err, "canceled object should no longer exist")

	afterCancel := getStreamRecord(t, ctx, paymentClient, ownerAddr.String())
	require.True(t, afterCancel.LockBalance.Equal(baseline.LockBalance), "lock balance must return to baseline")
	require.True(t, afterCancel.NetflowRate.Equal(baseline.NetflowRate), "rates must be unaffected throughout")
	require.True(t, afterCancel.StaticBalance.Sub(baseline.StaticBalance).Equal(lockedFee),
		"the exact locked fee should be refunded to static balance, no penalty")
}
