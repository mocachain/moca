package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storage"
	mocatypes "github.com/mocachain/moca/v2/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

// TestPermissionAccountGrantEvmFlow drives x/permission's account-principal
// grant through the storage precompile's putPolicy method, exercising the
// full flow of the retired suite's TestDeleteBucketPermission -- the
// canonical grant/allow pattern the rest of the permission suite builds on --
// including the grantee actually exercising the grant with deleteBucket.
func TestPermissionAccountGrantEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	precompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(ctx, t, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(ctx, t, client, chainID, ownerAddr, fundingAmountMOCA)

	granteeKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	granteeAddr := crypto.PubkeyToAddress(granteeKey.PublicKey)
	fundMoca(ctx, t, client, chainID, granteeAddr, fundingAmountMOCA)

	bucketName := createTestBucket(ctx, t, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PUBLIC_READ, 0)

	verify := func(operator common.Address) permtypes.Effect {
		resp, err := storageClient.VerifyPermission(ctx, &storagetypes.QueryVerifyPermissionRequest{
			Operator:   operator.String(),
			BucketName: bucketName,
			ActionType: permtypes.ACTION_DELETE_BUCKET,
		})
		require.NoError(t, err)
		return resp.Effect
	}

	// Baseline: no grant, default deny.
	require.Equal(t, permtypes.EFFECT_DENY, verify(granteeAddr))

	// Owner grants the grantee ACTION_DELETE_BUCKET directly (account principal).
	putPolicyMethod := storage.GetAbiMethod(storage.PutPolicyMethodName)
	putPolicyArgs, err := putPolicyMethod.Inputs.Pack(
		storage.Principal{
			PrincipalType: int32(permtypes.PRINCIPAL_TYPE_GNFD_ACCOUNT),
			Value:         granteeAddr.String(),
		},
		mocatypes.NewBucketGRN(bucketName).String(),
		[]storage.Statement{{
			Effect:         int32(permtypes.EFFECT_ALLOW),
			Actions:        []int32{int32(permtypes.ACTION_DELETE_BUCKET)},
			Resources:      nil,
			ExpirationTime: 0,
			LimitSize:      0,
		}},
		int64(0),
	)
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, putPolicyMethod.ID...), putPolicyArgs...))

	require.Equal(t, permtypes.EFFECT_ALLOW, verify(granteeAddr), "grantee should have delete rights once granted")

	// The grantee exercises the grant: deleteBucket signed by the grantee, not
	// the owner. This is the path that faulted before #408 (the delete-side
	// policy sweep lived in a transient store the precompile multi-store
	// doesn't carry), so it doubles as that fix's regression test.
	deleteMethod := storage.GetAbiMethod(storage.DeleteBucketMethodName)
	deleteArgs, err := deleteMethod.Inputs.Pack(bucketName)
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, granteeKey, precompile.Address(),
		append(append([]byte{}, deleteMethod.ID...), deleteArgs...))

	_, err = storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.Error(t, err, "bucket should be gone after the grantee's deleteBucket")
}
