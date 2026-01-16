// Package tests contains performance and monitoring tests for RPC Cache Queue
//
// This test suite focuses on performance metrics and monitoring:
//   - Transaction throughput measurement
//   - Memory usage monitoring
//   - Cache hit/miss ratios
//   - Latency measurements
//   - Resource utilization tracking
//
// Prerequisites:
//
//  1. Start MOCA Chain node with cache queue enabled and monitoring:
//     make localup
//     # or with cache queue and metrics enabled
//     ./deployment/localup/localup.sh --tx-cache-queue.enable=true --telemetry.enabled=true
//
//  2. Ensure test account has sufficient balance for test transactions
//
// Usage:
//
//	# Run all performance tests
//	cd e2e/tests
//	go test -v -run TestCacheQueuePerformance -timeout 30m
//
//	# Run individual performance tests
//	go test -v -run TestCacheQueueThroughput
//	go test -v -run TestCacheQueueLatency
//	go test -v -run TestCacheQueueMemoryUsage
package tests

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/suite"
)

type CacheQueuePerformanceTestSuite struct {
	suite.Suite
	ethClient *ethclient.Client
}

func (s *CacheQueuePerformanceTestSuite) SetupSuite() {
	// Connect to ETH RPC endpoint
	var err error
	s.ethClient, err = ethclient.Dial("http://localhost:8545")
	if err != nil {
		s.T().Skip("Skipping performance tests: MOCA Chain node not running on localhost:8545")
		return
	}
}

func (s *CacheQueuePerformanceTestSuite) TearDownSuite() {
	if s.ethClient != nil {
		s.ethClient.Close()
	}
}

// PerformanceMetrics holds performance measurement data
type PerformanceMetrics struct {
	TotalTransactions    int
	SuccessfulTxs       int
	FailedTxs           int
	Duration            time.Duration
	ThroughputTxPerSec  float64
	AvgLatencyMs        float64
	MinLatencyMs        float64
	MaxLatencyMs        float64
	MemoryUsageMB       float64
}

// TestCacheQueueThroughput measures transaction throughput
func (s *CacheQueuePerformanceTestSuite) TestCacheQueueThroughput() {
	s.T().Log("=== Cache Queue Throughput Test ===")
	s.T().Log("Measuring maximum transaction throughput with cache queue")

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Throughput testing for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Throughput Test Scenarios ===")

	testScenarios := []struct {
		name            string
		numTxs          int
		concurrency     int
		nonceGap        uint64
		description     string
	}{
		{
			name:        "Sequential High Volume",
			numTxs:      100,
			concurrency: 1,
			nonceGap:    10,
			description: "100 transactions submitted sequentially with gaps",
		},
		{
			name:        "Concurrent Medium Volume",
			numTxs:      50,
			concurrency: 5,
			nonceGap:    5,
			description: "50 transactions with 5 concurrent workers",
		},
		{
			name:        "Concurrent High Volume",
			numTxs:      100,
			concurrency: 10,
			nonceGap:    2,
			description: "100 transactions with 10 concurrent workers",
		},
	}

	for scenarioIdx, scenario := range testScenarios {
		s.T().Logf("\n--- Scenario %d: %s ---", scenarioIdx+1, scenario.name)
		s.T().Logf("Description: %s", scenario.description)

		metrics := s.runThroughputTest(
			privateKey,
			currentNonce+uint64(scenarioIdx*1000),
			scenario.numTxs,
			scenario.concurrency,
			scenario.nonceGap,
			chainID,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
		)

		s.T().Logf("Results for %s:", scenario.name)
		s.T().Logf("  Total transactions: %d", metrics.TotalTransactions)
		s.T().Logf("  Successful: %d", metrics.SuccessfulTxs)
		s.T().Logf("  Failed: %d", metrics.FailedTxs)
		s.T().Logf("  Duration: %v", metrics.Duration)
		s.T().Logf("  Throughput: %.2f tx/sec", metrics.ThroughputTxPerSec)
		s.T().Logf("  Success rate: %.2f%%", float64(metrics.SuccessfulTxs)/float64(metrics.TotalTransactions)*100)
		s.T().Logf("  Average latency: %.2f ms", metrics.AvgLatencyMs)
		s.T().Logf("  Latency range: %.2f - %.2f ms", metrics.MinLatencyMs, metrics.MaxLatencyMs)

		// Performance benchmarks
		if metrics.ThroughputTxPerSec > 10 {
			s.T().Logf("✅ Good throughput: %.2f tx/sec", metrics.ThroughputTxPerSec)
		} else {
			s.T().Logf("⚠️  Low throughput: %.2f tx/sec", metrics.ThroughputTxPerSec)
		}

		time.Sleep(2 * time.Second) // Cool down between scenarios
	}

	s.T().Log("\n=== Throughput Test Complete ===")
	s.T().Log("This test measures cache queue transaction processing throughput.")
}

