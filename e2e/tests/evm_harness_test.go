// Package tests holds moca's EVM/precompile-driven e2e suite. It replaces the
// old Cosmos-message suite retired alongside e2e/core: every message these
// tests exercise is dispatched through its precompile over a real signed EVM
// transaction against a live node, not constructed and signed directly.
//
// Run against a chain started via:
//
//	deployment/localup/localup.sh all 1 7
//	deployment/localup/localup.sh export_sps 1 7
package tests

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	sdkmath "cosmossdk.io/math"

	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

const (
	evmRPCAddr        = "http://127.0.0.1:8545"
	grpcAddr          = "localhost:9090"
	evmChainIDNum     = 5151
	spExportRelPath   = "../../deployment/localup/.local/sp_export.json"
	oneMocaInAmoca    = "1000000000000000000"
	fundingAmountMOCA = 10
)

// devAccountPrivateKeyHex is deployment/localup/localup.sh's own hardcoded
// well-known local-devnet key (devaccount_prikey), pre-funded at genesis for
// every localup chain purely for test purposes -- not a secret.
const devAccountPrivateKeyHex = "2228e392584d902843272c37fd62b8c73c10c81a5ecb901773c9ebe366e937bb"

type spExportEntry struct {
	OperatorAddress    string `json:"OperatorAddress"`
	ApprovalAddress    string `json:"ApprovalAddress"`
	GcAddress          string `json:"GcAddress"`
	FundingAddress     string `json:"FundingAddress"`
	OperatorPrivateKey string `json:"OperatorPrivateKey"`
	ApprovalPrivateKey string `json:"ApprovalPrivateKey"`
	GcPrivateKey       string `json:"GcPrivateKey"`
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

func mustBigInt(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 10)
	require.True(t, ok, "invalid integer literal: %s", s)
	return n
}

// dialChain connects to the local EVM JSON-RPC and Cosmos gRPC endpoints a
// localup chain exposes, registering cleanup for both.
func dialChain(t *testing.T) (*ethclient.Client, *grpc.ClientConn) {
	t.Helper()
	client, err := ethclient.Dial(evmRPCAddr)
	require.NoError(t, err, "no live chain at %s -- run localup.sh first", evmRPCAddr)
	t.Cleanup(client.Close)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return client, conn
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

// fundAccount sends a plain native-token transfer and waits for it to land.
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

// fundMoca funds `to` with the given whole-MOCA amount from the well-known
// local devnet dev account.
func fundMoca(t *testing.T, ctx context.Context, client *ethclient.Client, chainID *big.Int, to common.Address, wholeMoca int64) {
	t.Helper()
	devKey := mustHexKey(t, devAccountPrivateKeyHex)
	fundAccount(t, ctx, client, chainID, devKey, to, new(big.Int).Mul(big.NewInt(wholeMoca), mustBigInt(t, oneMocaInAmoca)))
}

// getStreamRecord fetches an account's payment stream record, treating a
// not-found response as an implicit all-zero record -- an account that has
// never had any payment activity has no row at all yet, same tolerance the
// retired suite's own getStreamRecord helper had.
func getStreamRecord(t *testing.T, ctx context.Context, paymentClient paymenttypes.QueryClient, account string) paymenttypes.StreamRecord {
	t.Helper()
	resp, err := paymentClient.StreamRecord(ctx, &paymenttypes.QueryGetStreamRecordRequest{Account: account})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return paymenttypes.StreamRecord{
				Account:           account,
				NetflowRate:       sdkmath.ZeroInt(),
				StaticBalance:     sdkmath.ZeroInt(),
				BufferBalance:     sdkmath.ZeroInt(),
				LockBalance:       sdkmath.ZeroInt(),
				FrozenNetflowRate: sdkmath.ZeroInt(),
			}
		}
		require.NoError(t, err)
	}
	return resp.StreamRecord
}
