package tests

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/mocachain/moca/v2/precompiles/storage"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	mocatypes "github.com/mocachain/moca/v2/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

const (
	// createObjectDeniedReason is x/storage's ErrAccessDenied text for a creator
	// without CREATE_OBJECT rights on the bucket, the string the retired suite's
	// TestCreateObjectByOthers* scenarios matched on.
	createObjectDeniedReason = "has no CreateObject permission of the bucket"
	mebibyte                 = uint64(1024 * 1024)
	// policyExpiryLeadSeconds is how far past the current block time a test
	// policy expires: enough for the grant to be exercised first, short enough
	// that waiting it out is cheap.
	policyExpiryLeadSeconds = 10
)

// createObjectGrant is a funded owner with a fresh public-read bucket on sp0,
// plus a funded grantee that starts with no rights on it.
type createObjectGrant struct {
	sp          spHandle
	ownerKey    *ecdsa.PrivateKey
	ownerAddr   common.Address
	granteeKey  *ecdsa.PrivateKey
	granteeAddr common.Address
	bucketName  string
	resource    string
	principal   storage.Principal
}

func newCreateObjectGrant(ctx context.Context, t *testing.T, client *ethclient.Client, conn *grpc.ClientConn, chainID *big.Int) createObjectGrant {
	t.Helper()
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
	return createObjectGrant{
		sp:          sp,
		ownerKey:    ownerKey,
		ownerAddr:   ownerAddr,
		granteeKey:  granteeKey,
		granteeAddr: granteeAddr,
		bucketName:  bucketName,
		resource:    mocatypes.NewBucketGRN(bucketName).String(),
		principal:   storage.Principal{PrincipalType: int32(permtypes.PRINCIPAL_TYPE_GNFD_ACCOUNT), Value: granteeAddr.String()},
	}
}

func storagePrecompileAddress() common.Address {
	p := storage.Precompile{}
	return p.Address()
}

// verify reports the grantee's CREATE_OBJECT effect on the bucket as the
// chain evaluates it now.
func (g createObjectGrant) verify(ctx context.Context, t *testing.T, storageClient storagetypes.QueryClient) permtypes.Effect {
	t.Helper()
	resp, err := storageClient.VerifyPermission(ctx, &storagetypes.QueryVerifyPermissionRequest{
		Operator:   g.granteeAddr.String(),
		BucketName: g.bucketName,
		ActionType: permtypes.ACTION_CREATE_OBJECT,
	})
	require.NoError(t, err)
	return resp.Effect
}

// policy returns the bucket policy stored for the grantee.
func (g createObjectGrant) policy(ctx context.Context, t *testing.T, storageClient storagetypes.QueryClient) *permtypes.Policy {
	t.Helper()
	resp, err := storageClient.QueryPolicyForAccount(ctx, &storagetypes.QueryPolicyForAccountRequest{
		Resource:         g.resource,
		PrincipalAddress: g.granteeAddr.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Policy)
	return resp.Policy
}

// putPolicy has the owner grant the grantee the given statement on the
// bucket, with a policy-level expiry at expiresAt (0 = never).
func (g createObjectGrant) putPolicy(ctx context.Context, t *testing.T, client *ethclient.Client, chainID *big.Int, statement storage.Statement, expiresAt int64) {
	t.Helper()
	method := storage.GetAbiMethod(storage.PutPolicyMethodName)
	args, err := method.Inputs.Pack(g.principal, g.resource, []storage.Statement{statement}, expiresAt)
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, g.ownerKey, storagePrecompileAddress(), append(append([]byte{}, method.ID...), args...))
}

func (g createObjectGrant) deletePolicy(ctx context.Context, t *testing.T, client *ethclient.Client, chainID *big.Int) {
	t.Helper()
	method := storage.GetAbiMethod(storage.DeletePolicyMethodName)
	args, err := method.Inputs.Pack(g.principal, g.resource)
	require.NoError(t, err)
	sendPrecompileTx(ctx, t, client, chainID, g.ownerKey, storagePrecompileAddress(), append(append([]byte{}, method.ID...), args...))
}

