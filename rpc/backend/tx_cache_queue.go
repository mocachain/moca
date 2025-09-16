package backend

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/emirpasic/gods/sets/treeset"
	"github.com/emirpasic/gods/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	rpctypes "github.com/evmos/evmos/v12/rpc/types"
	evmtypes "github.com/evmos/evmos/v12/x/evm/types"
)

// CachedTransaction represents a transaction stored in the cache queue
type CachedTransaction struct {
	RawTx       []byte         // RLP-encoded raw transaction data
	Transaction *types.Transaction
	From        common.Address // Transaction sender
	Nonce       uint64         // Transaction nonce
	Hash        common.Hash    // Transaction hash
	Timestamp   time.Time      // When the transaction was cached
	Retries     int            // Number of retry attempts
}

// CacheQueueConfig holds configuration for the transaction cache queue
type CacheQueueConfig struct {
	Enable                bool          `mapstructure:"enable"`
	MaxTxPerAccount       int           `mapstructure:"max-tx-per-account"`
	TxTimeout             time.Duration `mapstructure:"tx-timeout"`
	CleanupInterval       time.Duration `mapstructure:"cleanup-interval"`
	NoncePollingInterval  time.Duration `mapstructure:"nonce-polling-interval"`
	GlobalMaxTx           int           `mapstructure:"global-max-tx"`
	RetryInterval         time.Duration `mapstructure:"retry-interval"`
	MaxRetries            int           `mapstructure:"max-retries"`
	ReplacementGasPercent int           `mapstructure:"replacement-gas-percent"` // Minimum gas price increase for replacement (default: 10%)
}

// DefaultCacheQueueConfig returns default configuration
func DefaultCacheQueueConfig() *CacheQueueConfig {
	return &CacheQueueConfig{
		Enable:                false, // Disabled by default for safety
		MaxTxPerAccount:       1000,   // Increased for high-load production
		TxTimeout:             1 * time.Minute, // Reduced from 5 minutes
		CleanupInterval:       15 * time.Second, // More frequent cleanup
		NoncePollingInterval:  1 * time.Second, // Nonce polling interval
		GlobalMaxTx:           50000, // Increased for high-load production
		RetryInterval:         500 * time.Millisecond, // Faster retry
		MaxRetries:            3,
		ReplacementGasPercent: 10,    // 10% minimum gas price increase for replacement
	}
}

// CacheMetrics tracks performance and operational metrics
type CacheMetrics struct {
	CachedTxCount          int64         `json:"cached_tx_count"`
	AccountsWithCache      int64         `json:"accounts_with_cache"`
	AvgWaitTimeMs          float64       `json:"avg_wait_time_ms"`
	CacheHitRate           float64       `json:"cache_hit_rate"`
	ExpiredTxCount         int64         `json:"expired_tx_count"`
	ProcessedTxCount       int64         `json:"processed_tx_count"`
	PeakCachedTxCount      int64         `json:"peak_cached_tx_count"`
	PeakAccountsWithCache  int64         `json:"peak_accounts_with_cache"`
	StartTime              time.Time     `json:"start_time"`
	LastCleanupTime        time.Time     `json:"last_cleanup_time"`
	mu                     sync.RWMutex
}

// NonceSet wraps treeset.Set for nonce management with thread safety
type NonceSet struct {
	set *treeset.Set
	mu  sync.RWMutex
}

// NewNonceSet creates a new nonce set using treeset
func NewNonceSet() *NonceSet {
	return &NonceSet{
		set: treeset.NewWith(utils.UInt64Comparator),
	}
}

// Add inserts a nonce into the set
func (ns *NonceSet) Add(nonce uint64) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.set.Add(nonce)
}

// Remove removes a nonce from the set
func (ns *NonceSet) Remove(nonce uint64) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.set.Remove(nonce)
}

// RangeFrom returns all nonces starting from the given value
func (ns *NonceSet) RangeFrom(start uint64) []uint64 {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	var result []uint64
	iterator := ns.set.Iterator()

	// Move to first element >= start
	for iterator.Next() {
		if value := iterator.Value().(uint64); value >= start {
			result = append(result, value)
		}
	}

	return result
}

// Size returns the number of nonces in the set
func (ns *NonceSet) Size() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.set.Size()
}

