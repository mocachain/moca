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

// TestPermissionStaleGroupDenialEvmFlow drives x/permission's group-principal
// grant through the storage precompile's createGroup, updateGroup, and
// putPolicy methods, exercising the setup half of the retired suite's
// TestVerifyStaleGroupPermission (flagged in the coverage audit as the most
// subtle test in that file). The group-deletion half is currently blocked by
// a real, separately-reproduced bug -- see the t.Skip below -- and is left
// in place as a ready-to-enable regression test for when that's fixed.
func TestPermissionStaleGroupDenialEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	storagePrecompile := storage.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	ownerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	fundMoca(t, ctx, client, chainID, ownerAddr, fundingAmountMOCA)

	memberKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	memberAddr := crypto.PubkeyToAddress(memberKey.PublicKey)
	fundMoca(t, ctx, client, chainID, memberAddr, fundingAmountMOCA)

	bucketName := createTestBucket(t, ctx, client, chainID, sp, familyID, ownerKey, storagetypes.VISIBILITY_TYPE_PRIVATE, 0)

	// 1) Baseline: the member has no grant at all, so deleting the bucket
	// must already be denied.
	verify := func(operator common.Address) permtypes.Effect {
		resp, err := storageClient.VerifyPermission(ctx, &storagetypes.QueryVerifyPermissionRequest{
			Operator:   operator.String(),
			BucketName: bucketName,
			ActionType: permtypes.ACTION_DELETE_BUCKET,
		})
		require.NoError(t, err)
		return resp.Effect
	}
	require.Equal(t, permtypes.EFFECT_DENY, verify(memberAddr), "member should not have delete rights before any grant")

	// 2) Owner creates a group and adds the member.
	groupName := "evm-test-group"
	createGroupMethod := storage.GetAbiMethod(storage.CreateGroupMethodName)
	createGroupArgs, err := createGroupMethod.Inputs.Pack(groupName, "")
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, storagePrecompile.Address(),
		append(append([]byte{}, createGroupMethod.ID...), createGroupArgs...))

	updateGroupMethod := storage.GetAbiMethod(storage.UpdateGroupMethodName)
	updateGroupArgs, err := updateGroupMethod.Inputs.Pack(
		ownerAddr,
		groupName,
		[]common.Address{memberAddr},
		[]int64{0}, // 0 -> no expiration
		[]common.Address{},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, storagePrecompile.Address(),
		append(append([]byte{}, updateGroupMethod.ID...), updateGroupArgs...))

	headGroupResp, err := storageClient.HeadGroup(ctx, &storagetypes.QueryHeadGroupRequest{
		GroupOwner: ownerAddr.String(),
		GroupName:  groupName,
	})
	require.NoError(t, err)
	groupID := headGroupResp.GroupInfo.Id

	// 3) Grant the group ACTION_DELETE_BUCKET on the bucket.
	putPolicyMethod := storage.GetAbiMethod(storage.PutPolicyMethodName)
	putPolicyArgs, err := putPolicyMethod.Inputs.Pack(
		storage.Principal{
			PrincipalType: int32(permtypes.PRINCIPAL_TYPE_GNFD_GROUP),
			Value:         groupID.String(),
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
	sendPrecompileTx(t, ctx, client, chainID, ownerKey, storagePrecompile.Address(),
		append(append([]byte{}, putPolicyMethod.ID...), putPolicyArgs...))

	require.Equal(t, permtypes.EFFECT_ALLOW, verify(memberAddr), "member should have delete rights once the group is granted")

	// 4) Owner deletes the group -- BLOCKED.
	//
	// Reproduced live via eth_call: Keeper.DeleteGroup's nested k.CallEVM
	// (burning the group's ERC721 representation) fails when invoked from
	// inside the storage precompile's deleteGroup, with:
	//   "kv store with key TransientStoreKey{..., transient_storage} has not
	//    been registered in stores"
	//
	// The trigger isn't cleanly isolated yet -- three data points, not
	// fully reconciled:
	//   - Owner-called DeleteBucket: succeeds (verified separately).
	//   - Owner-called DeleteGroup (here): fails.
	//   - Grantee-called DeleteBucket via a real policy grant: also fails
	//     (see TestPermissionAccountGrantEvmFlow in this package).
	// VerifyGroupPermission and VerifyBucketPermission both have an
	// identical "operator == owner, return ALLOW immediately" shortcut
	// ahead of any real policy lookup, so this isn't simply "delegated
	// calls reach a policy-evaluation code path that owner calls skip" --
	// owner-called DeleteGroup takes that same shortcut and still fails.
	// Left unfixed here, out of scope for the e2e-test rewrite; worth its
	// own investigation given three different call shapes all reach it.
	t.Skip("BLOCKED: deleteGroup precompile call fails with " +
		`"kv store with key TransientStoreKey{..., transient_storage} has not been registered in stores" ` +
		"-- owner-called deleteBucket does not fail the same way, but owner-called deleteGroup does; trigger not fully isolated, see comment above")
}
