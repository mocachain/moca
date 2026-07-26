package flags

import (
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Tendermint/cosmos-sdk full-node start flags
const (
	WithCometBFT = "with-cometbft"
	Address      = "address"
	Transport    = "transport"
	TraceStore   = "trace-store"
	CPUProfile   = "cpu-profile"
	// The type of database for application and snapshots databases
	AppDBBackend = "app-db-backend"
)

// GRPC-related flags.
const (
	GRPCOnly       = "grpc-only"
	GRPCEnable     = "grpc.enable"
	GRPCAddress    = "grpc.address"
	GRPCWebEnable  = "grpc-web.enable"
	GRPCWebAddress = "grpc-web.address"
)

// Cosmos API flags
const (
	RPCEnable         = "api.enable"
	EnabledUnsafeCors = "api.enabled-unsafe-cors"
)

// JSON-RPC flags
const (
	JSONRPCEnable               = "json-rpc.enable"
	JSONRPCAPI                  = "json-rpc.api"
	JSONRPCAddress              = "json-rpc.address"
	JSONWsAddress               = "json-rpc.ws-address"
	JSONWsOrigins               = "json-rpc.ws-origins"
	JSONRPCGasCap               = "json-rpc.gas-cap"
	JSONRPCEVMTimeout           = "json-rpc.evm-timeout"
	JSONRPCTxFeeCap             = "json-rpc.txfee-cap"
	JSONRPCFilterCap            = "json-rpc.filter-cap"
	JSONRPCLogsCap              = "json-rpc.logs-cap"
	JSONRPCBlockRangeCap        = "json-rpc.block-range-cap"
	JSONRPCHTTPTimeout          = "json-rpc.http-timeout"
	JSONRPCHTTPIdleTimeout      = "json-rpc.http-idle-timeout"
	JSONRPCAllowUnprotectedTxs  = "json-rpc.allow-unprotected-txs"
	JSONRPCMaxOpenConnections   = "json-rpc.max-open-connections"
	JSONRPCEnableIndexer        = "json-rpc.enable-indexer"
	JSONRPCEnableProfiling      = "json-rpc.enable-profiling"
	JSONRPCAllowInsecureUnlock  = "json-rpc.allow-insecure-unlock"
	JSONRPCBatchRequestLimit    = "json-rpc.batch-request-limit"
	JSONRPCBatchResponseMaxSize = "json-rpc.batch-response-max-size"
	// JSONRPCEnableMetrics enables EVM RPC metrics server.
	// Set to `metrics` which is hardcoded flag from go-ethereum.
	// https://github.com/ethereum/go-ethereum/blob/master/metrics/metrics.go#L35-L55
	JSONRPCEnableMetrics = "metrics"
)

// EVM flags
const (
	EVMTracer         = "evm.tracer"
	EVMMaxTxGasWanted = "evm.max-tx-gas-wanted"
	// EVMChainID is the EIP-155 chain ID baked into the cosmos/evm keeper at
	// construction. When unset (0), cosmos/evm falls back to its default
	// (262144). The throwaway app built by the root command leaves this
	// unset, so the keeper's one-time SetChainConfig transitions default ->
	// configured value on the real app without tripping the "already set"
	// guard. Configure per-network in app.toml (devnet 5151 / testnet
	// 222888 / mainnet 2288).
	EVMChainID = "evm.evm-chain-id"
)

// TLS flags
const (
	TLSCertPath = "tls.certificate-path"
	TLSKeyPath  = "tls.key-path"
)

// AddTxFlags adds common flags for commands to post tx
func AddTxFlags(cmd *cobra.Command) (*cobra.Command, error) {
	cmd.PersistentFlags().String(flags.FlagChainID, "", "Specify Chain ID for sending Tx")
	cmd.PersistentFlags().String(flags.FlagFrom, "", "Name or address of private key with which to sign")
	cmd.PersistentFlags().String(flags.FlagFees, "", "Fees to pay along with transaction; eg: 10amoca")
	cmd.PersistentFlags().String(flags.FlagGasPrices, "", "Gas prices to determine the transaction fee (e.g. 10amoca)")
	cmd.PersistentFlags().String(flags.FlagNode, "tcp://localhost:26657", "<host>:<port> to tendermint rpc interface for this chain")
	cmd.PersistentFlags().String(flags.FlagEvmNode, "http://localhost:8545", "<host>:<port> to EVM RPC interface for this chain")
	cmd.PersistentFlags().Float64(flags.FlagGasAdjustment, flags.DefaultGasAdjustment, "adjustment factor to be multiplied against the estimate returned by the tx simulation; if the gas limit is set manually this flag is ignored ")
	cmd.PersistentFlags().StringP(flags.FlagBroadcastMode, "b", flags.BroadcastSync, "Transaction broadcasting mode (sync|async)")
	cmd.PersistentFlags().String(flags.FlagKeyringBackend, keyring.BackendOS, "Select keyring's backend")

	// --gas can accept integers and "simulate"
	// cmd.PersistentFlags().Var(&flags.GasFlagVar, "gas", fmt.Sprintf(
	//	"gas limit to set per-transaction; set to %q to calculate required gas automatically (default %d)",
	//	flags.GasFlagAuto, flags.DefaultGasLimit,
	// ))
	cmd.PersistentFlags().String(flags.FlagGas, "", "gas limit to set per-transaction")

	// viper.BindPFlag(flags.FlagTrustNode, cmd.Flags().Lookup(flags.FlagTrustNode))
	if err := viper.BindPFlag(flags.FlagNode, cmd.PersistentFlags().Lookup(flags.FlagNode)); err != nil {
		return nil, err
	}
	if err := viper.BindPFlag(flags.FlagKeyringBackend, cmd.PersistentFlags().Lookup(flags.FlagKeyringBackend)); err != nil {
		return nil, err
	}
	return cmd, nil
}