// AccountQueue manages transactions for a single account
type AccountQueue struct {
	transactions  map[uint64]*CachedTransaction // O(1) fast lookup by nonce
	sortedNonces  *NonceSet                     // Ordered nonce collection for range queries
	currentNonce  uint64                        // Current account nonce
	mu           sync.RWMutex                   // Concurrent access protection
}

// NewAccountQueue creates a new account queue
func NewAccountQueue() *AccountQueue {
	return &AccountQueue{
		transactions: make(map[uint64]*CachedTransaction),
		sortedNonces: NewNonceSet(),
		currentNonce: 0,
	}
}

// AddTransaction adds a transaction to the account queue with EVM-style replacement logic
func (aq *AccountQueue) AddTransaction(tx *CachedTransaction, config *CacheQueueConfig) error {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	// Check if transaction with same nonce already exists
	if existingTx, exists := aq.transactions[tx.Nonce]; exists {
		// Apply EVM-style replacement logic
		if canReplace, reason := aq.canReplaceTransaction(existingTx, tx, config); canReplace {
			// Replace the existing transaction
			aq.transactions[tx.Nonce] = tx
			// sortedNonces doesn't need update since nonce is the same

			log.Info("Transaction replaced in account queue",
				"from", tx.From.Hex(),
				"nonce", tx.Nonce,
				"old_hash", existingTx.Hash.Hex(),
				"new_hash", tx.Hash.Hex(),
				"reason", reason)

			return nil
		} else {
			return fmt.Errorf("transaction replacement rejected: %s", reason)
		}
	}

	// Add new transaction to both map and sorted set
	aq.transactions[tx.Nonce] = tx
	aq.sortedNonces.Add(tx.Nonce)

	log.Debug("Transaction added to account queue",
		"from", tx.From.Hex(),
		"nonce", tx.Nonce,
		"hash", tx.Hash.Hex())

	return nil
}

// canReplaceTransaction determines if a new transaction can replace an existing one
// following EVM mempool replacement rules
func (aq *AccountQueue) canReplaceTransaction(existing, new *CachedTransaction, config *CacheQueueConfig) (bool, string) {
	// Both transactions must have the same nonce (already checked by caller)
	if existing.Nonce != new.Nonce {
		return false, "nonce mismatch"
	}

	// Both transactions must be from the same sender (already ensured by account queue)
	if existing.From != new.From {
		return false, "different sender"
	}

	// Get gas prices from transactions
	existingGasPrice := existing.Transaction.GasPrice()
	newGasPrice := new.Transaction.GasPrice()

	// For EIP-1559 transactions, use effective gas price (maxFeePerGas)
	if existing.Transaction.Type() == 2 { // EIP-1559 transaction
		existingGasPrice = existing.Transaction.GasFeeCap()
	}
	if new.Transaction.Type() == 2 { // EIP-1559 transaction
		newGasPrice = new.Transaction.GasFeeCap()
	}

	// Use configured replacement percentage
	minIncreasePercent := config.ReplacementGasPercent
	if minIncreasePercent <= 0 {
		minIncreasePercent = 10 // Default to 10% if not configured
	}

	// Calculate required minimum price: existing * (100 + minIncreasePercent) / 100
	minRequiredPrice := big.NewInt(0)
	minRequiredPrice.Mul(existingGasPrice, big.NewInt(100+int64(minIncreasePercent)))
	minRequiredPrice.Div(minRequiredPrice, big.NewInt(100))

	if newGasPrice.Cmp(minRequiredPrice) >= 0 {
		// Calculate actual percentage increase
		increase := big.NewInt(0)
		increase.Sub(newGasPrice, existingGasPrice)
		percentIncrease := big.NewInt(0)
		percentIncrease.Mul(increase, big.NewInt(100))
		percentIncrease.Div(percentIncrease, existingGasPrice)

		return true, fmt.Sprintf("gas price increased by %s%% (from %s to %s)",
			percentIncrease.String(), existingGasPrice.String(), newGasPrice.String())
	}

	return false, fmt.Sprintf("insufficient gas price increase: current=%s, required=%s (min %d%% increase)",
		newGasPrice.String(), minRequiredPrice.String(), minIncreasePercent)
}

// RemoveTransaction removes a transaction from the account queue
func (aq *AccountQueue) RemoveTransaction(nonce uint64) *CachedTransaction {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	tx, exists := aq.transactions[nonce]
	if !exists {
		return nil
	}

	// Remove from both map and sorted set
	delete(aq.transactions, nonce)
	aq.sortedNonces.Remove(nonce)

	log.Debug("Transaction removed from account queue",
		"from", tx.From.Hex(),
		"nonce", nonce,
		"hash", tx.Hash.Hex())

	return tx
}

