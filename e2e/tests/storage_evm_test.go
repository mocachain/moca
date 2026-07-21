package tests

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/precompiles/virtualgroup"
	storageutils "github.com/mocachain/moca/v2/testutil/storage"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestStorageEvmFlow drives moca's storage/virtualgroup modules entirely
// through their EVM precompiles (createGlobalVirtualGroup, createBucket,
// updateBucketInfo), exercising the same functional path as the legacy
// Eip712TestSuite.TestMultiMessages but over real signed EVM transactions
// against a live node, instead of the now-broken SIGN_MODE_EIP_712 Cosmos
// path. Run against a chain started via:
//
//	deployment/localup/localup.sh all 1 7
//	deployment/localup/localup.sh export_sps 1 7
const (
	evmRPCAddr        = "http://127.0.0.1:8545"
	grpcAddr          = "localhost:9090"
	evmChainIDNum     = 5151
	spExportRelPath   = "../../deployment/localup/.local/sp_export.json"
	oneMocaInAmoca    = "1000000000000000000"
	gvgDepositAmount  = "1000000000000000000000" // 1000 MOCA in amoca base units, mirrors e2e/core's default GVG deposit.
	fundingAmountMOCA = 10
)

// devAccountPrivateKeyHex is deployment/localup/localup.sh's own hardcoded
// well-known local-devnet key (devaccount_prikey), pre-funded at genesis for
// every localup chain purely for test purposes -- not a secret.
const devAccountPrivateKeyHex = "2228e392584d902843272c37fd62b8c73c10c81a5ecb901773c9ebe366e937bb"

type spExportEntry struct {
	OperatorAddress    string `json:"OperatorAddress"`
	ApprovalAddress    string `json:"ApprovalAddress"`
	OperatorPrivateKey string `json:"OperatorPrivateKey"`
	ApprovalPrivateKey string `json:"ApprovalPrivateKey"`
}

func loadSPExport(t *testing.T) map[string]spExportEntry {
	t.Helper()
	data, err := os.ReadFile(spExportRelPath) //nolint:gosec // fixed relative path to local test fixture
	require.NoError(t, err, "run `localup.sh export_sps 1 7` before this test")
	var out map[string]spExportEntry
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

func mustHexKey(t *testing.T, hexKey string) *ecdsa.PrivateKey {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(hexKey), "0x"))
	require.NoError(t, err)
	return key
}

// sendPrecompileTx signs and sends a dynamic-fee tx carrying calldata to a
// precompile address, waits for the receipt, and asserts success.
func sendPrecompileTx(t *testing.T, ctx context.Context, client *ethclient.Client, chainID *big.Int, key *ecdsa.PrivateKey, to common.Address, calldata []byte) *types.Receipt {
	t.Helper()
	from := crypto.PubkeyToAddress(key.PublicKey)

	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)

	tipCap, err := client.SuggestGasTipCap(ctx)
	require.NoError(t, err)

	header, err := client.HeaderByNumber(ctx, nil)
	require.NoError(t, err)
	feeCap := new(big.Int).Add(tipCap, new(big.Int).Mul(header.BaseFee, big.NewInt(2)))

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: from,
		To:   &to,
		Data: calldata,
	})
	if err != nil {
		// Precompile gas estimation can be unreliable; fall back to a
		// generous fixed limit rather than failing the whole tx upfront.
		gas = 2_000_000
	} else {
		gas += gas / 5 // 20% headroom
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tipCap,
		GasFeeCap: feeCap,
		Gas:       gas,
		To:        &to,
		Data:      calldata,
	})

	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), key)
	require.NoError(t, err)
	require.NoError(t, client.SendTransaction(ctx, signedTx))

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, client, signedTx)
	require.NoError(t, err)
	if receipt.Status != 1 {
		// Re-simulate at the reverting block to surface the revert reason;
		// go-ethereum doesn't attach it to the mined receipt.
		_, callErr := client.CallContract(ctx, ethereum.CallMsg{
			From: from, To: &to, Data: calldata,
		}, receipt.BlockNumber)
		t.Fatalf("tx %s reverted; revert reason: %v", signedTx.Hash(), callErr)
	}
	return receipt
}

func fundAccount(t *testing.T, ctx context.Context, client *ethclient.Client, chainID *big.Int, funder *ecdsa.PrivateKey, to common.Address, amount *big.Int) {
	t.Helper()
	from := crypto.PubkeyToAddress(funder.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	tipCap, err := client.SuggestGasTipCap(ctx)
	require.NoError(t, err)
	header, err := client.HeaderByNumber(ctx, nil)
	require.NoError(t, err)
	feeCap := new(big.Int).Add(tipCap, new(big.Int).Mul(header.BaseFee, big.NewInt(2)))

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: tipCap,
		GasFeeCap: feeCap,
		Gas:       21_000,
		To:        &to,
		Value:     amount,
	})
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), funder)
	require.NoError(t, err)
	require.NoError(t, client.SendTransaction(ctx, signedTx))

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, client, signedTx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status)
}

func TestStorageEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)

	client, err := ethclient.Dial(evmRPCAddr)
	require.NoError(t, err, "no live chain at %s -- run localup.sh first", evmRPCAddr)
	defer client.Close()

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

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
	devKey := mustHexKey(t, devAccountPrivateKeyHex)

	// Fund sp0's operator account for gas -- it holds its self-deposit but
	// may have no free spendable balance for tx fees.
	fundAccount(t, ctx, client, chainID, devKey, crypto.PubkeyToAddress(sp0OperatorKey.PublicKey),
		new(big.Int).Mul(big.NewInt(fundingAmountMOCA), mustBigInt(t, oneMocaInAmoca)))

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
	fundAccount(t, ctx, client, chainID, devKey, userAddr,
		new(big.Int).Mul(big.NewInt(fundingAmountMOCA), mustBigInt(t, oneMocaInAmoca)))

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

func mustBigInt(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 10)
	require.True(t, ok, "invalid integer literal: %s", s)
	return n
}
