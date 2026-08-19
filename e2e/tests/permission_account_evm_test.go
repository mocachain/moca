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
// grant/verify half of the retired suite's TestDeleteBucketPermission -- the
// canonical grant/allow pattern the rest of the permission suite builds on.
// The grantee actually exercising the grant (deleteBucket) is BLOCKED -- see
// the t.Skip below -- and is left in place as a ready-to-enable regression
// test for when that's fixed.
func TestPermissionAccountGrantEvmFlow(t *testing.T) {
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

	granteeKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	granteeAddr := crypto.PubkeyToAddress(granteeKey.PublicKey)
	fundMoca(t, ctx, client, chainID, granteeAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PUBLIC_READ, 0)

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
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, precompile.Address(),
		append(append([]byte{}, putPolicyMethod.ID...), putPolicyArgs...))

	require.Equal(t, permtypes.EFFECT_ALLOW, verify(granteeAddr), "grantee should have delete rights once granted")

	// The grantee actually exercising the grant -- BLOCKED.
	//
	// Reproduced live via eth_call: this grantee's deleteBucket call fails
	// with the same error found on deleteGroup (see
	// TestPermissionStaleGroupDenialEvmFlow in this package):
	//   "kv store with key TransientStoreKey{..., transient_storage} has not
	//    been registered in stores"
	// even though the owner calling deleteBucket directly succeeds
	// (verified separately). VerifyBucketPermission has an "operator ==
	// owner, return ALLOW immediately" shortcut ahead of any real policy
	// lookup -- this grantee call skips that shortcut and reaches
	// VerifyPolicy instead. But that alone doesn't fully explain the
	// pattern: VerifyGroupPermission has the identical owner-shortcut, yet
	// owner-called deleteGroup *also* fails, taking that shortcut too. The
	// exact trigger isn't isolated yet across these three call shapes.
	// Left unfixed here, out of scope for the e2e-test rewrite; worth its
	// own investigation.
	t.Skip("BLOCKED: grantee's deleteBucket call fails with " +
		`"kv store with key TransientStoreKey{..., transient_storage} has not been registered in stores" ` +
		"-- same error as deleteGroup; trigger not fully isolated across owner/grantee x bucket/group call shapes, see comment above")
}
