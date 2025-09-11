package backend

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"cosmossdk.io/log"
)

// ProductionOptimizations contains production-ready optimizations and edge case handling
type ProductionOptimizations struct {
	tcq    *TransactionCacheQueue
	logger log.Logger
}

// NewProductionOptimizations creates production optimizations
func NewProductionOptimizations(tcq *TransactionCacheQueue, logger log.Logger) *ProductionOptimizations {
	return &ProductionOptimizations{
		tcq:    tcq,
		logger: logger,
	}
}

// HandleGracefulShutdown handles graceful shutdown of cache queue
func (po *ProductionOptimizations) HandleGracefulShutdown(ctx context.Context) error {
	po.logger.Info("Starting graceful shutdown of transaction cache queue")

	// Set a timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Stop accepting new transactions
		po.tcq.config.Enable = false

		// Process remaining transactions with a deadline
		po.processRemainingTransactions()

		// Stop background workers
		po.tcq.Stop()
	}()

	select {
	case <-done:
		po.logger.Info("Cache queue shutdown completed successfully")
		return nil
	case <-shutdownCtx.Done():
		po.logger.Warn("Cache queue shutdown timed out, forcing stop")
		po.tcq.Stop()
		return fmt.Errorf("shutdown timeout")
	}
}

// processRemainingTransactions attempts to process remaining cached transactions
func (po *ProductionOptimizations) processRemainingTransactions() {
	po.logger.Info("Processing remaining cached transactions")

	po.tcq.mu.RLock()
	totalRemaining := 0
	for _, queue := range po.tcq.accounts {
		totalRemaining += queue.Size()
	}
	po.tcq.mu.RUnlock()

	if totalRemaining == 0 {
		return
	}

	po.logger.Info("Found remaining transactions to process", "count", totalRemaining)

	// Wait briefly for any pending events to process remaining transactions
	time.Sleep(2 * time.Second)

	// Log final state
	po.tcq.mu.RLock()
	finalRemaining := 0
	for addr, queue := range po.tcq.accounts {
		size := queue.Size()
		finalRemaining += size
		if size > 0 {
			po.logger.Warn("Account has unprocessed transactions at shutdown",
				"address", addr.Hex(),
				"count", size)
		}
	}
	po.tcq.mu.RUnlock()

	if finalRemaining > 0 {
		po.logger.Warn("Shutdown with unprocessed transactions", "count", finalRemaining)
	}
}

// PerformHealthCheck checks system health and returns status
func (po *ProductionOptimizations) PerformHealthCheck() map[string]interface{} {
	healthStatus := map[string]interface{}{
		"timestamp": time.Now(),
		"enabled":   po.tcq.IsEnabled(),
		"healthy":   true,
		"warnings":  []string{},
		"errors":    []string{},
	}

	warnings := []string{}
	errors := []string{}

	// Check memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Check if memory usage is too high (>100MB for cache queue)
	if memStats.Alloc > 100*1024*1024 {
		warnings = append(warnings, fmt.Sprintf("High memory usage: %d MB", memStats.Alloc/1024/1024))
	}

	// Check cache queue metrics
	metrics := po.tcq.GetMetrics()

	// Check for too many cached transactions
	if metrics.CachedTxCount > int64(po.tcq.config.GlobalMaxTx)*8/10 {
		warnings = append(warnings, fmt.Sprintf("Cache queue approaching limit: %d/%d",
			metrics.CachedTxCount, po.tcq.config.GlobalMaxTx))
	}

	// Check for accounts with too many transactions
	po.tcq.mu.RLock()
	oversizedAccounts := 0
	for addr, queue := range po.tcq.accounts {
		if queue.Size() > po.tcq.config.MaxTxPerAccount*8/10 {
			oversizedAccounts++
			warnings = append(warnings, fmt.Sprintf("Account %s approaching tx limit: %d/%d",
				addr.Hex()[:10]+"...", queue.Size(), po.tcq.config.MaxTxPerAccount))
		}
	}
	po.tcq.mu.RUnlock()

	// Check uptime
	uptime := time.Since(metrics.StartTime)
	if uptime < 5*time.Minute {
		warnings = append(warnings, "System recently started")
	}

	// Check for high expired transaction rate
	if metrics.ExpiredTxCount > 0 && metrics.ProcessedTxCount > 0 {
		expiredRate := float64(metrics.ExpiredTxCount) / float64(metrics.ProcessedTxCount+metrics.ExpiredTxCount)
		if expiredRate > 0.1 { // > 10% expired
			warnings = append(warnings, fmt.Sprintf("High expired transaction rate: %.1f%%", expiredRate*100))
		}
	}

	healthStatus["warnings"] = warnings
	healthStatus["errors"] = errors
	healthStatus["healthy"] = len(errors) == 0

	// Add detailed metrics
	healthStatus["metrics"] = map[string]interface{}{
		"cached_transactions":   metrics.CachedTxCount,
		"active_accounts":      metrics.AccountsWithCache,
		"processed_transactions": metrics.ProcessedTxCount,
		"expired_transactions":  metrics.ExpiredTxCount,
		"uptime_seconds":       int64(uptime.Seconds()),
		"memory_mb":           memStats.Alloc / 1024 / 1024,
	}

	return healthStatus
}

// OptimizeMemoryUsage performs memory optimization
func (po *ProductionOptimizations) OptimizeMemoryUsage() {
	po.logger.Debug("Performing memory optimization")

	// Force garbage collection
	runtime.GC()

	// Trigger cache queue memory enforcement
	po.tcq.enforceMemoryLimits()

	// Log memory usage after optimization
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	po.logger.Debug("Memory optimization completed",
		"allocated_mb", memStats.Alloc/1024/1024,
		"gc_cycles", memStats.NumGC)
}

// HandlePanicRecovery provides panic recovery for cache queue operations
func (po *ProductionOptimizations) HandlePanicRecovery(operation string) {
	if r := recover(); r != nil {
		po.logger.Error("Panic recovered in cache queue operation",
			"operation", operation,
			"panic", r,
			"stack", getStackTrace())

		// Perform health check after panic
		health := po.PerformHealthCheck()
		po.logger.Info("Post-panic health check", "status", health)
	}
}

// getStackTrace returns current stack trace as string
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// ValidateConfiguration validates production configuration
func ValidateConfiguration(config *CacheQueueConfig) []string {
	warnings := []string{}

	if !config.Enable {
		return []string{"Cache queue is disabled"}
	}

	// Check reasonable limits
	if config.MaxTxPerAccount > 2000 {
		warnings = append(warnings, "MaxTxPerAccount is very high (>2000), may impact performance")
	}

	if config.GlobalMaxTx > 100000 {
		warnings = append(warnings, "GlobalMaxTx is very high (>100000), may impact memory")
	}

	if config.TxTimeout < 30*time.Second {
		warnings = append(warnings, "TxTimeout is very short (<30s), may expire transactions too quickly")
	}

	if config.TxTimeout > 30*time.Minute {
		warnings = append(warnings, "TxTimeout is very long (>30m), may hold too much memory")
	}

	if config.CleanupInterval < 10*time.Second {
		warnings = append(warnings, "CleanupInterval is very short (<10s), may impact performance")
	}

	if config.CleanupInterval > 5*time.Minute {
		warnings = append(warnings, "CleanupInterval is very long (>5m), may delay cleanup")
	}

	return warnings
}
