package backend

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNonceSet(t *testing.T) {
	ns := NewNonceSet()

	// Test adding values
	ns.Add(5)
	ns.Add(3)
	ns.Add(7)
	ns.Add(1)
	ns.Add(3) // Duplicate should be ignored

	assert.Equal(t, 4, ns.Size(), "Size should be 4 (no duplicates)")

	// Test range from
	values := ns.RangeFrom(3)
	expected := []uint64{3, 5, 7}
	assert.Equal(t, expected, values, "RangeFrom(3) should return [3, 5, 7]")

	// Test remove
	ns.Remove(5)
	values = ns.RangeFrom(0)
	expected = []uint64{1, 3, 7}
	assert.Equal(t, expected, values, "After removing 5, should have [1, 3, 7]")
}

func TestAccountQueue(t *testing.T) {
	queue := NewAccountQueue()
	config := DefaultCacheQueueConfig()

	// Create test address
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Create test transactions
	tx1 := createTestCachedTransaction(testAddr, 1)
	tx2 := createTestCachedTransaction(testAddr, 3)
	tx3 := createTestCachedTransaction(testAddr, 4)
	tx5 := createTestCachedTransaction(testAddr, 6)

	// Add transactions
	require.NoError(t, queue.AddTransaction(tx1, config))
	require.NoError(t, queue.AddTransaction(tx2, config))
	require.NoError(t, queue.AddTransaction(tx3, config))
	require.NoError(t, queue.AddTransaction(tx5, config))

	assert.Equal(t, 4, queue.Size(), "Queue should have 4 transactions")

	// Test consecutive transaction finding
	consecutive := queue.FindConsecutiveTransactions(1)
	assert.Equal(t, 1, len(consecutive), "Should find 1 consecutive transaction starting from nonce 1")
	assert.Equal(t, uint64(1), consecutive[0].Nonce, "First consecutive should be nonce 1")

	consecutive = queue.FindConsecutiveTransactions(3)
	assert.Equal(t, 2, len(consecutive), "Should find 2 consecutive transactions starting from nonce 3")
	assert.Equal(t, uint64(3), consecutive[0].Nonce, "First consecutive should be nonce 3")
	assert.Equal(t, uint64(4), consecutive[1].Nonce, "Second consecutive should be nonce 4")

	consecutive = queue.FindConsecutiveTransactions(6)
	assert.Equal(t, 1, len(consecutive), "Should find 1 consecutive transaction starting from nonce 6")
	assert.Equal(t, uint64(6), consecutive[0].Nonce, "Consecutive should be nonce 6")

	consecutive = queue.FindConsecutiveTransactions(2)
	assert.Equal(t, 0, len(consecutive), "Should find no consecutive transactions starting from nonce 2 (gap)")

	// Test removing transactions
	removed := queue.RemoveTransaction(3)
	require.NotNil(t, removed, "Should have removed transaction with nonce 3")
	assert.Equal(t, uint64(3), removed.Nonce, "Removed transaction should have nonce 3")

	assert.Equal(t, 3, queue.Size(), "Queue should have 3 transactions after removal")

	// Test batch removal
	toRemove := []*CachedTransaction{tx1, tx5}
	queue.RemoveTransactions(toRemove)
	assert.Equal(t, 1, queue.Size(), "Queue should have 1 transaction after batch removal")

	// Verify only tx3 remains (which was re-added implicitly, should be tx4)
	remaining := queue.GetTransaction(4)
	require.NotNil(t, remaining, "Transaction with nonce 4 should still exist")
}

func TestTransactionCacheQueue(t *testing.T) {
	config := &CacheQueueConfig{
		Enable:          true,
		MaxTxPerAccount: 10,
		TxTimeout:       1 * time.Minute,
		CleanupInterval: 10 * time.Second,
		GlobalMaxTx:     100,
		RetryInterval:   1 * time.Second,
		MaxRetries:      3,
	}

	tcq := NewTransactionCacheQueue(config)
	defer tcq.Stop()

	assert.True(t, tcq.IsEnabled(), "Cache queue should be enabled")

	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Test caching a transaction
	tx := createTestTransaction(1)
	rawTx, err := tx.MarshalBinary()
	require.NoError(t, err, "Should marshal transaction")

	hash, err := tcq.cacheTransaction(rawTx, testAddr, 1, tx)
	require.NoError(t, err, "Should cache transaction successfully")
	assert.Equal(t, tx.Hash(), hash, "Returned hash should match transaction hash")

	// Verify transaction was cached
	queue := tcq.getAccountQueue(testAddr)
	require.NotNil(t, queue, "Account queue should exist")
	assert.Equal(t, 1, queue.Size(), "Queue should have 1 transaction")

	// Test duplicate transaction rejection
	_, err = tcq.cacheTransaction(rawTx, testAddr, 1, tx)
	assert.Error(t, err, "Should reject duplicate transaction")

	// Test metrics
	metrics := tcq.GetMetrics()
	assert.Equal(t, int64(1), metrics.CachedTxCount, "Should have 1 cached transaction")
	assert.Equal(t, int64(1), metrics.AccountsWithCache, "Should have 1 account with cache")
}