// RemoveTransactions removes multiple transactions from the account queue
func (aq *AccountQueue) RemoveTransactions(txs []*CachedTransaction) {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	for _, tx := range txs {
		delete(aq.transactions, tx.Nonce)
		aq.sortedNonces.Remove(tx.Nonce)

		log.Debug("Batch transaction removed from account queue",
			"from", tx.From.Hex(),
			"nonce", tx.Nonce,
			"hash", tx.Hash.Hex())
	}
}

// RemoveTransactionsUpToNonce removes all transactions with nonce <= maxNonce
// This is optimized for Cosmos where transactions are executed in strict nonce order
func (aq *AccountQueue) RemoveTransactionsUpToNonce(maxNonce uint64) int {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	var removedCount int
	var noncesToRemove []uint64

	// Find all nonces <= maxNonce
	for nonce := range aq.transactions {
		if nonce <= maxNonce {
			noncesToRemove = append(noncesToRemove, nonce)
		}
	}

	// Remove transactions and update sorted nonces
	for _, nonce := range noncesToRemove {
		if tx, exists := aq.transactions[nonce]; exists {
			delete(aq.transactions, nonce)
			aq.sortedNonces.Remove(nonce)
			removedCount++

			log.Debug("Cosmos-optimized transaction removal",
				"from", tx.From.Hex(),
				"nonce", nonce,
				"hash", tx.Hash.Hex(),
				"max_nonce", maxNonce)
		}
	}

	return removedCount
}

// FindConsecutiveTransactions finds all consecutive transactions starting from startNonce
// This is the core algorithm optimized for O(log n + k) performance using treeset
func (aq *AccountQueue) FindConsecutiveTransactions(startNonce uint64) []*CachedTransaction {
	aq.mu.RLock()
	defer aq.mu.RUnlock()

	var result []*CachedTransaction
	expectedNonce := startNonce

	// Use NonceSet's treeset for efficient range query
	// TreeSet provides O(log n) positioning and O(k) scanning
	for _, nonce := range aq.sortedNonces.RangeFrom(startNonce) {
		if nonce != expectedNonce {
			break // Found discontinuity, stop scanning
		}
		if tx, exists := aq.transactions[nonce]; exists {
			result = append(result, tx)
			expectedNonce++
		}
	}

	if len(result) > 0 {
		log.Debug("Found consecutive transactions",
			"start_nonce", startNonce,
			"count", len(result),
			"from", result[0].From.Hex())
	}

	return result
}

// GetTransaction retrieves a specific transaction by nonce
func (aq *AccountQueue) GetTransaction(nonce uint64) *CachedTransaction {
	aq.mu.RLock()
	defer aq.mu.RUnlock()
	return aq.transactions[nonce]
}

// UpdateCurrentNonce updates the current nonce for this account
func (aq *AccountQueue) UpdateCurrentNonce(nonce uint64) {
	aq.mu.Lock()
	defer aq.mu.Unlock()
	aq.currentNonce = nonce
}

// GetCurrentNonce returns the current nonce for this account
func (aq *AccountQueue) GetCurrentNonce() uint64 {
	aq.mu.RLock()
	defer aq.mu.RUnlock()
	return aq.currentNonce
}

// Size returns the number of cached transactions for this account
func (aq *AccountQueue) Size() int {
	aq.mu.RLock()
	defer aq.mu.RUnlock()
	return len(aq.transactions)
}

// IsEmpty returns true if the account queue has no transactions
func (aq *AccountQueue) IsEmpty() bool {
	aq.mu.RLock()
	defer aq.mu.RUnlock()
	return len(aq.transactions) == 0
}

// GetExpiredTransactions returns transactions that have exceeded the timeout
func (aq *AccountQueue) GetExpiredTransactions(timeout time.Duration) []*CachedTransaction {
	aq.mu.RLock()
	defer aq.mu.RUnlock()

	var expired []*CachedTransaction
	now := time.Now()

	for _, tx := range aq.transactions {
		if now.Sub(tx.Timestamp) > timeout {
			expired = append(expired, tx)
		}
	}

	return expired
}

