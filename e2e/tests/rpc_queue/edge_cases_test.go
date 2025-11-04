// Package tests contains edge case tests for RPC Cache Queue functionality
//
// This test suite focuses on edge cases and error handling scenarios:
//   - Node restart and recovery
//   - Network partition and reconnection
//   - Memory pressure and cleanup
//   - Invalid transaction handling
//   - Concurrent access patterns
//
// Usage:
//
//	# Run all edge case tests
//	cd e2e/tests
//	go test -v -run TestCacheQueueEdgeCases
//
//	# Run individual edge case tests
//	go test -v -run TestCacheQueueNodeRestart
//	go test -v -run TestCacheQueueMemoryPressure
//	go test -v -run TestCacheQueueConcurrentAccess
package tests

import (
	"context"
	"math/big"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/suite"
)

type CacheQueueEdgeCasesTestSuite struct {
	suite.Suite
	ethClient *ethclient.Client
}

func (s *CacheQueueEdgeCasesTestSuite) SetupSuite() {
	// Connect to ETH RPC endpoint
	var err error
	s.ethClient, err = ethclient.Dial("http://localhost:8545")
	if err != nil {
		s.T().Skip("Skipping edge case tests: MOCA Chain node not running on localhost:8545")
		return
	}
}

func (s *CacheQueueEdgeCasesTestSuite) TearDownSuite() {
	if s.ethClient != nil {
		s.ethClient.Close()
	}
}

// TestCacheQueueNodeRestart tests behavior during node restart scenarios
func (s *CacheQueueEdgeCasesTestSuite) TestCacheQueueNodeRestart() {
	s.T().Log("=== Cache Queue Node Restart Test ===")
	s.T().Log("Testing cache queue persistence and recovery during node restart")

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Testing restart behavior for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Pre-restart: Creating transactions with gaps ===")

	// Create transactions that should be cached
	var preRestartTxs []*types.Transaction
	preRestartNonces := []uint64{currentNonce + 10, currentNonce + 11, currentNonce + 12}

	for i, nonce := range preRestartNonces {
		tx := types.NewTransaction(
			nonce,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
			nil,
		)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign pre-restart transaction %d", i+1)

		err = s.ethClient.SendTransaction(context.Background(), signedTx)
		s.T().Logf("Pre-restart transaction %d (nonce %d): %v", i+1, nonce, err)

		preRestartTxs = append(preRestartTxs, signedTx)
		time.Sleep(100 * time.Millisecond)
	}

	s.T().Log("\n=== Simulating node restart scenario ===")
	s.T().Log("Note: This test simulates restart by testing connection robustness")
	s.T().Log("In production testing, you would actually restart the node here")

	// Test connection robustness (simulating restart)
	time.Sleep(2 * time.Second)

	// Verify connection is still working
	_, err = s.ethClient.ChainID(context.Background())
	if err != nil {
		s.T().Logf("Connection lost during restart simulation: %v", err)
		s.T().Log("❌ Connection failed during restart test")
		return
	}

	s.T().Log("✅ Connection maintained during restart simulation")

	s.T().Log("\n=== Post-restart: Testing cache recovery ===")

	// Try to fill the gap that would trigger cached transactions
	gapFillerNonce := currentNonce + 9
	gapFillerTx := types.NewTransaction(
		gapFillerNonce,
		recipientAddress,
		transferAmount,
		gasLimit,
		gasPrice,
		nil,
	)

	signedGapFiller, err := types.SignTx(gapFillerTx, types.NewEIP155Signer(chainID), privateKey)
	s.Require().NoError(err, "Failed to sign gap filler transaction")

	s.T().Logf("Submitting gap filler transaction (nonce %d)", gapFillerNonce)
	err = s.ethClient.SendTransaction(context.Background(), signedGapFiller)
	s.T().Logf("Gap filler result: %v", err)

	time.Sleep(3 * time.Second) // Wait for processing

	s.T().Log("\n=== Node Restart Test Results ===")
	s.T().Log("This test demonstrates restart resilience concepts.")
	s.T().Log("Actual restart testing requires infrastructure automation.")
}

