// TestEip1153TransientStorageClearsBetweenTransactions drives EIP-1153
// (transient storage: TLOAD/TSTORE) through real transactions against a live
// moca chain. No solc/forge needed -- the contract is 22 bytes of
// hand-assembled bytecode:
//
//	TLOAD slot0             (60 00 5c)        v = transient slot 0 as the tx starts
//	SLOAD slot0, ADD        (60 00 54 01)
//	SSTORE slot0 = old + v  (60 00 55)        storage slot 0 sums every v seen
//	TSTORE slot0 = 42       (60 2a 60 00 5d)
//	TLOAD slot0             (60 00 5c)        w = what was just stored
//	SSTORE slot1 = w        (60 01 55)
//	STOP                    (00)
//
// Transient storage must be wiped at the end of every transaction, so however
// many times the contract is called, every call starts by reading 0 and
// storage slot 0 stays 0; if one call's TSTORE leaked into a later call,
// slot 0 would pick up 42. Storage slot 1 must be 42, which proves TSTORE and
// TLOAD really store and load within a transaction -- without it, slot 0
// staying 0 would also be consistent with TLOAD always returning 0.
//
// eth_call can't check this: it never commits state, so a leak between
// transactions would be invisible to it.
package tests

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

const (
	// eip1153InitHeaderHex is the 11-byte constructor (PUSH1 0x16, DUP1,
	// PUSH1 0x0b, PUSH1 0x00, CODECOPY, PUSH1 0x00, RETURN): it copies the
	// 22-byte runtime that follows it into memory and returns it as the
	// deployed code.
	eip1153InitHeaderHex = "601680600b6000396000f3"
	eip1153RuntimeHex    = "60005c60005401600055602a60005d60005c60015500"
)

func TestEip1153TransientStorageClearsBetweenTransactions(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, _ := dialChain(t)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	from := crypto.PubkeyToAddress(key.PublicKey)
	fundMoca(ctx, t, client, chainID, from, fundingAmountMOCA)

	runtime, err := hex.DecodeString(eip1153RuntimeHex)
	require.NoError(t, err)
	initCode, err := hex.DecodeString(eip1153InitHeaderHex + eip1153RuntimeHex)
	require.NoError(t, err)

	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	deployed := eip1153Mine(ctx, t, client, eip1153Send(ctx, t, client, chainID, key, nonce, nil, initCode, 200_000))
	contract := deployed.ContractAddress
	code, err := client.CodeAt(ctx, contract, nil)
	require.NoError(t, err)
	require.Equal(t, runtime, code, "the constructor must deploy exactly the runtime")

	// Two calls sent back to back, which usually share a block, then a third
	// sent after they are mined, which lands in a later block. Each is its own
	// transaction and must start with empty transient storage either way.
	call1 := eip1153Send(ctx, t, client, chainID, key, nonce+1, &contract, nil, 100_000)
	call2 := eip1153Send(ctx, t, client, chainID, key, nonce+2, &contract, nil, 100_000)
	r1 := eip1153Mine(ctx, t, client, call1)
	r2 := eip1153Mine(ctx, t, client, call2)
	t.Logf("back-to-back calls mined in blocks %d and %d", r1.BlockNumber, r2.BlockNumber)
	r3 := eip1153Mine(ctx, t, client, eip1153Send(ctx, t, client, chainID, key, nonce+3, &contract, nil, 100_000))
	require.Greater(t, r3.BlockNumber.Uint64(), r2.BlockNumber.Uint64())

	within, err := client.StorageAt(ctx, contract, common.BigToHash(big.NewInt(1)), nil)
	require.NoError(t, err)
	require.Equal(t, int64(42), new(big.Int).SetBytes(within).Int64(),
		"TLOAD right after TSTORE in the same transaction must read the stored 42")

	seen, err := client.StorageAt(ctx, contract, common.BigToHash(big.NewInt(0)), nil)
	require.NoError(t, err)
	require.Equal(t, int64(0), new(big.Int).SetBytes(seen).Int64(),
		"every call must start with empty transient storage; a non-zero sum means a call saw an earlier call's TSTORE")
}

// eip1153Send signs and broadcasts a dynamic-fee tx with an explicit nonce, so
// several can be in flight at once; to == nil deploys data as init code.
func eip1153Send(ctx context.Context, t *testing.T, client *ethclient.Client, chainID *big.Int, key *ecdsa.PrivateKey, nonce uint64, to *common.Address, data []byte, gas uint64) *types.Transaction {
	t.Helper()
	tipCap, err := client.SuggestGasTipCap(ctx)
	require.NoError(t, err)
	header, err := client.HeaderByNumber(ctx, nil)
	require.NoError(t, err)
	feeCap := new(big.Int).Add(tipCap, new(big.Int).Mul(header.BaseFee, big.NewInt(2)))

	tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID: chainID, Nonce: nonce, GasTipCap: tipCap, GasFeeCap: feeCap,
		Gas: gas, To: to, Data: data,
	}), types.LatestSignerForChainID(chainID), key)
	require.NoError(t, err)
	require.NoError(t, client.SendTransaction(ctx, tx))
	return tx
}

// eip1153Mine waits for tx to be mined and requires it to have succeeded.
func eip1153Mine(ctx context.Context, t *testing.T, client *ethclient.Client, tx *types.Transaction) *types.Receipt {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, client, tx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status, "tx %s must succeed", tx.Hash())
	return receipt
}
