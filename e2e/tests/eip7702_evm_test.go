// TestEip7702Delegation drives EIP-7702 (set-code transactions) against a
// live moca chain, using go-ethereum's own SetCodeTx/SignSetCode primitives
// directly rather than importing cosmos/evm's own eip7702 integration suite
// (blocked: that suite requires embedding ibctesting.TestingApp, whose own
// package pulls in ibc-go's testing/simapp reference app -- which doesn't
// compile against moca-cosmos-sdk's forked auth/staking/bank keeper
// constructors, most notably staking's NewValidator requiring moca's
// PoA/BLS fields (relayer, challenger, blsKey) that a vanilla reference app
// has no way to supply).
package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// TestEip7702Delegation authorizes a fresh EOA to delegate to an arbitrary
// address (the designator half of EIP-7702 only records "this EOA delegates
// to address X" and doesn't require X to hold code, so proving the
// mechanism itself doesn't need a deployed contract -- verifying delegated
// *execution* semantics against a real Solidity contract is a further step
// not taken here), sends a SetCodeTx carrying that authorization, and
// confirms the authority's eth_getCode now returns the EIP-7702 delegation
// designator (0xef0100 || address).
func TestEip7702Delegation(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, _ := dialChain(t)

	authorityKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	authorityAddr := crypto.PubkeyToAddress(authorityKey.PublicKey)
	fundMoca(ctx, t, client, chainID, authorityAddr, fundingAmountMOCA)

	before, err := client.CodeAt(ctx, authorityAddr, nil)
	require.NoError(t, err)
	require.Empty(t, before, "a fresh EOA must have no code before any delegation")

	nonce, err := client.PendingNonceAt(ctx, authorityAddr)
	require.NoError(t, err)

	// Since the authority here is also the tx sender, its on-chain nonce
	// increments as part of ordinary tx processing *before* the authorization
	// is checked -- the authorization's nonce must therefore be tx-nonce+1,
	// not tx-nonce, or applyAuthorization silently rejects it as a nonce
	// mismatch (errors are swallowed and merely logged at debug level, so a
	// mismatch here looks like a fully successful, no-op transaction).
	delegateTarget := common.HexToAddress("0x00000000000000000000000000000000c0ffee")
	auth, err := types.SignSetCode(authorityKey, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: delegateTarget,
		Nonce:   nonce + 1,
	})
	require.NoError(t, err)

	recoveredAuthority, err := auth.Authority()
	require.NoError(t, err)
	require.Equal(t, authorityAddr, recoveredAuthority, "SignSetCode's own authority recovery must match the signer")

	tipCap, err := client.SuggestGasTipCap(ctx)
	require.NoError(t, err)
	header, err := client.HeaderByNumber(ctx, nil)
	require.NoError(t, err)
	feeCap := new(big.Int).Add(tipCap, new(big.Int).Mul(header.BaseFee, big.NewInt(2)))

	setCodeTx := types.NewTx(&types.SetCodeTx{
		ChainID:   uint256.MustFromBig(chainID),
		Nonce:     nonce,
		GasTipCap: uint256.MustFromBig(tipCap),
		GasFeeCap: uint256.MustFromBig(feeCap),
		Gas:       100_000,
		To:        authorityAddr,
		AuthList:  []types.SetCodeAuthorization{auth},
	})
	signedTx, err := types.SignTx(setCodeTx, types.LatestSignerForChainID(chainID), authorityKey)
	require.NoError(t, err)
	require.NoError(t, client.SendTransaction(ctx, signedTx))

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, client, signedTx)
	require.NoError(t, err)
	if receipt.Status != 1 {
		_, callErr := client.CallContract(ctx, ethereum.CallMsg{
			From: authorityAddr, To: &authorityAddr, Data: nil,
		}, receipt.BlockNumber)
		t.Fatalf("SetCodeTx reverted; revert reason: %v", callErr)
	}

	after, err := client.CodeAt(ctx, authorityAddr, nil)
	require.NoError(t, err)
	target, ok := types.ParseDelegation(after)
	require.True(t, ok, "authority's code must be a valid EIP-7702 delegation designator, got: %x", after)
	require.Equal(t, delegateTarget, target)
}