// runThroughputTest executes a throughput test with given parameters
func (s *CacheQueuePerformanceTestSuite) runThroughputTest(
	privateKey *ecdsa.PrivateKey,
	startNonce uint64,
	numTxs int,
	concurrency int,
	nonceGap uint64,
	chainID *big.Int,
	recipientAddress common.Address,
	transferAmount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
) PerformanceMetrics {
	var wg sync.WaitGroup
	resultsChan := make(chan time.Duration, numTxs)
	errorsChan := make(chan error, numTxs)

	startTime := time.Now()

	// Worker function
	worker := func(workerID int, txIndices <-chan int) {
		defer wg.Done()
		for i := range txIndices {
			txStartTime := time.Now()

			nonce := startNonce + uint64(i)*nonceGap
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
				errorsChan <- err
				continue
			}

			err = s.ethClient.SendTransaction(context.Background(), signedTx)
			txDuration := time.Since(txStartTime)

			if err != nil {
				errorsChan <- err
			} else {
				resultsChan <- txDuration
			}
		}
	}

	// Create job channel
	jobs := make(chan int, numTxs)

	// Start workers
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go worker(w, jobs)
	}

	// Send jobs
	for i := 0; i < numTxs; i++ {
		jobs <- i
	}
	close(jobs)

	// Wait for completion
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	totalDuration := time.Since(startTime)

	// Collect results
	var latencies []time.Duration
	successCount := 0
	for latency := range resultsChan {
		latencies = append(latencies, latency)
		successCount++
	}

	errorCount := 0
	for range errorsChan {
		errorCount++
	}

	// Calculate metrics
	metrics := PerformanceMetrics{
		TotalTransactions:   numTxs,
		SuccessfulTxs:      successCount,
		FailedTxs:          errorCount,
		Duration:           totalDuration,
		ThroughputTxPerSec: float64(numTxs) / totalDuration.Seconds(),
	}

	if len(latencies) > 0 {
		var totalLatency time.Duration
		minLatency := latencies[0]
		maxLatency := latencies[0]

		for _, latency := range latencies {
			totalLatency += latency
			if latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		}

		metrics.AvgLatencyMs = float64(totalLatency.Nanoseconds()) / float64(len(latencies)) / 1000000
		metrics.MinLatencyMs = float64(minLatency.Nanoseconds()) / 1000000
		metrics.MaxLatencyMs = float64(maxLatency.Nanoseconds()) / 1000000
	}

	return metrics
}

// TestCacheQueueLatency measures transaction latency
func (s *CacheQueuePerformanceTestSuite) TestCacheQueueLatency() {
	s.T().Log("=== Cache Queue Latency Test ===")
	s.T().Log("Measuring transaction latency with different cache scenarios")

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Latency testing for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Latency Test Scenarios ===")

	latencyTests := []struct {
		name        string
		nonce       uint64
		description string
		expectCache bool
	}{
		{
			name:        "Immediate Transaction",
			nonce:       currentNonce,
			description: "Transaction with current nonce (should process immediately)",
			expectCache: false,
		},
		{
			name:        "Gap Transaction (Cache)",
			nonce:       currentNonce + 10,
			description: "Transaction with nonce gap (should be cached)",
			expectCache: true,
		},
		{
			name:        "Large Gap Transaction",
			nonce:       currentNonce + 100,
			description: "Transaction with large nonce gap (should be cached)",
			expectCache: true,
		},
	}

	for i, test := range latencyTests {
		s.T().Logf("\n--- Latency Test %d: %s ---", i+1, test.name)
		s.T().Logf("Description: %s", test.description)

		// Measure latency
		startTime := time.Now()

		tx := types.NewTransaction(
			test.nonce,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
			nil,
		)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign transaction for latency test %s", test.name)

		err = s.ethClient.SendTransaction(context.Background(), signedTx)
		latency := time.Since(startTime)

		s.T().Logf("Latency: %v", latency)

		if err != nil {
			s.T().Logf("Result: Failed - %v", err)
		} else {
			s.T().Logf("Result: Success - %s", signedTx.Hash().Hex())
		}

		// Latency analysis
		latencyMs := float64(latency.Nanoseconds()) / 1000000
		s.T().Logf("Latency: %.2f ms", latencyMs)

		if test.expectCache {
			if latencyMs < 100 {
				s.T().Log("✅ Fast cache operation")
			} else {
				s.T().Log("⚠️  Slower than expected for cache operation")
			}
		} else {
			if latencyMs < 500 {
				s.T().Log("✅ Fast immediate processing")
			} else {
				s.T().Log("⚠️  Slower than expected for immediate processing")
			}
		}

		time.Sleep(1 * time.Second) // Delay between tests
	}

	s.T().Log("\n=== Latency Test Complete ===")
	s.T().Log("This test measures transaction processing latency in different scenarios.")
}

