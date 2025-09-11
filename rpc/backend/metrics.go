package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cosmossdk.io/log"
)

// MetricsHandler provides HTTP endpoints for cache queue metrics
type MetricsHandler struct {
	tcq    *TransactionCacheQueue
	logger log.Logger
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(tcq *TransactionCacheQueue, logger log.Logger) *MetricsHandler {
	return &MetricsHandler{
		tcq:    tcq,
		logger: logger,
	}
}

// RegisterRoutes registers metrics endpoints
func (mh *MetricsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/debug/cache_queue/metrics", mh.handleMetrics)
	mux.HandleFunc("/debug/cache_queue/status", mh.handleStatus)
	mux.HandleFunc("/debug/cache_queue/accounts", mh.handleAccounts)
}

// handleMetrics returns JSON metrics
func (mh *MetricsHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := mh.tcq.GetMetrics()
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		mh.logger.Error("Failed to encode metrics", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleStatus returns cache queue status
func (mh *MetricsHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mh.tcq.mu.RLock()
	totalTx := 0
	accountCount := len(mh.tcq.accounts)
	
	for _, queue := range mh.tcq.accounts {
		totalTx += queue.Size()
	}
	mh.tcq.mu.RUnlock()

	status := map[string]interface{}{
		"enabled":           mh.tcq.IsEnabled(),
		"total_transactions": totalTx,
		"active_accounts":   accountCount,
		"uptime":           time.Since(mh.tcq.metrics.StartTime).String(),
		"config": map[string]interface{}{
			"max_tx_per_account": mh.tcq.config.MaxTxPerAccount,
			"global_max_tx":     mh.tcq.config.GlobalMaxTx,
			"tx_timeout":        mh.tcq.config.TxTimeout.String(),
			"cleanup_interval":  mh.tcq.config.CleanupInterval.String(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		mh.logger.Error("Failed to encode status", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAccounts returns per-account queue information
func (mh *MetricsHandler) handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mh.tcq.mu.RLock()
	accounts := make([]map[string]interface{}, 0, len(mh.tcq.accounts))
	
	for addr, queue := range mh.tcq.accounts {
		queueInfo := map[string]interface{}{
			"address":        addr.Hex(),
			"queue_size":     queue.Size(),
			"current_nonce":  queue.currentNonce,
		}
		
		// Add transaction details if queue is small
		if queue.Size() <= 10 {
			queue.mu.RLock()
			transactions := make([]map[string]interface{}, 0, len(queue.transactions))
			for nonce, tx := range queue.transactions {
				transactions = append(transactions, map[string]interface{}{
					"nonce":     nonce,
					"hash":      tx.Hash.Hex(),
					"timestamp": tx.Timestamp,
					"retries":   tx.Retries,
				})
			}
			queue.mu.RUnlock()
			queueInfo["transactions"] = transactions
		}
		
		accounts = append(accounts, queueInfo)
	}
	mh.tcq.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(accounts); err != nil {
		mh.logger.Error("Failed to encode accounts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// PrometheusMetrics generates Prometheus-style metrics
func (mh *MetricsHandler) PrometheusMetrics() string {
	metrics := mh.tcq.GetMetrics()
	
	mh.tcq.mu.RLock()
	totalTx := 0
	accountCount := len(mh.tcq.accounts)
	for _, queue := range mh.tcq.accounts {
		totalTx += queue.Size()
	}
	mh.tcq.mu.RUnlock()

	return fmt.Sprintf(`# HELP moca_cache_queue_cached_transactions Number of cached transactions
# TYPE moca_cache_queue_cached_transactions gauge
moca_cache_queue_cached_transactions %d

# HELP moca_cache_queue_active_accounts Number of active accounts with cached transactions  
# TYPE moca_cache_queue_active_accounts gauge
moca_cache_queue_active_accounts %d

# HELP moca_cache_queue_processed_transactions_total Total number of processed transactions
# TYPE moca_cache_queue_processed_transactions_total counter
moca_cache_queue_processed_transactions_total %d

# HELP moca_cache_queue_expired_transactions_total Total number of expired transactions
# TYPE moca_cache_queue_expired_transactions_total counter  
moca_cache_queue_expired_transactions_total %d

# HELP moca_cache_queue_peak_cached_transactions Peak number of cached transactions
# TYPE moca_cache_queue_peak_cached_transactions gauge
moca_cache_queue_peak_cached_transactions %d

# HELP moca_cache_queue_enabled Whether cache queue is enabled
# TYPE moca_cache_queue_enabled gauge
moca_cache_queue_enabled %d
`,
		totalTx,
		accountCount,
		metrics.ProcessedTxCount,
		metrics.ExpiredTxCount,
		metrics.PeakCachedTxCount,
		boolToInt(mh.tcq.IsEnabled()),
	)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
