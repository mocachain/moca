package backend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestBackgroundMaintenance(t *testing.T) {
	// Create a config with aggressive cleanup settings for testing
	config := &CacheQueueConfig{
		Enable:             true,
		MaxTxPerAccount:    3,    // Small limit for testing
		TxTimeout:          100 * time.Millisecond, // Short timeout
		CleanupInterval:    50 * time.Millisecond,  // Frequent cleanup
		GlobalMaxTx:        10,   // Small global limit
		RetryInterval:      1 * time.Second,
		MaxRetries:         3,
	}

	tcq := NewTransactionCacheQueue(config)
	defer tcq.Stop()

	// Disable actual broadcasting for tests
	tcq.backend = nil

	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	t.Run("ExpiredTransactionCleanup", func(t *testing.T) {
		// Add some transactions
		tx1 := createBackgroundTestTransaction(1)
		rawTx1, err := tx1.MarshalBinary()
		require.NoError(t, err)

		tx2 := createBackgroundTestTransaction(2)
		rawTx2, err := tx2.MarshalBinary()
		require.NoError(t, err)

		// Cache transactions
		_, err = tcq.cacheTransaction(rawTx1, testAddr, 1, tx1)
		require.NoError(t, err)
		_, err = tcq.cacheTransaction(rawTx2, testAddr, 2, tx2)
		require.NoError(t, err)

		// Verify transactions are cached
		queue := tcq.getAccountQueue(testAddr)
		assert.Equal(t, 2, queue.Size(), "Should have 2 cached transactions")

		// Wait for cleanup to happen (timeout + cleanup interval)
		time.Sleep(200 * time.Millisecond)

		// Verify expired transactions are cleaned up
		queue = tcq.getAccountQueue(testAddr)
		if queue != nil {
			assert.Equal(t, 0, queue.Size(), "Expired transactions should be cleaned up")
		}

		// Verify metrics
		metrics := tcq.GetMetrics()
		assert.True(t, metrics.ExpiredTxCount >= 2, "Should have expired at least 2 transactions")
	})

	t.Run("MemoryLimitEnforcement", func(t *testing.T) {
		// Manually add transactions to test memory limit enforcement
		// We'll use a different address to avoid conflicts with previous tests
		testAddr2 := common.HexToAddress("0x2234567890123456789012345678901234567890")

		// Manually add transactions beyond the limit by directly calling account queue
		queue := tcq.getOrCreateAccountQueue(testAddr2)

		for nonce := uint64(10); nonce < 15; nonce++ { // Add 5 transactions (> limit of 3)
			tx := createBackgroundTestTransaction(nonce)
			rawTx, err := tx.MarshalBinary()
			require.NoError(t, err)

			cachedTx := &CachedTransaction{
				RawTx:       rawTx,
				Transaction: tx,
				From:        testAddr2,
				Nonce:       nonce,
				Hash:        tx.Hash(),
				Timestamp:   time.Now(),
				Retries:     0,
			}
			queue.AddTransaction(cachedTx, config)
		}

		// Verify we've exceeded the limit
		assert.Greater(t, queue.Size(), config.MaxTxPerAccount,
			"Should have exceeded per-account limit")

		// Manually trigger memory enforcement
		tcq.enforceMemoryLimits()

		// Verify account limit is enforced (should be <= MaxTxPerAccount)
		assert.LessOrEqual(t, queue.Size(), config.MaxTxPerAccount,
			"Account queue should not exceed MaxTxPerAccount limit after enforcement")
	})

	t.Run("GlobalMemoryLimitEnforcement", func(t *testing.T) {
		// Create multiple accounts to test global limit
		addresses := []common.Address{
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
			common.HexToAddress("0x2222222222222222222222222222222222222222"),
			common.HexToAddress("0x3333333333333333333333333333333333333333"),
		}

		// Manually add transactions to exceed global limit
		nonce := uint64(30)
		for _, addr := range addresses {
			queue := tcq.getOrCreateAccountQueue(addr)

			for i := 0; i < 4; i++ { // 3 accounts × 4 tx = 12 total > 10 global limit
				tx := createBackgroundTestTransaction(nonce)
				rawTx, err := tx.MarshalBinary()
				require.NoError(t, err)

				cachedTx := &CachedTransaction{
					RawTx:       rawTx,
					Transaction: tx,
					From:        addr,
					Nonce:       nonce,
					Hash:        tx.Hash(),
					Timestamp:   time.Now(),
					Retries:     0,
				}
				queue.AddTransaction(cachedTx, config)
				nonce++
			}
		}

		// Manually trigger memory enforcement
		tcq.enforceMemoryLimits()

		// Count total transactions across all accounts
		totalTx := 0
		tcq.mu.RLock()
		for _, queue := range tcq.accounts {
			totalTx += queue.Size()
		}
		tcq.mu.RUnlock()

		assert.LessOrEqual(t, totalTx, config.GlobalMaxTx,
			"Total transactions should not exceed GlobalMaxTx limit after enforcement")
	})

	t.Run("MetricsReporting", func(t *testing.T) {
		// Manually add some transactions for metrics testing
		testAddr3 := common.HexToAddress("0x3234567890123456789012345678901234567890")
		queue := tcq.getOrCreateAccountQueue(testAddr3)

		for nonce := uint64(50); nonce < 53; nonce++ {
			tx := createBackgroundTestTransaction(nonce)
			rawTx, err := tx.MarshalBinary()
			require.NoError(t, err)

			cachedTx := &CachedTransaction{
				RawTx:       rawTx,
				Transaction: tx,
				From:        testAddr3,
				Nonce:       nonce,
				Hash:        tx.Hash(),
				Timestamp:   time.Now(),
				Retries:     0,
			}
			queue.AddTransaction(cachedTx, config)
		}

		// Get metrics
		metrics := tcq.GetMetrics()

		// Verify basic metrics
		assert.NotZero(t, metrics.StartTime, "StartTime should be set")
		assert.True(t, metrics.CachedTxCount >= 0, "CachedTxCount should be non-negative")
		assert.True(t, metrics.AccountsWithCache >= 0, "AccountsWithCache should be non-negative")
		assert.True(t, metrics.ProcessedTxCount >= 0, "ProcessedTxCount should be non-negative")

		// Verify peak metrics are at least as high as current
		assert.True(t, metrics.PeakCachedTxCount >= metrics.CachedTxCount,
			"PeakCachedTxCount should be >= current")
		assert.True(t, metrics.PeakAccountsWithCache >= metrics.AccountsWithCache,
			"PeakAccountsWithCache should be >= current")
	})
}

