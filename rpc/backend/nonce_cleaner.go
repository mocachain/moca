package backend

import (
	"context"
	"sync"
	"time"

	"cosmossdk.io/log"
	"github.com/ethereum/go-ethereum/common"
	rpctypes "github.com/evmos/evmos/v12/rpc/types"
)

// NoncePollingCleaner handles periodic nonce-based cache cleanup
type NoncePollingCleaner struct {
	backend         *Backend
	logger          log.Logger
	ctx             context.Context
	cancelFunc      context.CancelFunc
	interval        time.Duration
	mu              sync.RWMutex
	lastProcessed   map[common.Address]uint64 // Track last processed nonce for each account
	cleanupCount    uint64
	lastCleanupTime time.Time
}

// startNonceBasedCleaner initializes and starts the nonce-based cache cleaner
func (b *Backend) startNonceBasedCleaner() {
	// Get nonce polling interval from config
	interval := b.txCacheQueue.config.NoncePollingInterval
	if interval <= 0 {
		interval = 1 * time.Second // Fallback to default if not configured
	}

	cleaner := &NoncePollingCleaner{
		backend:         b,
		logger:          b.logger.With("component", "nonce-polling-cleaner"),
		interval:        interval,
		lastProcessed:   make(map[common.Address]uint64),
		lastCleanupTime: time.Now(),
	}

	cleaner.ctx, cleaner.cancelFunc = context.WithCancel(context.Background())

	go cleaner.startNoncePolling()

	b.logger.Info("Nonce-based cache cleaner started", "interval", cleaner.interval)
}

// startNoncePolling starts the periodic nonce-based cache cleanup
func (npc *NoncePollingCleaner) startNoncePolling() {
	npc.logger.Info("Starting nonce-based cache cleanup polling", "interval", npc.interval)

	ticker := time.NewTicker(npc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-npc.ctx.Done():
			npc.logger.Info("Nonce polling cleaner stopped")
			return
		case <-ticker.C:
			npc.performCleanup()
		}
	}
}

// performCleanup checks all cached accounts and cleans executed transactions
func (npc *NoncePollingCleaner) performCleanup() {
	if !npc.backend.txCacheQueue.IsEnabled() {
		return
	}

	npc.logger.Debug("Starting nonce-based cleanup cycle")

	// Get all accounts with cached transactions
	cachedAccounts := npc.backend.txCacheQueue.GetCachedAccounts()
	npc.logger.Debug("Nonce cleanup: found cached accounts", "count", len(cachedAccounts))
	if len(cachedAccounts) == 0 {
		npc.logger.Debug("Nonce cleanup: no accounts with cached transactions")
		return // No accounts to process
	}

	var totalCleaned int
	var accountsProcessed int

	for _, addr := range cachedAccounts {
		cleaned := npc.cleanAccountTransactions(addr)
		if cleaned > 0 {
			totalCleaned += cleaned
			accountsProcessed++
		}
	}

	npc.mu.Lock()
	npc.cleanupCount++
	npc.lastCleanupTime = time.Now()
	npc.mu.Unlock()

	if totalCleaned > 0 {
		npc.logger.Info("Nonce-based cleanup completed",
			"accounts_processed", accountsProcessed,
			"transactions_cleaned", totalCleaned,
			"total_accounts", len(cachedAccounts))
	} else {
		npc.logger.Debug("Nonce-based cleanup completed",
			"accounts_checked", len(cachedAccounts),
			"transactions_cleaned", 0)
	}
}

// cleanAccountTransactions cleans executed transactions and processes ready transactions for a specific account
func (npc *NoncePollingCleaner) cleanAccountTransactions(addr common.Address) int {
	// Get current nonce from blockchain
	currentNonce, err := npc.backend.GetTransactionCount(addr, rpctypes.EthLatestBlockNumber)
	if err != nil {
		npc.logger.Debug("Failed to get current nonce for account",
			"address", addr.Hex(),
			"error", err)
		return 0
	}

	if currentNonce == nil {
		return 0
	}

	accountCurrentNonce := uint64(*currentNonce)

	// Check if nonce has changed since last cleanup
	npc.mu.RLock()
	_, _ = npc.lastProcessed[addr] // We'll update this at the end regardless
	npc.mu.RUnlock()

	// Get account queue
	queue := npc.backend.txCacheQueue.getAccountQueue(addr)
	if queue == nil {
		return 0 // No cached transactions for this account
	}

	var totalProcessed int

	// 1. Remove all transactions with nonce < current account nonce (already executed)
	var removedCount int
	if accountCurrentNonce > 0 {
		removedCount = queue.RemoveTransactionsUpToNonce(accountCurrentNonce - 1)
		totalProcessed += removedCount
	}

	// 2. CRITICAL: Process ready transactions starting from current nonce
	// Find consecutive transactions starting from the current expected nonce
	consecutiveTxs := queue.FindConsecutiveTransactions(accountCurrentNonce)
	if len(consecutiveTxs) > 0 {
		npc.logger.Info("Found ready transactions to broadcast",
			"from", addr.Hex(),
			"current_nonce", accountCurrentNonce,
			"ready_count", len(consecutiveTxs),
			"starting_nonce", consecutiveTxs[0].Nonce)

		// Broadcast transactions and immediately remove them from cache
		npc.backend.txCacheQueue.batchBroadcastTransactionsAndRemove(consecutiveTxs, queue)
		totalProcessed += len(consecutiveTxs)

		npc.logger.Info("Broadcasted ready cached transactions",
			"from", addr.Hex(),
			"count", len(consecutiveTxs),
			"starting_nonce", consecutiveTxs[0].Nonce)
	}

	// Update last processed nonce
	npc.mu.Lock()
	npc.lastProcessed[addr] = accountCurrentNonce
	npc.mu.Unlock()

	if totalProcessed > 0 {
		npc.logger.Info("Processed transactions for account",
			"address", addr.Hex(),
			"current_nonce", accountCurrentNonce,
			"cleaned_count", removedCount,
			"broadcasted_count", totalProcessed - removedCount,
			"total_processed", totalProcessed)

		// If queue is empty, remove the account entry
		if queue.IsEmpty() {
			npc.backend.txCacheQueue.removeAccountQueue(addr)
			npc.logger.Debug("Removed empty account queue", "address", addr.Hex())

			// Clean up our tracking map as well
			npc.mu.Lock()
			delete(npc.lastProcessed, addr)
			npc.mu.Unlock()
		}
	}

	return totalProcessed
}

// GetCleanupStats returns statistics about the cleanup process
func (npc *NoncePollingCleaner) GetCleanupStats() map[string]interface{} {
	npc.mu.RLock()
	defer npc.mu.RUnlock()

	return map[string]interface{}{
		"cleanup_count":       npc.cleanupCount,
		"last_cleanup_time":   npc.lastCleanupTime,
		"interval_seconds":    npc.interval.Seconds(),
		"tracked_accounts":    len(npc.lastProcessed),
		"active":              npc.ctx.Err() == nil,
	}
}

// Stop gracefully stops the nonce polling cleaner
func (npc *NoncePollingCleaner) Stop() {
	if npc.cancelFunc != nil {
		npc.cancelFunc()
		npc.logger.Info("Nonce polling cleaner stop requested")
	}
}
