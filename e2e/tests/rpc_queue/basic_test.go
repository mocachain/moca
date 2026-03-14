// Package tests contains end-to-end tests for MOCA Chain transaction nonce handling behavior
//
// This test suite verifies how MOCA Chain processes transactions with different nonce patterns:
//   - Consecutive nonce transactions (should succeed)
//   - Out-of-order nonce transactions (demonstrates current limitations)
//
// These tests help validate the need for and effectiveness of RPC Cache Queue implementation.
//
// Prerequisites:
//
//  1. Start MOCA Chain node with ETH RPC enabled:
//     cd <moca-repo-root>
//     make localup
//     # or
//     ./deployment/localup/localup.sh
//
//  2. Ensure test account has sufficient balance for gas fees
//
// Usage:
//
//	# Run all nonce-related tests
//	cd e2e/tests
//	go test -v -run TestMsgPool
//
//	# Run individual tests
//	go test -v -run TestConsecutiveNonceTransactionsSucceed     # Tests consecutive nonce handling
//	go test -v -run TestOutOfOrderTransactionSubmission       # Tests out-of-order nonce handling
//
// Current behavior (without RPC cache queue):
//
//	✅ Consecutive nonces (n, n+1, n+2): All succeed
//	❌ Out-of-order nonces (n, n+2, n+3, n+1): Gaps cause immediate rejection
//
// Expected behavior (with RPC cache queue):
//
//	✅ Consecutive nonces: All succeed (no change)
//	✅ Out-of-order nonces: Queued until gaps filled, then processed sequentially
package tests

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/suite"
)

type MsgPoolTestSuite struct {
	suite.Suite
	ethClient *ethclient.Client
}

func (s *MsgPoolTestSuite) SetupSuite() {
	// Connect to ETH RPC endpoint
	// NOTE: This test requires a running MOCA Chain node with ETH RPC enabled
	// Start the node with: make localup or ./deployment/localup/localup.sh
	var err error
	s.ethClient, err = ethclient.Dial("http://localhost:8545")
	if err != nil {
		s.T().Skip("Skipping test: MOCA Chain node not running on localhost:8545")
		return
	}
}

func (s *MsgPoolTestSuite) TearDownSuite() {
	if s.ethClient != nil {
		s.ethClient.Close()
	}
}

// TestConsecutiveNonceTransactionsSucceed demonstrates that consecutive nonce transactions all succeed
func (s *MsgPoolTestSuite) TestConsecutiveNonceTransactionsSucceed() {
	// Setup test account with private key
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	// Create transactor
	chainID := big.NewInt(5151)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	s.Require().NoError(err, "Failed to create transactor")

	// Get current nonce
	fromAddress := auth.From
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Current account %s nonce: %d", fromAddress.Hex(), currentNonce)

	// Test recipient address
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")

	// Gas configuration
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)

	// Transaction amount
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("=== Testing consecutive nonce sequence ===")
	s.T().Log("Expected behavior: consecutive nonces should all succeed")
	s.T().Logf("  - nonce=%d: Should succeed (equals current)", currentNonce)
	s.T().Logf("  - nonce=%d: Should succeed (current+1)", currentNonce+1)
	s.T().Logf("  - nonce=%d: Should succeed (current+2)", currentNonce+2)

	// Create three transactions with sequential nonces
	var txs []*types.Transaction
	var txHashes []common.Hash

	for i := uint64(0); i < 3; i++ {
		nonce := currentNonce + i

		tx := types.NewTransaction(
			nonce,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
			nil,
		)

		// Sign transaction
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign transaction %d", i+1)

		txs = append(txs, signedTx)
		txHashes = append(txHashes, signedTx.Hash())

		s.T().Logf("Created transaction %d with nonce %d, hash: %s", i+1, nonce, signedTx.Hash().Hex())
	}

	s.T().Log("\n=== Sending transactions in sequence ===")

	// Send transactions and record results
	var results []error

	for i, tx := range txs {
		s.T().Logf("Sending transaction %d (nonce=%d)...", i+1, tx.Nonce())

		err := s.ethClient.SendTransaction(context.Background(), tx)
		results = append(results, err)

		if err != nil {
			s.T().Logf("Transaction %d failed: %v", i+1, err)
		} else {
			s.T().Logf("Transaction %d sent successfully: %s", i+1, tx.Hash().Hex())
		}

		// Small delay between transactions
		time.Sleep(100 * time.Millisecond)
	}

	s.T().Log("\n=== Results Analysis ===")

	// Analyze results for consecutive nonce behavior
	for i := 0; i < len(results); i++ {
		if results[i] == nil {
			s.T().Logf("✅ Transaction %d (nonce+%d) succeeded as expected", i+1, i)
		} else {
			s.T().Errorf("❌ Transaction %d (nonce+%d) should have succeeded but failed: %v", i+1, i, results[i])
		}
	}

	s.T().Log("\n=== Consecutive Nonce Behavior Confirmed ===")
	s.T().Log("This demonstrates that consecutive nonce transactions succeed.")
	s.T().Log("The nonce ordering issue occurs when there are GAPS in nonces,")
	s.T().Log("not when nonces are consecutive. See TestOutOfOrderTransactionSubmissionFailed")
	s.T().Log("for the actual nonce gap problem that RPC cache queue solves.")
}