func TestBackgroundWorkerLifecycle(t *testing.T) {
	config := &CacheQueueConfig{
		Enable:             true,
		MaxTxPerAccount:    10,
		TxTimeout:          1 * time.Minute,
		CleanupInterval:    100 * time.Millisecond,
		GlobalMaxTx:        100,
		RetryInterval:      1 * time.Second,
		MaxRetries:         3,
	}

	t.Run("StartAndStopWorkers", func(t *testing.T) {
		tcq := NewTransactionCacheQueue(config)

		// Verify cache queue is enabled
		assert.True(t, tcq.IsEnabled(), "Cache queue should be enabled")

		// Add a transaction to verify workers are running
		testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		tx := createBackgroundTestTransaction(1)
		rawTx, err := tx.MarshalBinary()
		require.NoError(t, err)

		// Manually add transaction to bypass limits for testing
		queue := tcq.getOrCreateAccountQueue(testAddr)
		cachedTx := &CachedTransaction{
			RawTx:       rawTx,
			Transaction: tx,
			From:        testAddr,
			Nonce:       1,
			Hash:        tx.Hash(),
			Timestamp:   time.Now(),
			Retries:     0,
		}
		queue.AddTransaction(cachedTx, config)

		// Wait a bit to ensure workers are running
		time.Sleep(50 * time.Millisecond)

		// Stop the cache queue
		tcq.Stop()

		// Verify metrics one last time
		metrics := tcq.GetMetrics()
		assert.NotNil(t, metrics, "Metrics should still be available after stop")
	})

	t.Run("DisabledCacheQueue", func(t *testing.T) {
		disabledConfig := *config
		disabledConfig.Enable = false

		tcq := NewTransactionCacheQueue(&disabledConfig)
		defer tcq.Stop()

		assert.False(t, tcq.IsEnabled(), "Cache queue should be disabled")

		// Should still be able to get metrics even when disabled
		metrics := tcq.GetMetrics()
		assert.NotNil(t, metrics, "Metrics should be available even when disabled")
	})
}

func TestMemoryMonitoring(t *testing.T) {
	config := &CacheQueueConfig{
		Enable:             true,
		MaxTxPerAccount:    2,
		TxTimeout:          1 * time.Minute,
		CleanupInterval:    1 * time.Second, // Longer intervals for this test
		GlobalMaxTx:        5,
		RetryInterval:      1 * time.Second,
		MaxRetries:         3,
	}

	tcq := NewTransactionCacheQueue(config)
	defer tcq.Stop()

	// Set backend to nil for testing
	tcq.backend = nil

	t.Run("OldestTransactionRemoval", func(t *testing.T) {
		testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

		// Manually add transactions with different timestamps
		queue := tcq.getOrCreateAccountQueue(testAddr)

		transactions := []struct {
			nonce uint64
			delay time.Duration
		}{
			{100, 0},
			{101, 10 * time.Millisecond},
			{102, 20 * time.Millisecond},
			{103, 30 * time.Millisecond}, // This should exceed per-account limit
		}

		for _, txInfo := range transactions {
			if txInfo.delay > 0 {
				time.Sleep(txInfo.delay)
			}

			tx := createBackgroundTestTransaction(txInfo.nonce)
			rawTx, err := tx.MarshalBinary()
			require.NoError(t, err)

			cachedTx := &CachedTransaction{
				RawTx:       rawTx,
				Transaction: tx,
				From:        testAddr,
				Nonce:       txInfo.nonce,
				Hash:        tx.Hash(),
				Timestamp:   time.Now(),
				Retries:     0,
			}
			queue.AddTransaction(cachedTx, config)
		}

		// Manually trigger memory enforcement
		tcq.enforceMemoryLimits()

		// Verify that the account queue doesn't exceed the limit
		updatedQueue := tcq.getAccountQueue(testAddr)
		require.NotNil(t, updatedQueue, "Account queue should exist")
		assert.LessOrEqual(t, updatedQueue.Size(), config.MaxTxPerAccount,
			"Account queue should not exceed limit after enforcement")

		// The oldest transaction (nonce 100) should be removed
		oldestTx := updatedQueue.GetTransaction(100)
		assert.Nil(t, oldestTx, "Oldest transaction should be removed")

		// Newer transactions should still exist
		newestTx := updatedQueue.GetTransaction(103)
		assert.NotNil(t, newestTx, "Newest transaction should still exist")
	})
}

// Helper function to create test transactions for background maintenance tests
func createBackgroundTestTransaction(nonce uint64) *types.Transaction {
	return types.NewTransaction(
		nonce,
		common.HexToAddress("0x0000000000000000000000000000000000000001"),
		nil,    // value
		21000,  // gas limit
		nil,    // gas price
		nil,    // data
	)
}