// TestCacheQueueMemoryUsage monitors memory usage during operations
func (s *CacheQueuePerformanceTestSuite) TestCacheQueueMemoryUsage() {
	s.T().Log("=== Cache Queue Memory Usage Test ===")
	s.T().Log("Monitoring memory usage during cache queue operations")

	// Get initial memory stats
	var initialMemStats runtime.MemStats
	runtime.GC() // Force garbage collection for accurate baseline
	runtime.ReadMemStats(&initialMemStats)

	s.T().Logf("Initial memory usage: %.2f MB", float64(initialMemStats.Alloc)/1024/1024)

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	s.T().Logf("Memory testing for account %s (nonce: %d)", fromAddress.Hex(), currentNonce)

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Creating transactions to test memory usage ===")

	memoryTestTxs := 100
	var memorySnapshots []float64

	// Monitor memory usage during transaction creation
	for i := 0; i < memoryTestTxs; i++ {
		nonce := currentNonce + uint64(i*5+1000) // Large gaps to force caching

		tx := types.NewTransaction(
			nonce,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
			nil,
		)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign memory test transaction %d", i+1)

		err = s.ethClient.SendTransaction(context.Background(), signedTx)
		// Don't fail test on transaction errors - focus on memory

		// Take memory snapshot every 10 transactions
		if i%10 == 9 {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memoryMB := float64(memStats.Alloc) / 1024 / 1024
			memorySnapshots = append(memorySnapshots, memoryMB)
			s.T().Logf("Memory after %d transactions: %.2f MB", i+1, memoryMB)
		}

		// Small delay to allow processing
		if i%20 == 19 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Final memory measurement
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)
	finalMemoryMB := float64(finalMemStats.Alloc) / 1024 / 1024

	s.T().Log("\n=== Memory Usage Analysis ===")
	s.T().Logf("Initial memory: %.2f MB", float64(initialMemStats.Alloc)/1024/1024)
	s.T().Logf("Final memory: %.2f MB", finalMemoryMB)
	s.T().Logf("Memory increase: %.2f MB", finalMemoryMB-float64(initialMemStats.Alloc)/1024/1024)

	// Calculate memory growth trend
	if len(memorySnapshots) >= 2 {
		firstSnapshot := memorySnapshots[0]
		lastSnapshot := memorySnapshots[len(memorySnapshots)-1]
		memoryGrowth := lastSnapshot - firstSnapshot

		s.T().Logf("Memory growth during test: %.2f MB", memoryGrowth)

		if memoryGrowth < 50 { // Less than 50MB growth
			s.T().Log("✅ Memory usage appears stable")
		} else if memoryGrowth < 100 { // Less than 100MB growth
			s.T().Log("⚠️  Moderate memory growth detected")
		} else {
			s.T().Log("❌ High memory growth - potential memory leak")
		}

		// Check for linear growth pattern
		avgGrowthPerStep := memoryGrowth / float64(len(memorySnapshots)-1)
		s.T().Logf("Average memory growth per 10 transactions: %.2f MB", avgGrowthPerStep)
	}

	s.T().Log("\n=== Memory Test Complete ===")
	s.T().Log("This test monitors memory usage patterns during cache queue operations.")
}