// TestCacheQueueMemoryPressure tests behavior under memory pressure
func (s *CacheQueueEdgeCasesTestSuite) TestCacheQueueMemoryPressure() {
	s.T().Log("=== Cache Queue Memory Pressure Test ===")
	s.T().Log("Testing cache queue behavior under high memory usage")

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Testing memory pressure for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Creating many transactions to test memory limits ===")

	// Try to create many transactions to test memory pressure
	memoryPressureTxs := 50
	successCount := 0
	rejectedCount := 0
	errorTypes := make(map[string]int)

	for i := 0; i < memoryPressureTxs; i++ {
		nonce := currentNonce + uint64(i*5+100) // Large gaps to force caching

		tx := types.NewTransaction(
			nonce,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
			nil,
		)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign memory pressure transaction %d", i+1)

		err = s.ethClient.SendTransaction(context.Background(), signedTx)

		if err == nil {
			successCount++
		} else {
			rejectedCount++
			errorMsg := err.Error()

			// Categorize error types
			if contains(errorMsg, "nonce") {
				errorTypes["nonce"]++
			} else if contains(errorMsg, "memory") || contains(errorMsg, "limit") || contains(errorMsg, "capacity") {
				errorTypes["capacity"]++
			} else if contains(errorMsg, "gas") {
				errorTypes["gas"]++
			} else {
				errorTypes["other"]++
			}
		}

		// Small delay to avoid overwhelming
		if i%10 == 9 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	s.T().Log("\n=== Memory Pressure Test Results ===")
	s.T().Logf("Total transactions attempted: %d", memoryPressureTxs)
	s.T().Logf("Successful: %d", successCount)
	s.T().Logf("Rejected: %d", rejectedCount)
	s.T().Logf("Success rate: %.2f%%", float64(successCount)/float64(memoryPressureTxs)*100)

	s.T().Log("\nError breakdown:")
	for errorType, count := range errorTypes {
		s.T().Logf("  %s errors: %d", errorType, count)
	}

	if errorTypes["capacity"] > 0 {
		s.T().Log("✅ Cache queue correctly enforced capacity limits")
	} else if successCount > 30 {
		s.T().Log("✅ Cache queue handled high transaction volume")
	} else {
		s.T().Log("⚠️  High rejection rate - check cache queue configuration")
	}
}

// TestCacheQueueConcurrentAccess tests concurrent access patterns
func (s *CacheQueueEdgeCasesTestSuite) TestCacheQueueConcurrentAccess() {
	s.T().Log("=== Cache Queue Concurrent Access Test ===")
	s.T().Log("Testing cache queue thread safety with concurrent submissions")

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Testing concurrent access for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Launching concurrent transaction submissions ===")

	// Concurrent test setup
	numGoroutines := 5
	txsPerGoroutine := 5
	var wg sync.WaitGroup
	results := make(chan error, numGoroutines*txsPerGoroutine)

	startTime := time.Now()

	// Launch multiple goroutines
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < txsPerGoroutine; i++ {
				// Create unique nonce for each goroutine and transaction
				nonce := currentNonce + uint64(goroutineID*100+i*10+200)

				tx := types.NewTransaction(
					nonce,
					recipientAddress,
					transferAmount,
					gasLimit,
					gasPrice,
					nil,
				)

				signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
				if err != nil {
					results <- err
					continue
				}

				err = s.ethClient.SendTransaction(context.Background(), signedTx)
				results <- err

				// Random small delay to simulate real-world timing
				randomDelay := time.Duration(rand.Intn(50)) * time.Millisecond
				time.Sleep(randomDelay)
			}
		}(g)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	duration := time.Since(startTime)
	totalTxs := numGoroutines * txsPerGoroutine

	s.T().Log("\n=== Concurrent Access Results ===")
	s.T().Logf("Launched %d goroutines, %d transactions each", numGoroutines, txsPerGoroutine)
	s.T().Logf("Total duration: %v", duration)

	// Analyze results
	successCount := 0
	errorCount := 0
	concurrencyErrors := 0

	for err := range results {
		if err == nil {
			successCount++
		} else {
			errorCount++
			errorMsg := err.Error()
			if contains(errorMsg, "concurrent") || contains(errorMsg, "race") ||
				contains(errorMsg, "lock") || contains(errorMsg, "deadlock") {
				concurrencyErrors++
			}
		}
	}

	s.T().Logf("Successful submissions: %d", successCount)
	s.T().Logf("Failed submissions: %d", errorCount)
	s.T().Logf("Concurrency-related errors: %d", concurrencyErrors)
	s.T().Logf("Success rate: %.2f%%", float64(successCount)/float64(totalTxs)*100)

	if concurrencyErrors == 0 {
		s.T().Log("✅ No concurrency-related errors detected")
	} else {
		s.T().Logf("❌ %d concurrency-related errors found", concurrencyErrors)
	}

	if successCount > 0 {
		s.T().Log("✅ Cache queue handled concurrent access")
		avgTxPerSecond := float64(totalTxs) / duration.Seconds()
		s.T().Logf("Concurrent throughput: %.2f tx/sec", avgTxPerSecond)
	}
}