// TransactionCacheQueue is the main cache queue manager
type TransactionCacheQueue struct {
	accounts     map[common.Address]*AccountQueue // Per-account transaction queues
	config       *CacheQueueConfig                // Configuration settings
	metrics      *CacheMetrics                    // Performance metrics
	backend      *Backend                         // Reference to backend for broadcasting
	mu           sync.RWMutex                     // Global lock for account map
	stopCh       chan struct{}                    // Stop signal for background tasks
	wg           sync.WaitGroup                   // Wait group for background tasks
}

// NewTransactionCacheQueue creates a new transaction cache queue
func NewTransactionCacheQueue(config *CacheQueueConfig) *TransactionCacheQueue {
	if config == nil {
		config = DefaultCacheQueueConfig()
	}
	tcq := &TransactionCacheQueue{
		accounts: make(map[common.Address]*AccountQueue),
		config:   config,
		metrics: &CacheMetrics{
			StartTime: time.Now(),
		},
		stopCh: make(chan struct{}),
	}

	// Start background maintenance tasks if enabled
	if config.Enable {
		tcq.startBackgroundTasks()
	}
	return tcq
}

// IsEnabled returns true if the cache queue is enabled
func (tcq *TransactionCacheQueue) IsEnabled() bool {
	if tcq.config == nil {
		return false
	}
	return tcq.config.Enable
}

// SetBackend sets the backend reference for transaction broadcasting
func (tcq *TransactionCacheQueue) SetBackend(backend *Backend) {
	tcq.backend = backend
}

// ProcessTransaction processes an incoming transaction and caches it for future processing
func (tcq *TransactionCacheQueue) ProcessTransaction(rawTx []byte, from common.Address, nonce uint64) (common.Hash, error) {
	if !tcq.IsEnabled() {
		return common.Hash{}, fmt.Errorf("cache queue is disabled")
	}

	// Decode transaction to get hash
	tx := &types.Transaction{}
	if err := tx.UnmarshalBinary(rawTx); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode raw transaction: %w", err)
	}

	log.Info("Caching transaction for future processing",
		"from", from.Hex(),
		"nonce", nonce,
		"hash", tx.Hash().Hex())

	return tcq.cacheTransaction(rawTx, from, nonce, tx)
}

// cacheTransaction adds a transaction to the appropriate account queue
func (tcq *TransactionCacheQueue) cacheTransaction(rawTx []byte, from common.Address, nonce uint64, tx *types.Transaction) (common.Hash, error) {
	// Check global transaction limit
	if tcq.getTotalCachedCount() >= tcq.config.GlobalMaxTx {
		return common.Hash{}, fmt.Errorf("global transaction cache limit exceeded")
	}

	// Get or create account queue
	queue := tcq.getOrCreateAccountQueue(from)

	// Check per-account transaction limit
	if queue.Size() >= tcq.config.MaxTxPerAccount {
		return common.Hash{}, fmt.Errorf("account transaction cache limit exceeded")
	}

	// Create cached transaction
	cachedTx := &CachedTransaction{
		RawTx:       rawTx,
		Transaction: tx,
		From:        from,
		Nonce:       nonce,
		Hash:        tx.Hash(),
		Timestamp:   time.Now(),
		Retries:     0,
	}

	// Add to account queue
	if err := queue.AddTransaction(cachedTx, tcq.config); err != nil {
		return common.Hash{}, fmt.Errorf("failed to cache transaction: %w", err)
	}

	// Update metrics
	tcq.updateMetrics()

	log.Info("Transaction cached successfully",
		"from", from.Hex(),
		"nonce", nonce,
		"hash", tx.Hash().Hex())

	return tx.Hash(), nil
}