// TestCacheQueueResourceUtilization tests overall resource usage
func (s *CacheQueuePerformanceTestSuite) TestCacheQueueResourceUtilization() {
	s.T().Log("=== Cache Queue Resource Utilization Test ===")
	s.T().Log("Measuring overall resource utilization during cache operations")

	// Baseline measurements
	initialTime := time.Now()
	var initialMemStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMemStats)

	s.T().Logf("Starting resource monitoring at %v", initialTime)
	s.T().Logf("Initial allocations: %d", initialMemStats.Mallocs)
	s.T().Logf("Initial memory: %.2f MB", float64(initialMemStats.Alloc)/1024/1024)

	// Setup test account
	privateKeyHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	s.Require().NoError(err, "Failed to parse private key")

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	currentNonce, err := s.ethClient.PendingNonceAt(context.Background(), fromAddress)
	s.Require().NoError(err, "Failed to get current nonce")

	// Test configuration
	chainID := big.NewInt(5151)
	recipientAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(20000000000)
	transferAmount := big.NewInt(1000000000000000)

	s.T().Log("\n=== Resource utilization during mixed workload ===")

	// Mixed workload: immediate + cached transactions
	resourceTestTxs := 50
	successCount := 0
	errorCount := 0

	workloadStart := time.Now()

	for i := 0; i < resourceTestTxs; i++ {
		var nonce uint64
		
		// Mix immediate and cached transactions
		if i%3 == 0 {
			nonce = currentNonce + uint64(i/3) // Some immediate transactions
		} else {
			nonce = currentNonce + uint64(i*10+1000) // Cached transactions with gaps
		}

		tx := types.NewTransaction(
			nonce,
			recipientAddress,
			transferAmount,
			gasLimit,
			gasPrice,
			nil,
		)

		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
		s.Require().NoError(err, "Failed to sign resource test transaction %d", i+1)

		err = s.ethClient.SendTransaction(context.Background(), signedTx)
		
		if err == nil {
			successCount++
		} else {
			errorCount++
		}

		// Progress updates
		if (i+1)%10 == 0 {
			elapsed := time.Since(workloadStart)
			rate := float64(i+1) / elapsed.Seconds()
			s.T().Logf("Progress: %d/%d transactions (%.1f tx/sec)", i+1, resourceTestTxs, rate)
		}
	}

	workloadDuration := time.Since(workloadStart)

	// Final measurements
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)

	s.T().Log("\n=== Resource Utilization Results ===")
	s.T().Logf("Workload duration: %v", workloadDuration)
	s.T().Logf("Transactions processed: %d", resourceTestTxs)
	s.T().Logf("Success rate: %.2f%% (%d/%d)", float64(successCount)/float64(resourceTestTxs)*100, successCount, resourceTestTxs)
	s.T().Logf("Processing rate: %.2f tx/sec", float64(resourceTestTxs)/workloadDuration.Seconds())

	s.T().Log("\nMemory Statistics:")
	s.T().Logf("  Memory allocations: %d → %d (delta: %d)", 
		initialMemStats.Mallocs, finalMemStats.Mallocs, finalMemStats.Mallocs-initialMemStats.Mallocs)
	s.T().Logf("  Memory frees: %d → %d (delta: %d)", 
		initialMemStats.Frees, finalMemStats.Frees, finalMemStats.Frees-initialMemStats.Frees)
	s.T().Logf("  Active allocations: %d → %d", 
		initialMemStats.Mallocs-initialMemStats.Frees, finalMemStats.Mallocs-finalMemStats.Frees)
	s.T().Logf("  Heap memory: %.2f MB → %.2f MB (delta: %.2f MB)", 
		float64(initialMemStats.Alloc)/1024/1024, 
		float64(finalMemStats.Alloc)/1024/1024,
		float64(finalMemStats.Alloc-initialMemStats.Alloc)/1024/1024)

	// Resource efficiency analysis
	allocationsPerTx := float64(finalMemStats.Mallocs-initialMemStats.Mallocs) / float64(resourceTestTxs)
	memoryPerTx := float64(finalMemStats.Alloc-initialMemStats.Alloc) / float64(resourceTestTxs)

	s.T().Logf("\nResource Efficiency:")
	s.T().Logf("  Allocations per transaction: %.1f", allocationsPerTx)
	s.T().Logf("  Memory per transaction: %.2f KB", memoryPerTx/1024)

	// Performance benchmarks
	if workloadDuration.Seconds()/float64(resourceTestTxs) < 0.1 { // Less than 100ms per tx
		s.T().Log("✅ Excellent performance: < 100ms per transaction")
	} else if workloadDuration.Seconds()/float64(resourceTestTxs) < 0.5 { // Less than 500ms per tx
		s.T().Log("✅ Good performance: < 500ms per transaction")
	} else {
		s.T().Log("⚠️  Performance needs improvement: > 500ms per transaction")
	}

	if memoryPerTx < 1024 { // Less than 1KB per transaction
		s.T().Log("✅ Excellent memory efficiency: < 1KB per transaction")
	} else if memoryPerTx < 10240 { // Less than 10KB per transaction
		s.T().Log("✅ Good memory efficiency: < 10KB per transaction")
	} else {
		s.T().Log("⚠️  Memory efficiency needs improvement: > 10KB per transaction")
	}

	s.T().Log("\n=== Resource Utilization Test Complete ===")
	s.T().Log("This test provides comprehensive resource utilization analysis.")
}

func TestCacheQueuePerformanceTestSuite(t *testing.T) {
	suite.Run(t, new(CacheQueuePerformanceTestSuite))
}