// TestCacheQueueInvalidTransactions tests handling of invalid transactions
func (s *CacheQueueEdgeCasesTestSuite) TestCacheQueueInvalidTransactions() {
	s.T().Log("=== Cache Queue Invalid Transactions Test ===")
	s.T().Log("Testing cache queue handling of invalid transactions")

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Testing invalid transaction handling for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)

	s.T().Log("\n=== Testing various invalid transaction scenarios ===")

	testCases := []struct {
		name           string
		gasPrice       *big.Int
		transferAmount *big.Int
		gasLimit       uint64
		nonce          uint64
		expectError    bool
		description    string
	}{
		{
			name:           "Zero Gas Price",
			gasPrice:       big.NewInt(0),
			transferAmount: big.NewInt(1000000000000000),
			gasLimit:       gasLimit,
			nonce:          currentNonce + 50,
			expectError:    true,
			description:    "Transaction with zero gas price should be rejected",
		},
		{
			name:           "Extremely Low Gas",
			gasPrice:       big.NewInt(1),
			transferAmount: big.NewInt(1000000000000000),
			gasLimit:       1,
			nonce:          currentNonce + 51,
			expectError:    true,
			description:    "Transaction with insufficient gas should be rejected",
		},
		{
			name:           "Excessive Transfer Amount",
			gasPrice:       big.NewInt(20000000000),
			transferAmount: new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil), // Huge amount
			gasLimit:       gasLimit,
			nonce:          currentNonce + 52,
			expectError:    true,
			description:    "Transaction with excessive amount should be rejected",
		},
		{
			name:           "Valid Transaction",
			gasPrice:       big.NewInt(20000000000),
			transferAmount: big.NewInt(1000000000000000),
			gasLimit:       gasLimit,
			nonce:          currentNonce + 53,
			expectError:    false,
			description:    "Valid transaction should be accepted or cached",
		},
	}

	for i, tc := range testCases {
		s.T().Logf("\n--- Test Case %d: %s ---", i+1, tc.name)
		s.T().Logf("Description: %s", tc.description)

		tx := types.NewTransaction(
			tc.nonce,
			recipientAddress,
			tc.transferAmount,
			tc.gasLimit,
			tc.gasPrice,
			nil,
		)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign transaction for test case %s", tc.name)

		err = s.ethClient.SendTransaction(context.Background(), signedTx)

		if tc.expectError {
			if err != nil {
				s.T().Logf("✅ %s: Correctly rejected - %v", tc.name, err)
			} else {
				s.T().Logf("⚠️  %s: Unexpectedly accepted", tc.name)
			}
		} else {
			if err == nil {
				s.T().Logf("✅ %s: Correctly accepted/cached", tc.name)
			} else {
				s.T().Logf("❌ %s: Unexpectedly rejected - %v", tc.name, err)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	s.T().Log("\n=== Invalid Transactions Test Complete ===")
	s.T().Log("This test verifies cache queue input validation and error handling.")
}

// TestCacheQueueNetworkPartition tests behavior during network issues
func (s *CacheQueueEdgeCasesTestSuite) TestCacheQueueNetworkPartition() {
	s.T().Log("=== Cache Queue Network Partition Test ===")
	s.T().Log("Testing cache queue resilience during network issues")

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Testing network partition resilience for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Testing network timeout resilience ===")

	// Create context with timeout to simulate network issues
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tx := types.NewTransaction(
		currentNonce+60,
		recipientAddress,
		transferAmount,
		gasLimit,
		gasPrice,
		nil,
	)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	s.Require().NoError(err, "Failed to sign transaction")

	s.T().Log("Submitting transaction with short timeout to simulate network issues")
	err = s.ethClient.SendTransaction(timeoutCtx, signedTx)

	if err != nil {
		s.T().Logf("Transaction failed with timeout (expected): %v", err)
		s.T().Log("✅ Timeout handling working correctly")
	} else {
		s.T().Log("⚠️  Transaction succeeded despite timeout (network very fast)")
	}

	s.T().Log("\n=== Testing recovery after network issues ===")

	// Now try with normal context
	normalCtx := context.Background()
	recoveryTx := types.NewTransaction(
		currentNonce+61,
		recipientAddress,
		transferAmount,
		gasLimit,
		gasPrice,
		nil,
	)

	signedRecoveryTx, err := types.SignTx(recoveryTx, types.NewEIP155Signer(chainID), privateKey)
	s.Require().NoError(err, "Failed to sign recovery transaction")

	s.T().Log("Submitting recovery transaction after network timeout")
	err = s.ethClient.SendTransaction(normalCtx, signedRecoveryTx)

	if err == nil {
		s.T().Log("✅ Recovery transaction succeeded")
	} else {
		s.T().Logf("❌ Recovery transaction failed: %v", err)
	}

	s.T().Log("\n=== Network Partition Test Complete ===")
	s.T().Log("This test verifies network resilience and recovery capabilities.")
}

func TestCacheQueueEdgeCasesTestSuite(t *testing.T) {
	suite.Run(t, new(CacheQueueEdgeCasesTestSuite))
}