// batchBroadcastTransactionsAndRemove broadcasts transactions and immediately removes them from cache
// This prevents "already exists" errors when new transactions arrive quickly
func (tcq *TransactionCacheQueue) batchBroadcastTransactionsAndRemove(txs []*CachedTransaction, queue *AccountQueue) {
	if len(txs) == 0 {
		return
	}

	log.Info("Broadcasting and removing cached transactions",
		"count", len(txs),
		"from", txs[0].From.Hex())

	// First, remove transactions from cache to prevent conflicts
	queue.RemoveTransactions(txs)

	// Then broadcast each transaction using the backend
	for _, tx := range txs {
		log.Info("Broadcasting cached transaction",
			"from", tx.From.Hex(),
			"nonce", tx.Nonce,
			"hash", tx.Hash.Hex())

		// Skip actual broadcast if backend is not set (e.g., in tests)
		if tcq.backend == nil {
			log.Debug("Skipping broadcast - no backend set (likely in test mode)")
			continue
		}

		// Create MsgEthereumTx from cached transaction
		ethereumTx := &evmtypes.MsgEthereumTx{}
		if err := ethereumTx.FromEthereumTx(tx.Transaction); err != nil {
			log.Error("Failed to convert cached transaction",
				"hash", tx.Hash.Hex(),
				"error", err)
			continue
		}

		// Use backend's broadcast method
		broadcastHash, err := tcq.backend.broadcastTransaction(ethereumTx, tx.RawTx)
		if err != nil {
			log.Error("Failed to broadcast cached transaction",
				"hash", tx.Hash.Hex(),
				"error", err)
			// TODO: Implement retry logic here
			tx.Retries++
			if tx.Retries < tcq.config.MaxRetries {
				log.Info("Scheduling transaction for retry",
					"hash", tx.Hash.Hex(),
					"retries", tx.Retries)
			}
		} else {
			log.Info("Successfully broadcasted cached transaction",
				"hash", broadcastHash.Hex(),
				"originalHash", tx.Hash.Hex())
		}
	}

	// Update metrics
	tcq.metrics.mu.Lock()
	tcq.metrics.ProcessedTxCount += int64(len(txs))
	tcq.metrics.mu.Unlock()
}

// getOrCreateAccountQueue gets an existing account queue or creates a new one
func (tcq *TransactionCacheQueue) getOrCreateAccountQueue(from common.Address) *AccountQueue {
	tcq.mu.Lock()
	defer tcq.mu.Unlock()

	if queue, exists := tcq.accounts[from]; exists {
		return queue
	}

	// Create new account queue
	queue := NewAccountQueue()
	// Note: Current nonce will be updated by the nonce polling cleaner
	tcq.accounts[from] = queue

	log.Debug("Created new account queue", "from", from.Hex())
	return queue
}

// getAccountQueue gets an existing account queue (read-only)
func (tcq *TransactionCacheQueue) getAccountQueue(from common.Address) *AccountQueue {
	tcq.mu.RLock()
	defer tcq.mu.RUnlock()
	return tcq.accounts[from]
}

// removeAccountQueue removes an account queue when it becomes empty
func (tcq *TransactionCacheQueue) removeAccountQueue(from common.Address) {
	tcq.mu.Lock()
	defer tcq.mu.Unlock()
	delete(tcq.accounts, from)

	log.Debug("Removed empty account queue", "from", from.Hex())
}


// getTotalCachedCount returns the total number of cached transactions across all accounts
func (tcq *TransactionCacheQueue) getTotalCachedCount() int {
	tcq.mu.RLock()
	defer tcq.mu.RUnlock()

	total := 0
	for _, queue := range tcq.accounts {
		total += queue.Size()
	}
	return total
}

// updateMetrics updates performance metrics
func (tcq *TransactionCacheQueue) updateMetrics() {
	tcq.metrics.mu.Lock()
	defer tcq.metrics.mu.Unlock()

	tcq.metrics.CachedTxCount = int64(tcq.getTotalCachedCount())
	tcq.metrics.AccountsWithCache = int64(len(tcq.accounts))
}

// GetCachedAccounts returns a list of all accounts with cached transactions
func (tcq *TransactionCacheQueue) GetCachedAccounts() []common.Address {
	tcq.mu.RLock()
	defer tcq.mu.RUnlock()

	accounts := make([]common.Address, 0, len(tcq.accounts))
	for addr := range tcq.accounts {
		accounts = append(accounts, addr)
	}

	return accounts
}

// GetMetrics returns current performance metrics
func (tcq *TransactionCacheQueue) GetMetrics() *CacheMetrics {
	tcq.updateMetrics()

	tcq.metrics.mu.RLock()
	defer tcq.metrics.mu.RUnlock()

	// Return a copy to avoid race conditions
	return &CacheMetrics{
		CachedTxCount:          tcq.metrics.CachedTxCount,
		AccountsWithCache:      tcq.metrics.AccountsWithCache,
		AvgWaitTimeMs:          tcq.metrics.AvgWaitTimeMs,
		CacheHitRate:           tcq.metrics.CacheHitRate,
		ExpiredTxCount:         tcq.metrics.ExpiredTxCount,
		ProcessedTxCount:       tcq.metrics.ProcessedTxCount,
		PeakCachedTxCount:      tcq.metrics.PeakCachedTxCount,
		PeakAccountsWithCache:  tcq.metrics.PeakAccountsWithCache,
		StartTime:              tcq.metrics.StartTime,
		LastCleanupTime:        tcq.metrics.LastCleanupTime,
	}
}