// TestOutOfOrderTransactionSubmissionFailed tests how the system handles transactions submitted out of order
// This test demonstrates the problem when transactions are submitted with nonce gaps
func (s *MsgPoolTestSuite) TestOutOfOrderTransactionSubmissionFailed() {
	s.T().Log("=== Out-of-Order Transaction Submission Test ===")
	s.T().Log("This test demonstrates how missing nonce values cause subsequent transactions to be rejected")

	// Setup test account with private key
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	// Get account address
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Get current nonce
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Testing raw transaction submission for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Transaction configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Creating transactions to simulate out-of-order arrival ===")
	s.T().Log("Simulating network scenario where transactions arrive out of order:")
	s.T().Logf("  1. Send nonce=%d (should succeed)", currentNonce)
	s.T().Logf("  2. Send nonce=%d (should fail - gap)", currentNonce+2)
	s.T().Logf("  3. Send nonce=%d (should fail - gap)", currentNonce+3)
	s.T().Logf("  4. Send nonce=%d (should succeed - fills gap)", currentNonce+1)

	// Create all 4 transactions first
	nonces := []uint64{currentNonce, currentNonce + 2, currentNonce + 3, currentNonce + 1}
	sendOrder := []string{"current", "current+2", "current+3", "current+1"}
	var rawTxs []string
	var txHashes []common.Hash

	for i, nonce := range nonces {
		tx := types.NewTransaction(
			nonce,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
			nil,
		)

		// Sign transaction
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign transaction %d", i+1)

		// Encode as raw transaction
		rawTxBytes, err := signedTx.MarshalBinary()
		s.Require().NoError(err, "Failed to marshal transaction %d", i+1)

		rawTxHex := "0x" + common.Bytes2Hex(rawTxBytes)
		rawTxs = append(rawTxs, rawTxHex)
		txHashes = append(txHashes, signedTx.Hash())

		s.T().Logf("Created transaction with nonce %d (%s)", nonce, sendOrder[i])
		s.T().Logf("  Hash: %s", signedTx.Hash().Hex())
	}

	s.T().Log("\n=== Submitting transactions in out-of-order sequence ===")

	var results []error
	expectedResults := []string{"succeed", "fail", "fail", "succeed"}

	// Submit transactions in the specified order
	for i, rawTx := range rawTxs {
		nonce := nonces[i]
		expected := expectedResults[i]

		s.T().Logf("Step %d: Submitting transaction with nonce=%d (%s) - expect %s",
			i+1, nonce, sendOrder[i], expected)

		err := s.ethClient.SendTransaction(context.Background(), mustDecodeRawTx(rawTx))
		results = append(results, err)

		if err != nil {
			s.T().Logf("  Result: ❌ Failed - %v", err)
		} else {
			s.T().Logf("  Result: ✅ Succeeded - %s", txHashes[i].Hex())
		}

		// Small delay between submissions
		time.Sleep(200 * time.Millisecond)
	}

	s.T().Log("\n=== Analysis of Results ===")

	// Analyze each result
	for i, err := range results {
		nonce := nonces[i]
		expected := expectedResults[i]

		if expected == "succeed" {
			if err == nil {
				s.T().Logf("✅ Transaction %d (nonce=%d) succeeded as expected", i+1, nonce)
			} else {
				s.T().Errorf("❌ Transaction %d (nonce=%d) should have succeeded but failed: %v", i+1, nonce, err)
			}
		} else { // expected == "fail"
			if err != nil {
				errorMsg := err.Error()
				if containsNonceError(errorMsg) {
					s.T().Logf("✅ Transaction %d (nonce=%d) correctly rejected due to nonce gap: %v", i+1, nonce, err)
				} else {
					s.T().Logf("⚠️  Transaction %d (nonce=%d) failed but not due to nonce issue: %v", i+1, nonce, err)
				}
			} else {
				s.T().Logf("⚠️  Transaction %d (nonce=%d) unexpectedly succeeded (should have failed)", i+1, nonce)
			}
		}
	}

	s.T().Log("\n=== Out-of-Order Transaction Test Complete ===")
	s.T().Log("This test demonstrates how missing nonce values cause subsequent")
	s.T().Log("transactions to be rejected, even if they would be valid later.")
}