// createObjectCalldata packs a createObject call for the grantee. The SP
// approval signs MsgCreateObject.GetApprovalBytes() with the grantee as the
// creator, so the calldata is only valid when the grantee is the tx sender.
func (g createObjectGrant) createObjectCalldata(t *testing.T, objectName string, payloadSize uint64) []byte {
	t.Helper()
	rawChecksums, b64Checksums := threeChecksums()
	fakeMsg := storagetypes.NewMsgCreateObject(
		g.granteeAddr.Bytes(), g.bucketName, objectName, payloadSize, storagetypes.VISIBILITY_TYPE_INHERIT,
		rawChecksums, "application/octet-stream", storagetypes.REDUNDANCY_EC_TYPE, 0, nil,
	)
	approvalSig, err := crypto.Sign(crypto.Keccak256(fakeMsg.GetApprovalBytes()), g.sp.ApprovalKey)
	require.NoError(t, err)

	method := storage.GetAbiMethod(storage.CreateObjectMethodName)
	args, err := method.Inputs.Pack(
		g.bucketName,
		objectName,
		payloadSize,
		uint8(storagetypes.VISIBILITY_TYPE_INHERIT),
		"application/octet-stream",
		storage.Approval{ExpiredHeight: 0, GlobalVirtualGroupFamilyId: 0, Sig: approvalSig},
		b64Checksums,
		uint8(storagetypes.REDUNDANCY_EC_TYPE),
	)
	require.NoError(t, err)
	return append(append([]byte{}, method.ID...), args...)
}

// createObject has the grantee create an object of payloadSize bytes and
// checks the chain stored it under the bucket owner with the grantee as creator.
func (g createObjectGrant) createObject(ctx context.Context, t *testing.T, client *ethclient.Client, chainID *big.Int, storageClient storagetypes.QueryClient, payloadSize uint64) {
	t.Helper()
	objectName := storageutils.GenRandomObjectName()
	sendPrecompileTx(ctx, t, client, chainID, g.granteeKey, storagePrecompileAddress(), g.createObjectCalldata(t, objectName, payloadSize))
	head, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: g.bucketName, ObjectName: objectName})
	require.NoError(t, err)
	require.Equal(t, payloadSize, head.ObjectInfo.PayloadSize)
	require.Equal(t, g.ownerAddr.String(), head.ObjectInfo.Owner, "an object a grantee creates still belongs to the bucket owner")
	require.Equal(t, g.granteeAddr.String(), head.ObjectInfo.Creator)
}

// createObjectDenied has the grantee attempt a create the chain must refuse
// for lack of CREATE_OBJECT permission, and checks nothing was stored.
func (g createObjectGrant) createObjectDenied(ctx context.Context, t *testing.T, client *ethclient.Client, chainID *big.Int, storageClient storagetypes.QueryClient, payloadSize uint64) {
	t.Helper()
	objectName := storageutils.GenRandomObjectName()
	reason := sendPrecompileTxExpectRevert(ctx, t, client, chainID, g.granteeKey, storagePrecompileAddress(), g.createObjectCalldata(t, objectName, payloadSize))
	require.Contains(t, reason, createObjectDeniedReason)
	_, err := storageClient.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{BucketName: g.bucketName, ObjectName: objectName})
	require.Error(t, err, "a refused create must leave no object behind")
}