// startBackgroundTasks starts background maintenance tasks
func (tcq *TransactionCacheQueue) startBackgroundTasks() {
	// Start cleanup worker
	tcq.wg.Add(1)
	go tcq.cleanupWorker()

	// Start memory monitor worker
	tcq.wg.Add(1)
	go tcq.memoryMonitorWorker()

	// Start metrics worker
	tcq.wg.Add(1)
	go tcq.metricsWorker()

	log.Info("Transaction cache queue background tasks started",
		"cleanup_interval", tcq.config.CleanupInterval,
		"max_tx_per_account", tcq.config.MaxTxPerAccount,
		"global_max_tx", tcq.config.GlobalMaxTx)
}

// cleanupWorker runs periodic cleanup of expired transactions
func (tcq *TransactionCacheQueue) cleanupWorker() {
	defer tcq.wg.Done()

	ticker := time.NewTicker(tcq.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tcq.cleanupExpiredTransactions()
		case <-tcq.stopCh:
			log.Info("Transaction cache queue cleanup worker stopped")
			return
		}
	}
}

// cleanupExpiredTransactions removes expired transactions from all account queues
func (tcq *TransactionCacheQueue) cleanupExpiredTransactions() {
	log.Debug("Starting cleanup of expired transactions")

	var expiredCount int64
	var executedCount int64
	var emptyAccounts []common.Address

	tcq.mu.RLock()
	accounts := make(map[common.Address]*AccountQueue)
	for addr, queue := range tcq.accounts {
		accounts[addr] = queue
	}
	tcq.mu.RUnlock()

	// Check each account for expired transactions AND already executed transactions
	for addr, queue := range accounts {
		// BACKUP CLEANUP: Check account nonce to clean executed transactions
		// This handles cases where event listener missed execution events
		log.Debug("Checking backup cleanup for account", "from", addr.Hex(), "backend_nil", tcq.backend == nil)

		if tcq.backend != nil {
			currentNonce, err := tcq.backend.GetTransactionCount(addr, rpctypes.EthLatestBlockNumber)
			log.Debug("Backend nonce query result",
				"from", addr.Hex(),
				"nonce", currentNonce,
				"error", err)

			if err == nil && currentNonce != nil {
				accountCurrentNonce := uint64(*currentNonce)

				// Remove all transactions with nonce < current account nonce
				// These transactions have been executed but events were missed
				if accountCurrentNonce > 0 {
					removedExecuted := queue.RemoveTransactionsUpToNonce(accountCurrentNonce - 1)
					if removedExecuted > 0 {
						executedCount += int64(removedExecuted)
						log.Info("Backup cleanup: removed already executed transactions",
							"from", addr.Hex(),
							"current_nonce", accountCurrentNonce,
							"removed_count", removedExecuted)
					} else {
						log.Debug("Backup cleanup: no executed transactions to remove",
							"from", addr.Hex(),
							"current_nonce", accountCurrentNonce)
					}
				}
			} else {
				log.Warn("Backup cleanup: failed to get current nonce",
					"from", addr.Hex(),
					"error", err)
			}
		} else {
			log.Warn("Backup cleanup: backend is nil")
		}

		// Original expired transaction cleanup
		expired := queue.GetExpiredTransactions(tcq.config.TxTimeout)
		for _, tx := range expired {
			queue.RemoveTransaction(tx.Nonce)
			expiredCount++

			log.Debug("Removed expired transaction",
				"from", addr.Hex(),
				"nonce", tx.Nonce,
				"hash", tx.Hash.Hex(),
				"age", time.Since(tx.Timestamp))
		}

		// Mark empty accounts for removal
		if queue.IsEmpty() {
			emptyAccounts = append(emptyAccounts, addr)
		}
	}

	// Remove empty account queues
	for _, addr := range emptyAccounts {
		tcq.removeAccountQueue(addr)
	}

	// Update metrics
	tcq.metrics.mu.Lock()
	tcq.metrics.ExpiredTxCount += expiredCount
	tcq.metrics.LastCleanupTime = time.Now()
	tcq.metrics.mu.Unlock()

	if expiredCount > 0 || executedCount > 0 {
		log.Info("Cleanup completed",
			"expired_transactions", expiredCount,
			"executed_transactions", executedCount,
			"removed_accounts", len(emptyAccounts))
	}
}