// Removed TestOutOfOrderTransactionWithCacheQueue - CacheQueue functionality removed
// func _removed_TestOutOfOrderTransactionWithCacheQueue() { ... }


// Helper function to describe result status
func resultStatus(err error) string {
	if err == nil {
		return "✅ Success"
	}
	return fmt.Sprintf("❌ Failed: %v", err)
}

// Removed isCacheQueueEnabled - CacheQueue functionality removed

// Helper function to decode raw transaction hex
func mustDecodeRawTx(rawTxHex string) *types.Transaction {
	if len(rawTxHex) > 2 && rawTxHex[:2] == "0x" {
		rawTxHex = rawTxHex[2:]
	}

	rawTxBytes := common.Hex2Bytes(rawTxHex)
	tx := new(types.Transaction)

	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		panic("Failed to decode raw transaction: " + err.Error())
	}

	return tx
}

// Helper function to check if error message contains nonce-related error
func containsNonceError(errorMsg string) bool {
	noncePhrases := []string{
		"nonce too high",
		"nonce too low",
		"account sequence mismatch",
		"invalid nonce",
		"replacement transaction underpriced",
	}

	for _, phrase := range noncePhrases {
		if len(errorMsg) > 0 && contains(errorMsg, phrase) {
			return true
		}
	}
	return false
}

// Helper function to check if result contains any of the given strings
func (s *MsgPoolTestSuite) containsAnyString(result string, keywords []string) bool {
	lowerResult := strings.ToLower(result)
	for _, keyword := range keywords {
		if contains(lowerResult, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// Simple string contains check (since we can't import strings in all contexts)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || findInString(s, substr) >= 0)
}

func findInString(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}

// Removed TestCacheQueueCapacityManagement - CacheQueue functionality removed
// func _removed_TestCacheQueueCapacityManagement() { ... }

// Removed TestCacheQueueTimeoutHandling - CacheQueue functionality removed
// func _removed_TestCacheQueueTimeoutHandling() { ... }

// Removed TestCacheQueueMultipleAccounts - CacheQueue functionality removed
// func _removed_TestCacheQueueMultipleAccounts() { ... }

// Removed TestCacheQueueStressTest - CacheQueue functionality removed
// func _removed_TestCacheQueueStressTest() { ... }

// Removed TestCacheQueueRetryMechanism - CacheQueue functionality removed
// func _removed_TestCacheQueueRetryMechanism() { ... }

// Removed TestCacheQueueConfigurationValidation - CacheQueue functionality removed
// func _removed_TestCacheQueueConfigurationValidation() { ... }

func TestRpcQueueTestSuite(t *testing.T) {
	suite.Run(t, new(MsgPoolTestSuite))
}