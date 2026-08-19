// TestEip1153TransientStorageClearsBetweenTransactions drives EIP-1153
// (transient storage: TLOAD/TSTORE) against a live moca chain. No solc/forge
// needed -- the contract is 16 bytes of hand-assembled bytecode, verified by
// disassembly before ever being sent to a chain:
//
//	TLOAD slot0 -> stack       (60 00 5c)
//	MSTORE mem[0:32] = <that>  (60 00 52)
//	TSTORE slot0 = 42          (60 2a 60 00 5d)
//	RETURN mem[0:32]           (60 20 60 00 f3)
//
// Every call reads-then-writes the same transient slot and returns what it
// read *before* its own write. The defining property of EIP-1153 (as
// opposed to ordinary SSTORE/SLOAD) is that transient storage is wiped at
// the end of every top-level call -- so calling this contract twice must
// return 0 both times. If transient storage ever leaked from one call into
// the next, the second call would observe the first call's TSTORE and
// return 42 instead.
package tests

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// eip1153InitCodeHex deploys the 16-byte runtime above. The 11-byte header
// (PUSH1 0x10, DUP1, PUSH1 0x0b, PUSH1 0x00, CODECOPY, PUSH1 0x00, RETURN)
// copies the runtime code that follows it into memory and returns it as the
// deployed contract's code -- disassembly-verified, see the comment above.
const eip1153InitCodeHex = "601080600b6000396000f360005c600052602a60005d60206000f3"

func TestEip1153TransientStorageClearsBetweenTransactions(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(eipEvmChainIDNum)
	client := eipDialChain(t)

	deployerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	deployerAddr := crypto.PubkeyToAddress(deployerKey.PublicKey)
	eipFundAccount(t, ctx, client, chainID, deployerAddr, 10)

	initCode, err := hex.DecodeString(eip1153InitCodeHex)
	require.NoError(t, err)

	nonce, err := client.PendingNonceAt(ctx, deployerAddr)
	require.NoError(t, err)
	tipCap, err := client.SuggestGasTipCap(ctx)
	require.NoError(t, err)
	header, err := client.HeaderByNumber(ctx, nil)
	require.NoError(t, err)
	feeCap := new(big.Int).Add(tipCap, new(big.Int).Mul(header.BaseFee, big.NewInt(2)))

	deployTx := types.NewTx(&types.DynamicFeeTx{
		ChainID: chainID, Nonce: nonce, GasTipCap: tipCap, GasFeeCap: feeCap,
		Gas: 200_000, Data: initCode,
	})
	signedDeployTx, err := types.SignTx(deployTx, types.LatestSignerForChainID(chainID), deployerKey)
	require.NoError(t, err)
	require.NoError(t, client.SendTransaction(ctx, signedDeployTx))

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, client, signedDeployTx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status, "contract deployment must succeed")
	contractAddr := receipt.ContractAddress
	require.NotEqual(t, common.Address{}, contractAddr)

	code, err := client.CodeAt(ctx, contractAddr, nil)
	require.NoError(t, err)
	require.Len(t, code, 16, "constructor must have deployed exactly the 16-byte runtime")

	// Two independent top-level calls -- eth_call runs each as its own
	// isolated execution against the given block, exactly like two separate
	// transactions would each get their own fresh transient-storage scope.
	first, err := client.CallContract(ctx, ethereum.CallMsg{From: deployerAddr, To: &contractAddr}, nil)
	require.NoError(t, err)
	require.True(t, new(big.Int).SetBytes(first).Sign() == 0, "the very first call has nothing transient stored yet, must read 0")

	second, err := client.CallContract(ctx, ethereum.CallMsg{From: deployerAddr, To: &contractAddr}, nil)
	require.NoError(t, err)
	require.True(t, new(big.Int).SetBytes(second).Sign() == 0,
		"a second, independent call must NOT observe the first call's TSTORE -- "+
			"transient storage is required to clear at the end of every top-level call; "+
			"a non-zero result here means it leaked across the call boundary")
}