// memoryMonitorWorker monitors memory usage and enforces limits
func (tcq *TransactionCacheQueue) memoryMonitorWorker() {
	defer tcq.wg.Done()

	// Monitor every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tcq.enforceMemoryLimits()
		case <-tcq.stopCh:
			log.Info("Transaction cache queue memory monitor stopped")
			return
		}
	}
}

// enforceMemoryLimits ensures memory usage stays within configured bounds
func (tcq *TransactionCacheQueue) enforceMemoryLimits() {
	tcq.mu.Lock()
	defer tcq.mu.Unlock()

	totalTxCount := 0
	oversizedAccounts := []common.Address{}

	// Count total transactions and identify oversized accounts
	for addr, queue := range tcq.accounts {
		queueSize := queue.Size()
		totalTxCount += queueSize

		if queueSize > tcq.config.MaxTxPerAccount {
			oversizedAccounts = append(oversizedAccounts, addr)
		}
	}

	// Enforce global limit
	if totalTxCount > tcq.config.GlobalMaxTx {
		excess := totalTxCount - tcq.config.GlobalMaxTx
		log.Warn("Global transaction limit exceeded, removing oldest transactions",
			"total_transactions", totalTxCount,
			"global_limit", tcq.config.GlobalMaxTx,
			"excess", excess)

		tcq.removeOldestTransactions(excess)
	}

	// Enforce per-account limits
	for _, addr := range oversizedAccounts {
		queue := tcq.accounts[addr]
		if queue == nil {
			continue
		}

		excess := queue.Size() - tcq.config.MaxTxPerAccount
		log.Warn("Account transaction limit exceeded, removing oldest transactions",
			"account", addr.Hex(),
			"queue_size", queue.Size(),
			"account_limit", tcq.config.MaxTxPerAccount,
			"excess", excess)

		tcq.removeOldestTransactionsFromAccount(addr, excess)
	}

	// Update memory metrics
	tcq.updateMemoryMetrics(totalTxCount, len(oversizedAccounts))
}

// removeOldestTransactions removes the oldest transactions globally across all accounts
func (tcq *TransactionCacheQueue) removeOldestTransactions(count int) {
	type txWithTimestamp struct {
		addr      common.Address
		nonce     uint64
		timestamp time.Time
	}

	var allTxs []txWithTimestamp

	// Collect all transactions with timestamps
	for addr, queue := range tcq.accounts {
		queue.mu.RLock()
		for nonce, tx := range queue.transactions {
			allTxs = append(allTxs, txWithTimestamp{
				addr:      addr,
				nonce:     nonce,
				timestamp: tx.Timestamp,
			})
		}
		queue.mu.RUnlock()
	}

	// Sort by timestamp (oldest first)
	for i := 0; i < len(allTxs)-1; i++ {
		for j := i + 1; j < len(allTxs); j++ {
			if allTxs[i].timestamp.After(allTxs[j].timestamp) {
				allTxs[i], allTxs[j] = allTxs[j], allTxs[i]
			}
		}
	}

	// Remove oldest transactions
	removed := 0
	for _, tx := range allTxs {
		if removed >= count {
			break
		}

		queue := tcq.accounts[tx.addr]
		if queue != nil {
			queue.RemoveTransaction(tx.nonce)
			removed++

			log.Debug("Removed oldest transaction for memory limit",
				"account", tx.addr.Hex(),
				"nonce", tx.nonce,
				"age", time.Since(tx.timestamp))

			// Remove empty account queues
			if queue.IsEmpty() {
				delete(tcq.accounts, tx.addr)
			}
		}
	}

	log.Info("Removed oldest transactions for global memory limit", "removed", removed)
}

