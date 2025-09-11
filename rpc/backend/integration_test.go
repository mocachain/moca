package backend

import (
	"testing"
	"time"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/assert"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TestCacheQueueIntegration tests the complete cache queue system integration
func TestCacheQueueIntegration(t *testing.T) {
	// Quick integration test with minimal delays
	config := &CacheQueueConfig{
		Enable:          true,
		MaxTxPerAccount: 5,
		TxTimeout:       100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
		GlobalMaxTx:     20,
		RetryInterval:   10 * time.Millisecond,
		MaxRetries:      2,
	}

	tcq := NewTransactionCacheQueue(config)
	defer tcq.Stop()

	// Disable backend for testing
	tcq.backend = nil

	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	t.Run("CompleteWorkflow", func(t *testing.T) {
		// Step 1: Cache out-of-order transaction (nonce 2)
		tx2 := createIntegrationTestTransaction(2)
		queue := tcq.getOrCreateAccountQueue(testAddr)
		cachedTx2 := &CachedTransaction{
			RawTx:       mustMarshalTx(tx2),
			Transaction: tx2,
			From:        testAddr,
			Nonce:       2,
			Hash:        tx2.Hash(),
			Timestamp:   time.Now(),
			Retries:     0,
		}
		queue.AddTransaction(cachedTx2, config)

		// Verify transaction is cached
		assert.Equal(t, 1, queue.Size(), "Should have 1 cached transaction")

		// Step 2: Simulate nonce polling cleaner processing (nonce 1 completed)
		// In the new implementation, this would be handled by periodic nonce polling
		queue.UpdateCurrentNonce(2)
		consecutiveTxs := queue.FindConsecutiveTransactions(2)
		if len(consecutiveTxs) > 0 {
			// Simulate batch broadcasting and removal
			queue.RemoveTransactions(consecutiveTxs)
		}

		// Should trigger processing of nonce 2
		assert.Equal(t, 0, queue.Size(), "Transaction should be processed and removed")

		// Verify metrics
		metrics := tcq.GetMetrics()
		assert.True(t, metrics.ProcessedTxCount >= 1, "Should have processed at least 1 transaction")
	})

	t.Run("MultipleAccountsWorkflow", func(t *testing.T) {
		// Test with multiple accounts
		accounts := []common.Address{
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
			common.HexToAddress("0x2222222222222222222222222222222222222222"),
		}

		// Cache transactions for each account
		for i, addr := range accounts {
			queue := tcq.getOrCreateAccountQueue(addr)
			nonce := uint64(10 + i)
			
			tx := createIntegrationTestTransaction(nonce)
			cachedTx := &CachedTransaction{
				RawTx:       mustMarshalTx(tx),
				Transaction: tx,
				From:        addr,
				Nonce:       nonce,
				Hash:        tx.Hash(),
				Timestamp:   time.Now(),
				Retries:     0,
			}
			queue.AddTransaction(cachedTx, config)
		}

		// Verify both accounts have cached transactions
		tcq.mu.RLock()
		assert.Equal(t, len(accounts), len(tcq.accounts), "Should have queues for all accounts")
		tcq.mu.RUnlock()

		// Simulate nonce polling cleaner processing for all accounts
		for i, addr := range accounts {
			queue := tcq.getAccountQueue(addr)
			if queue != nil {
				// Simulate current nonce being the cached transaction nonce
				queue.UpdateCurrentNonce(uint64(10 + i))
				consecutiveTxs := queue.FindConsecutiveTransactions(uint64(10 + i))
				if len(consecutiveTxs) > 0 {
					queue.RemoveTransactions(consecutiveTxs)
				}
			}
		}

		// Verify all transactions processed
		tcq.mu.RLock()
		for _, addr := range accounts {
			queue := tcq.accounts[addr]
			if queue != nil {
				assert.Equal(t, 0, queue.Size(), "All transactions should be processed")
			}
		}
		tcq.mu.RUnlock()
	})

	t.Run("ExpiredTransactionCleanup", func(t *testing.T) {
		// Add a transaction that will expire quickly
		addr := common.HexToAddress("0x3333333333333333333333333333333333333333")
		queue := tcq.getOrCreateAccountQueue(addr)
		
		tx := createIntegrationTestTransaction(100)
		cachedTx := &CachedTransaction{
			RawTx:       mustMarshalTx(tx),
			Transaction: tx,
			From:        addr,
			Nonce:       100,
			Hash:        tx.Hash(),
			Timestamp:   time.Now().Add(-200 * time.Millisecond), // Make it expired
			Retries:     0,
		}
		queue.AddTransaction(cachedTx, config)

		// Wait for cleanup to happen
		time.Sleep(150 * time.Millisecond)

		// Verify expired transaction is cleaned up
		updatedQueue := tcq.getAccountQueue(addr)
		if updatedQueue != nil {
			assert.Equal(t, 0, updatedQueue.Size(), "Expired transaction should be cleaned up")
		}

		// Verify cleanup metrics
		metrics := tcq.GetMetrics()
		assert.True(t, metrics.ExpiredTxCount >= 1, "Should have expired at least 1 transaction")
	})
}

// TestMetricsIntegration tests the metrics and monitoring system
func TestMetricsIntegration(t *testing.T) {
	config := &CacheQueueConfig{
		Enable:          true,
		MaxTxPerAccount: 3,
		TxTimeout:       1 * time.Minute,
		CleanupInterval: 1 * time.Second,
		GlobalMaxTx:     10,
		RetryInterval:   100 * time.Millisecond,
		MaxRetries:      2,
	}

	tcq := NewTransactionCacheQueue(config)
	defer tcq.Stop()

	// Test metrics handler
	metricsHandler := NewMetricsHandler(tcq, log.NewNopLogger())
	
	t.Run("MetricsCollection", func(t *testing.T) {
		// Add some test data
		testAddr := common.HexToAddress("0x4444444444444444444444444444444444444444")
		queue := tcq.getOrCreateAccountQueue(testAddr)
		
		for nonce := uint64(1); nonce <= 2; nonce++ {
			tx := createIntegrationTestTransaction(nonce)
			cachedTx := &CachedTransaction{
				RawTx:       mustMarshalTx(tx),
				Transaction: tx,
				From:        testAddr,
				Nonce:       nonce,
				Hash:        tx.Hash(),
				Timestamp:   time.Now(),
				Retries:     0,
			}
			queue.AddTransaction(cachedTx, config)
		}

		// Test metrics
		metrics := tcq.GetMetrics()
		assert.True(t, metrics.CachedTxCount >= 2, "Should have at least 2 cached transactions")
		assert.True(t, metrics.AccountsWithCache >= 1, "Should have at least 1 account")
		assert.NotZero(t, metrics.StartTime, "StartTime should be set")

		// Test Prometheus metrics
		prometheusMetrics := metricsHandler.PrometheusMetrics()
		assert.Contains(t, prometheusMetrics, "moca_cache_queue_cached_transactions", "Should contain cached transactions metric")
		assert.Contains(t, prometheusMetrics, "moca_cache_queue_active_accounts", "Should contain active accounts metric")
	})
}

// TestConfigIntegration tests the configuration system
func TestConfigIntegration(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		appConfig := DefaultCacheQueueAppConfig()
		
		assert.True(t, appConfig.Cache.Enable, "Should be enabled by default")
		assert.Equal(t, 10, appConfig.Cache.MaxTxPerAccount, "Should have default max tx per account")
		assert.Equal(t, 5*time.Minute, appConfig.Cache.TxTimeout, "Should have default timeout")
		assert.Equal(t, 1000, appConfig.Cache.GlobalMaxTx, "Should have default global max")
		
		// Convert to internal config
		cacheConfig := appConfig.ToCacheQueueConfig()
		assert.Equal(t, appConfig.Cache.Enable, cacheConfig.Enable)
		assert.Equal(t, appConfig.Cache.MaxTxPerAccount, cacheConfig.MaxTxPerAccount)
	})

	t.Run("CustomConfig", func(t *testing.T) {
		customConfig := &CacheQueueConfig{
			Enable:          false,
			MaxTxPerAccount: 20,
			TxTimeout:       10 * time.Minute,
			CleanupInterval: 1 * time.Minute,
			GlobalMaxTx:     2000,
			RetryInterval:   2 * time.Second,
			MaxRetries:      5,
		}

		tcq := NewTransactionCacheQueue(customConfig)
		defer tcq.Stop()

		assert.False(t, tcq.IsEnabled(), "Should respect custom enable setting")
		assert.Equal(t, 20, tcq.config.MaxTxPerAccount, "Should use custom max tx per account")
		assert.Equal(t, 2000, tcq.config.GlobalMaxTx, "Should use custom global max")
	})
}

// Helper functions for integration tests
func createIntegrationTestTransaction(nonce uint64) *types.Transaction {
	return types.NewTransaction(
		nonce,
		common.HexToAddress("0x0000000000000000000000000000000000000001"),
		nil,    // value
		21000,  // gas limit
		nil,    // gas price
		nil,    // data
	)
}

func mustMarshalTx(tx *types.Transaction) []byte {
	data, err := tx.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return data
}