func TestTransactionExecution(t *testing.T) {
	config := &CacheQueueConfig{
		Enable:          true,
		MaxTxPerAccount: 10,
		TxTimeout:       1 * time.Minute,
		CleanupInterval: 10 * time.Second,
		GlobalMaxTx:     100,
		RetryInterval:   1 * time.Second,
		MaxRetries:      3,
	}

	tcq := NewTransactionCacheQueue(config)
	defer tcq.Stop()

	// Disable actual broadcasting for tests by setting backend to nil
	// In production, this would be set correctly
	tcq.backend = nil

	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Cache transactions with nonces 2, 3, 5
	tx2 := createTestTransaction(2)
	tx3 := createTestTransaction(3)
	tx5 := createTestTransaction(5)

	rawTx2, _ := tx2.MarshalBinary()
	rawTx3, _ := tx3.MarshalBinary()
	rawTx5, _ := tx5.MarshalBinary()

	_, err := tcq.cacheTransaction(rawTx2, testAddr, 2, tx2)
	require.NoError(t, err)
	_, err = tcq.cacheTransaction(rawTx3, testAddr, 3, tx3)
	require.NoError(t, err)
	_, err = tcq.cacheTransaction(rawTx5, testAddr, 5, tx5)
	require.NoError(t, err)

	// Verify 3 transactions are cached
	queue := tcq.getAccountQueue(testAddr)
	assert.Equal(t, 3, queue.Size(), "Should have 3 cached transactions")

	// Simulate blockchain executed nonces 0, 1, 2
	// RemoveTransactionsUpToNonce(2) removes all transactions with nonce <= 2
	cleanedCount := queue.RemoveTransactionsUpToNonce(2)
	assert.Equal(t, 1, cleanedCount, "Should clean 1 transaction (nonce 2)")

	// Should have 2 remaining transactions (nonces 3 and 5)
	assert.Equal(t, 2, queue.Size(), "Should have 2 remaining transactions (nonces 3 and 5)")

	remaining := queue.GetTransaction(3)
	require.NotNil(t, remaining, "Transaction with nonce 3 should remain")
	remaining = queue.GetTransaction(5)
	require.NotNil(t, remaining, "Transaction with nonce 5 should remain")

	// Simulate blockchain executed up to nonce 4
	// This should remove nonce 3 (since 3 <= 4), but not nonce 5
	cleanedCount = queue.RemoveTransactionsUpToNonce(4)
	assert.Equal(t, 1, cleanedCount, "Should clean 1 transaction (nonce 3)")

	// Should have 1 remaining transaction (nonce 5)
	assert.Equal(t, 1, queue.Size(), "Should have 1 remaining transaction (nonce 5)")

	// Simulate blockchain executed nonce 5
	cleanedCount = queue.RemoveTransactionsUpToNonce(5)
	assert.Equal(t, 1, cleanedCount, "Should clean 1 transaction (nonce 5)")

	// Now queue should be empty
	assert.Equal(t, 0, queue.Size(), "Should have no remaining transactions")

	// Note: Account queue is NOT automatically removed when empty
	// The cleanup happens in cleanupExpiredTransactions() which runs periodically
	// So we don't assert that the queue is nil here
}

// Helper function to create a test transaction
func createTestTransaction(nonce uint64) *types.Transaction {
	return types.NewTransaction(
		nonce,
		common.HexToAddress("0x0000000000000000000000000000000000000001"),
		big.NewInt(1000000000000000), // 0.001 ETH
		21000,                        // Gas limit
		big.NewInt(20000000000),      // 20 Gwei gas price
		nil,                          // No data
	)
}

// Helper function to create a test cached transaction
func createTestCachedTransaction(from common.Address, nonce uint64) *CachedTransaction {
	tx := createTestTransaction(nonce)
	rawTx, _ := tx.MarshalBinary()

	return &CachedTransaction{
		RawTx:       rawTx,
		Transaction: tx,
		From:        from,
		Nonce:       nonce,
		Hash:        tx.Hash(),
		Timestamp:   time.Now(),
		Retries:     0,
	}
}