// TestPermissionCreateObjectExpirationEvmFlow drives a policy-level expiry
// through the storage precompile's putPolicy method, the retired suite's
// TestCreateObjectByOthers and TestCreateObjectByOthersExpiration in one
// flow: while the grant is live the grantee creates an object that still
// belongs to the bucket owner, and once block time passes the policy's expiry
// the permission query flips to deny and a real createObject tx reverts with
// x/storage's access-denied reason.
func TestPermissionCreateObjectExpirationEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	g := newCreateObjectGrant(ctx, t, client, conn, chainID)

	require.Equal(t, permtypes.EFFECT_DENY, g.verify(ctx, t, storageClient), "baseline: no grant yet")

	expiresAt := latestBlockTime(ctx, t, client) + policyExpiryLeadSeconds
	g.putPolicy(ctx, t, client, chainID, storage.Statement{
		Effect:  int32(permtypes.EFFECT_ALLOW),
		Actions: []int32{int32(permtypes.ACTION_CREATE_OBJECT)},
	}, expiresAt)
	require.Equal(t, permtypes.EFFECT_ALLOW, g.verify(ctx, t, storageClient), "grant should take effect")
	policy := g.policy(ctx, t, storageClient)
	require.NotNil(t, policy.ExpirationTime)
	require.Equal(t, expiresAt, policy.ExpirationTime.Unix())

	g.createObject(ctx, t, client, chainID, storageClient, mebibyte)

	waitForBlockTime(ctx, t, client, expiresAt)
	require.Equal(t, permtypes.EFFECT_DENY, g.verify(ctx, t, storageClient), "an expired policy must stop granting")
	g.createObjectDenied(ctx, t, client, chainID, storageClient, mebibyte)

	// The owner can still clean the expired policy up explicitly.
	g.deletePolicy(ctx, t, client, chainID)
	require.Equal(t, permtypes.EFFECT_DENY, g.verify(ctx, t, storageClient))
}

// TestPermissionCreateObjectLimitSizeEvmFlow drives a statement-level
// LimitSize through the storage precompile's putPolicy method, the retired
// suite's TestCreateObjectByOthersLimitSize: every object the grantee creates
// spends the allowance, a create larger than what is left reverts without
// touching it, an object of exactly the remainder fits, and after that
// nothing further does.
func TestPermissionCreateObjectLimitSizeEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	storageClient := storagetypes.NewQueryClient(conn)
	g := newCreateObjectGrant(ctx, t, client, conn, chainID)

	require.Equal(t, permtypes.EFFECT_DENY, g.verify(ctx, t, storageClient), "baseline: no grant yet")

	limit := 3 * mebibyte / 2
	g.putPolicy(ctx, t, client, chainID, storage.Statement{
		Effect:    int32(permtypes.EFFECT_ALLOW),
		Actions:   []int32{int32(permtypes.ACTION_CREATE_OBJECT)},
		LimitSize: limit,
	}, 0)
	require.Equal(t, permtypes.EFFECT_ALLOW, g.verify(ctx, t, storageClient), "grant should take effect")
	remaining := func() uint64 {
		statements := g.policy(ctx, t, storageClient).Statements
		require.Len(t, statements, 1)
		return statements[0].LimitSize.GetValue()
	}
	require.Equal(t, limit, remaining())

	g.createObject(ctx, t, client, chainID, storageClient, mebibyte)
	require.Equal(t, limit-mebibyte, remaining(), "the allowance must shrink by the created object's size")

	g.createObjectDenied(ctx, t, client, chainID, storageClient, mebibyte)
	require.Equal(t, limit-mebibyte, remaining(), "a refused create must not spend any allowance")

	g.createObject(ctx, t, client, chainID, storageClient, limit-mebibyte)
	require.Zero(t, remaining(), "an object of exactly the remainder must exhaust the allowance")

	g.createObjectDenied(ctx, t, client, chainID, storageClient, 1)
	// The generic permission query carries no size, so it still reports the
	// statement as allowing: the allowance is enforced at create time only.
	require.Equal(t, permtypes.EFFECT_ALLOW, g.verify(ctx, t, storageClient))

	g.deletePolicy(ctx, t, client, chainID)
	require.Equal(t, permtypes.EFFECT_DENY, g.verify(ctx, t, storageClient), "deletePolicy must fully revoke the grant")
}