// removeOldestTransactionsFromAccount removes oldest transactions from a specific account
func (tcq *TransactionCacheQueue) removeOldestTransactionsFromAccount(addr common.Address, count int) {
	queue := tcq.accounts[addr]
	if queue == nil {
		return
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	type txWithTimestamp struct {
		nonce     uint64
		timestamp time.Time
	}

	var txs []txWithTimestamp
	for nonce, tx := range queue.transactions {
		txs = append(txs, txWithTimestamp{
			nonce:     nonce,
			timestamp: tx.Timestamp,
		})
	}

	// Sort by timestamp (oldest first)
	for i := 0; i < len(txs)-1; i++ {
		for j := i + 1; j < len(txs); j++ {
			if txs[i].timestamp.After(txs[j].timestamp) {
				txs[i], txs[j] = txs[j], txs[i]
			}
		}
	}

	// Remove oldest transactions
	removed := 0
	for _, tx := range txs {
		if removed >= count {
			break
		}

		queue.RemoveTransaction(tx.nonce)
		removed++

		log.Debug("Removed oldest transaction from account for memory limit",
			"account", addr.Hex(),
			"nonce", tx.nonce,
			"age", time.Since(tx.timestamp))
	}

	// Remove empty account queue
	if queue.IsEmpty() {
		delete(tcq.accounts, addr)
	}

	log.Info("Removed oldest transactions from account for memory limit",
		"account", addr.Hex(),
		"removed", removed)
}

// updateMemoryMetrics updates memory-related metrics
func (tcq *TransactionCacheQueue) updateMemoryMetrics(totalTx int, oversizedAccounts int) {
	tcq.metrics.mu.Lock()
	defer tcq.metrics.mu.Unlock()

	tcq.metrics.CachedTxCount = int64(totalTx)
	tcq.metrics.AccountsWithCache = int64(len(tcq.accounts))

	// Update peak metrics
	if tcq.metrics.CachedTxCount > tcq.metrics.PeakCachedTxCount {
		tcq.metrics.PeakCachedTxCount = tcq.metrics.CachedTxCount
	}

	if tcq.metrics.AccountsWithCache > tcq.metrics.PeakAccountsWithCache {
		tcq.metrics.PeakAccountsWithCache = tcq.metrics.AccountsWithCache
	}

	if oversizedAccounts > 0 {
		log.Debug("Memory monitoring statistics",
			"total_transactions", totalTx,
			"active_accounts", len(tcq.accounts),
			"oversized_accounts", oversizedAccounts,
			"peak_transactions", tcq.metrics.PeakCachedTxCount,
			"peak_accounts", tcq.metrics.PeakAccountsWithCache)
	}
}

// metricsWorker periodically logs performance metrics
func (tcq *TransactionCacheQueue) metricsWorker() {
	defer tcq.wg.Done()

	// Report metrics every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tcq.logPerformanceMetrics()
		case <-tcq.stopCh:
			log.Info("Transaction cache queue metrics worker stopped")
			return
		}
	}
}

// logPerformanceMetrics logs comprehensive performance statistics
func (tcq *TransactionCacheQueue) logPerformanceMetrics() {
	metrics := tcq.GetMetrics()

	tcq.mu.RLock()
	totalTx := 0
	maxQueueSize := 0
	minQueueSize := int(^uint(0) >> 1) // max int

	for _, queue := range tcq.accounts {
		size := queue.Size()
		totalTx += size
		if size > maxQueueSize {
			maxQueueSize = size
		}
		if size < minQueueSize {
			minQueueSize = size
		}
	}

	if len(tcq.accounts) == 0 {
		minQueueSize = 0
	}

	accountCount := len(tcq.accounts)
	tcq.mu.RUnlock()

	log.Info("Transaction cache queue performance metrics",
		"cached_transactions", totalTx,
		"active_accounts", accountCount,
		"processed_transactions", metrics.ProcessedTxCount,
		"expired_transactions", metrics.ExpiredTxCount,
		"peak_cached_transactions", metrics.PeakCachedTxCount,
		"peak_active_accounts", metrics.PeakAccountsWithCache,
		"max_queue_size", maxQueueSize,
		"min_queue_size", minQueueSize,
		"last_cleanup", metrics.LastCleanupTime.Format(time.RFC3339),
		"uptime", time.Since(tcq.metrics.StartTime).Round(time.Second))
}

// Stop stops the cache queue and all background tasks
func (tcq *TransactionCacheQueue) Stop() {
	if tcq.config.Enable {
		close(tcq.stopCh)
		tcq.wg.Wait()
		log.Info("Transaction cache queue stopped")
	}
}
