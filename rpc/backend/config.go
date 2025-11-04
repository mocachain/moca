package backend

import "time"

// CacheQueueAppConfig represents application-level cache queue configuration
type CacheQueueAppConfig struct {
	// Cache queue settings
	Cache CacheQueueSettings `mapstructure:"cache"`
}

// CacheQueueSettings contains all cache queue configuration options
type CacheQueueSettings struct {
	// Enable enables or disables the transaction cache queue
	Enable bool `mapstructure:"enable"`

	// MaxTxPerAccount sets the maximum number of transactions per account
	MaxTxPerAccount int `mapstructure:"max_tx_per_account"`

	// TxTimeout sets the timeout for cached transactions
	TxTimeout time.Duration `mapstructure:"tx_timeout"`

	// CleanupInterval sets the interval for cleanup operations
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`

	// NoncePollingInterval sets the interval for nonce-based polling cleanup
	NoncePollingInterval time.Duration `mapstructure:"nonce_polling_interval"`

	// GlobalMaxTx sets the global maximum number of cached transactions
	GlobalMaxTx int `mapstructure:"global_max_tx"`

	// RetryInterval sets the interval between retry attempts
	RetryInterval time.Duration `mapstructure:"retry_interval"`

	// MaxRetries sets the maximum number of retry attempts
	MaxRetries int `mapstructure:"max_retries"`

	// EnableMetrics enables performance metrics collection
	EnableMetrics bool `mapstructure:"enable_metrics"`

	// MetricsInterval sets the metrics reporting interval
	MetricsInterval time.Duration `mapstructure:"metrics_interval"`

	// ReplacementGasPercent sets the minimum gas price increase for transaction replacement
	ReplacementGasPercent int `mapstructure:"replacement_gas_percent"`
}

// DefaultCacheQueueAppConfig returns default configuration
func DefaultCacheQueueAppConfig() CacheQueueAppConfig {
	return CacheQueueAppConfig{
		Cache: CacheQueueSettings{
			Enable:                true,
			MaxTxPerAccount:       1000,          // Increased for high-load production
			TxTimeout:             1 * time.Minute, // Reduced from 5 minutes
			CleanupInterval:       15 * time.Second, // More frequent cleanup
			NoncePollingInterval:  1 * time.Second, // Nonce polling interval
			GlobalMaxTx:           50000,        // Increased for high-load production
			RetryInterval:         500 * time.Millisecond, // Faster retry
			MaxRetries:            3,
			EnableMetrics:         true,
			MetricsInterval:       1 * time.Minute, // More frequent metrics reporting
			ReplacementGasPercent: 10,           // 10% minimum gas price increase for replacement
		},
	}
}

// ToCacheQueueConfig converts app config to internal config
func (c CacheQueueAppConfig) ToCacheQueueConfig() *CacheQueueConfig {
	return &CacheQueueConfig{
		Enable:                c.Cache.Enable,
		MaxTxPerAccount:       c.Cache.MaxTxPerAccount,
		TxTimeout:             c.Cache.TxTimeout,
		CleanupInterval:       c.Cache.CleanupInterval,
		NoncePollingInterval:  c.Cache.NoncePollingInterval,
		GlobalMaxTx:           c.Cache.GlobalMaxTx,
		RetryInterval:         c.Cache.RetryInterval,
		MaxRetries:            c.Cache.MaxRetries,
		ReplacementGasPercent: c.Cache.ReplacementGasPercent,
	}
}
