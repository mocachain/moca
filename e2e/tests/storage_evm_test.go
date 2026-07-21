package tests

import (
	"context"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/precompiles/virtualgroup"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// gvgDepositAmount is 1000 MOCA in amoca base units, mirroring the retired
// e2e/core suite's own default GVG deposit.
const gvgDepositAmount = "1000000000000000000000"

// TestStorageEvmFlow drives moca's storage/virtualgroup modules entirely
// through their EVM precompiles (createGlobalVirtualGroup, createBucket,
// updateBucketInfo), exercising the same functional path as the retired
// suite's eip712_test.go (create a bucket, then flip its visibility) but
// over real signed EVM transactions against a live node.
func TestStorageEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)

	spClient := sptypes.NewQueryClient(conn)
	storageClient := storagetypes.NewQueryClient(conn)
	vgClient := virtualgroupmoduletypes.NewQueryClient(conn)

	spExport := loadSPExport(t)
	sp0Export, ok := spExport["sp0"]
	require.True(t, ok, "sp_export.json missing sp0 -- expected at least 7 SPs (localup.sh all 1 7)")

	// Resolve on-chain numeric SP IDs: sp0 is primary, everyone else is secondary.
	spsResp, err := spClient.StorageProviders(ctx, &sptypes.QueryStorageProvidersRequest{
		Pagination: &query.PageRequest{Limit: math.MaxUint64},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(spsResp.Sps), 7, "expected >=7 genesis SPs; run localup.sh all 1 7")

	var primarySPID uint32
	var secondarySPIDs []uint32
	for _, sp := range spsResp.Sps {
		if strings.EqualFold(sp.OperatorAddress, sp0Export.OperatorAddress) {
			primarySPID = sp.Id
		} else {
			secondarySPIDs = append(secondarySPIDs, sp.Id)
		}
	}
	require.NotZero(t, primarySPID, "sp0 not found on-chain by operator address")

	sp0OperatorKey := mustHexKey(t, sp0Export.OperatorPrivateKey)
	sp0ApprovalKey := mustHexKey(t, sp0Export.ApprovalPrivateKey)

	// Fund sp0's operator account for gas -- it holds its self-deposit but
	// may have no free spendable balance for tx fees.
	fundMoca(t, ctx, client, chainID, crypto.PubkeyToAddress(sp0OperatorKey.PublicKey), fundingAmountMOCA)

	// 1) Create a fresh GVG family for sp0 via the virtualgroup precompile.
	vgPrecompile := virtualgroup.Precompile{}
	vgMethod, err := virtualgroup.GetMethod(virtualgroup.CreateGlobalVirtualGroupMethodName)
	require.NoError(t, err)
	depositAmt := mustBigInt(t, gvgDepositAmount)
	// x/storage's GetExpectSecondarySPNumForECObject fixes the EC scheme's
	// secondary-SP count (2 in this genesis); the module rejects any other
	// count outright, so only the first 2 of the other 6 SPs are used here.
	require.GreaterOrEqual(t, len(secondarySPIDs), 2)
	packedArgs, err := vgMethod.Inputs.Pack(
		uint32(0), // familyId=0 -> create a new family
		secondarySPIDs[:2],
		virtualgroup.Coin{Denom: "amoca", Amount: depositAmt},
	)
	require.NoError(t, err)
	calldata := append(append([]byte{}, vgMethod.ID...), packedArgs...)
	sendPrecompileTx(t, ctx, client, chainID, sp0OperatorKey, vgPrecompile.Address(), calldata)

	// 2) Discover the family/GVG IDs just created.
	familiesResp, err := vgClient.GlobalVirtualGroupFamilies(ctx, &virtualgroupmoduletypes.QueryGlobalVirtualGroupFamiliesRequest{
		Pagination: &query.PageRequest{Limit: math.MaxUint64},
	})
	require.NoError(t, err)
	var familyID uint32
	for _, fam := range familiesResp.GvgFamilies {
		if fam.PrimarySpId == primarySPID {
			familyID = fam.Id
		}
	}
	require.NotZero(t, familyID, "no GVG family found for sp0 after createGlobalVirtualGroup")

	// 3) Create + fund a fresh EVM "user" key to act as bucket creator.
	userKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	userAddr := crypto.PubkeyToAddress(userKey.PublicKey)
	fundMoca(t, ctx, client, chainID, userAddr, fundingAmountMOCA)

	// 4) Compute the SP approval signature the same way the storage keeper
	// verifies it: Keccak256(MsgCreateBucket.GetApprovalBytes()), ECDSA-signed.
	bucketName := storageutils.GenRandomBucketName()
	fakeMsg := storagetypes.NewMsgCreateBucket(
		userAddr.Bytes(), bucketName, storagetypes.VISIBILITY_TYPE_PUBLIC_READ,
		crypto.PubkeyToAddress(sp0OperatorKey.PublicKey).Bytes(), userAddr.Bytes(),
		math.MaxUint, nil, 0,
	)
	fakeMsg.PrimarySpApproval.GlobalVirtualGroupFamilyId = familyID
	approvalDigest := crypto.Keccak256(fakeMsg.GetApprovalBytes())
	approvalSig, err := crypto.Sign(approvalDigest, sp0ApprovalKey)
	require.NoError(t, err)

	// 5) createBucket via the storage precompile.
	storagePrecompile := storage.Precompile{}
	createMethod := storage.GetAbiMethod(storage.CreateBucketMethodName)
	createArgs, err := createMethod.Inputs.Pack(
		bucketName,
		uint8(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		userAddr,
		crypto.PubkeyToAddress(sp0OperatorKey.PublicKey),
		storage.Approval{
			ExpiredHeight:              math.MaxUint,
			GlobalVirtualGroupFamilyId: familyID,
			Sig:                        approvalSig,
		},
		uint64(0),
	)
	require.NoError(t, err)
	createCalldata := append(append([]byte{}, createMethod.ID...), createArgs...)
	sendPrecompileTx(t, ctx, client, chainID, userKey, storagePrecompile.Address(), createCalldata)

	// 6) updateBucketInfo via the storage precompile: flip to private.
	updateMethod := storage.GetAbiMethod(storage.UpdateBucketInfoMethodName)
	updateArgs, err := updateMethod.Inputs.Pack(
		bucketName,
		uint8(storagetypes.VISIBILITY_TYPE_PRIVATE),
		userAddr,
		big.NewInt(-1), // chargedReadQuota: -1 sentinel means "leave unchanged"
	)
	require.NoError(t, err)
	updateCalldata := append(append([]byte{}, updateMethod.ID...), updateArgs...)
	sendPrecompileTx(t, ctx, client, chainID, userKey, storagePrecompile.Address(), updateCalldata)

	// 7) Verify via the same gRPC query the legacy suite used.
	headResp, err := storageClient.HeadBucket(ctx, &storagetypes.QueryHeadBucketRequest{BucketName: bucketName})
	require.NoError(t, err)
	require.Equal(t, storagetypes.VISIBILITY_TYPE_PRIVATE, headResp.BucketInfo.Visibility)
}
