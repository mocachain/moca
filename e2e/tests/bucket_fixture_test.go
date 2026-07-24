package tests

import (
	"context"
	"crypto/ecdsa"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

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

// spHandle bundles a genesis storage provider's on-chain identity with its
// role keys, for tests that need to act as that SP.
type spHandle struct {
	OperatorKey  *ecdsa.PrivateKey
	ApprovalKey  *ecdsa.PrivateKey
	OperatorAddr common.Address
	SPID         uint32
}

// setupPrimarySP resolves sp0's on-chain identity, funds its operator
// account, and creates a fresh GVG family for it via the virtualgroup
// precompile. Returns the SP handle and the new family's ID, ready for
// createTestBucket.
func setupPrimarySP(t *testing.T, ctx context.Context, client *ethclient.Client, conn *grpc.ClientConn, chainID *big.Int) (spHandle, uint32) {
	t.Helper()
	spClient := sptypes.NewQueryClient(conn)
	vgClient := virtualgroupmoduletypes.NewQueryClient(conn)

	spExport := loadSPExport(t)
	sp0Export, ok := spExport["sp0"]
	require.True(t, ok, "sp_export.json missing sp0 -- run localup.sh export_sps 1 7 first")

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

	sp := spHandle{
		OperatorKey: mustHexKey(t, sp0Export.OperatorPrivateKey),
		ApprovalKey: mustHexKey(t, sp0Export.ApprovalPrivateKey),
		SPID:        primarySPID,
	}
	sp.OperatorAddr = crypto.PubkeyToAddress(sp.OperatorKey.PublicKey)
	fundMoca(t, ctx, client, chainID, sp.OperatorAddr, fundingAmountMOCA)

	vgPrecompile := virtualgroup.Precompile{}
	vgMethod, err := virtualgroup.GetMethod(virtualgroup.CreateGlobalVirtualGroupMethodName)
	require.NoError(t, err)
	// x/storage's GetExpectSecondarySPNumForECObject fixes the EC scheme's
	// secondary-SP count (2 in this genesis); the module rejects any other
	// count outright, so only the first 2 of the other 6 SPs are used here.
	require.GreaterOrEqual(t, len(secondarySPIDs), 2)
	packedArgs, err := vgMethod.Inputs.Pack(
		uint32(0), // familyId=0 -> create a new family
		secondarySPIDs[:2],
		virtualgroup.Coin{Denom: "amoca", Amount: mustBigInt(t, gvgDepositAmount)},
	)
	require.NoError(t, err)
	calldata := append(append([]byte{}, vgMethod.ID...), packedArgs...)
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, vgPrecompile.Address(), calldata)

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

	return sp, familyID
}

// createTestBucket creates a fresh bucket in familyID via the storage
// precompile's createBucket method, owned and signed by owner (which must
// already be funded), approved by sp, with the given charged read quota.
// Returns the generated bucket name.
func createTestBucket(t *testing.T, ctx context.Context, client *ethclient.Client, chainID *big.Int, sp spHandle, familyID uint32, owner *ecdsa.PrivateKey, visibility storagetypes.VisibilityType, chargedReadQuota uint64) string {
	t.Helper()
	ownerAddr := crypto.PubkeyToAddress(owner.PublicKey)

	// Compute the SP approval signature the same way the storage keeper
	// verifies it: Keccak256(MsgCreateBucket.GetApprovalBytes()), ECDSA-signed.
	bucketName := storageutils.GenRandomBucketName()
	fakeMsg := storagetypes.NewMsgCreateBucket(
		ownerAddr.Bytes(), bucketName, visibility,
		sp.OperatorAddr.Bytes(), ownerAddr.Bytes(),
		math.MaxUint, nil, chargedReadQuota,
	)
	fakeMsg.PrimarySpApproval.GlobalVirtualGroupFamilyId = familyID
	approvalDigest := crypto.Keccak256(fakeMsg.GetApprovalBytes())
	approvalSig, err := crypto.Sign(approvalDigest, sp.ApprovalKey)
	require.NoError(t, err)

	storagePrecompile := storage.Precompile{}
	createMethod := storage.GetAbiMethod(storage.CreateBucketMethodName)
	createArgs, err := createMethod.Inputs.Pack(
		bucketName,
		uint8(visibility),
		ownerAddr,
		sp.OperatorAddr,
		storage.Approval{
			ExpiredHeight:              math.MaxUint,
			GlobalVirtualGroupFamilyId: familyID,
			Sig:                        approvalSig,
		},
		chargedReadQuota,
	)
	require.NoError(t, err)
	calldata := append(append([]byte{}, createMethod.ID...), createArgs...)
	sendPrecompileTx(t, ctx, client, chainID, owner, storagePrecompile.Address(), calldata)

	return bucketName
}
