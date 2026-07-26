package config

// DefaultConfigTemplate defines the configuration template for the EVM RPC configuration
const DefaultConfigTemplate = `
###############################################################################
###                             EVM Configuration                           ###
###############################################################################

[evm]

# Tracer defines the 'vm.Tracer' type that the EVM will use when the node is run in
# debug mode. To enable tracing use the '--evm.tracer' flag when starting your node.
# Valid types are: json|struct|access_list|markdown
tracer = "{{ .EVM.Tracer }}"

# MaxTxGasWanted defines the gas wanted for each eth tx returned in ante handler in check tx mode.
max-tx-gas-wanted = {{ .EVM.MaxTxGasWanted }}

# EVMChainID is the EIP-155 EVM chain ID baked into the cosmos/evm keeper.
# Set it per network: devnet 5151, testnet 222888, mainnet 2288.
# Leaving it 0 makes cosmos/evm fall back to its default (262144), which is wrong for a live network.
evm-chain-id = {{ .EVM.EVMChainID }}

###############################################################################
###                           JSON RPC Configuration                        ###
###############################################################################

[json-rpc]

# Enable defines if the gRPC server should be enabled.
enable = {{ .JSONRPC.Enable }}

# Address defines the EVM RPC HTTP server address to bind to.
address = "{{ .JSONRPC.Address }}"

# Address defines the EVM WebSocket server address to bind to.
ws-address = "{{ .JSONRPC.WsAddress }}"

# ws-origins defines the allowed Origin headers for WebSocket (eth_subscribe)
# connections. Non-browser clients (empty Origin) are always allowed; browser
# clients served from other origins must be listed here. An empty list rejects all.
ws-origins = [{{range $index, $elmt := .JSONRPC.WSOrigins}}{{if $index}}, {{end}}"{{$elmt}}"{{end}}]

# API defines a list of JSON-RPC namespaces that should be enabled
# Example: "eth,txpool,personal,net,debug,web3"
api = "{{range $index, $elmt := .JSONRPC.API}}{{if $index}},{{$elmt}}{{else}}{{$elmt}}{{end}}{{end}}"

# GasCap sets a cap on gas that can be used in eth_call/estimateGas (0=infinite). Default: 25,000,000.
gas-cap = {{ .JSONRPC.GasCap }}

# EVMTimeout is the global timeout for eth_call. Default: 5s.
evm-timeout = "{{ .JSONRPC.EVMTimeout }}"

# TxFeeCap is the global tx-fee cap for send transaction. Default: 1eth.
txfee-cap = {{ .JSONRPC.TxFeeCap }}

# FilterCap sets the global cap for total number of filters that can be created
filter-cap = {{ .JSONRPC.FilterCap }}

# FeeHistoryCap sets the global cap for total number of blocks that can be fetched
feehistory-cap = {{ .JSONRPC.FeeHistoryCap }}

# LogsCap defines the max number of results can be returned from single 'eth_getLogs' query.
logs-cap = {{ .JSONRPC.LogsCap }}

# BlockRangeCap defines the max block range allowed for 'eth_getLogs' query.
block-range-cap = {{ .JSONRPC.BlockRangeCap }}

# HTTPTimeout is the read/write timeout of http json-rpc server.
http-timeout = "{{ .JSONRPC.HTTPTimeout }}"

# HTTPIdleTimeout is the idle timeout of http json-rpc server.
http-idle-timeout = "{{ .JSONRPC.HTTPIdleTimeout }}"

# AllowUnprotectedTxs restricts unprotected (non EIP155 signed) transactions to be submitted via
# the node's RPC when the global parameter is disabled.
allow-unprotected-txs = {{ .JSONRPC.AllowUnprotectedTxs }}

# AllowInsecureUnlock enables keyring-backed account RPCs
# (eth_accounts, eth_sendTransaction, personal_*). Public nodes SHOULD set this to false.
allow-insecure-unlock = {{ .JSONRPC.AllowInsecureUnlock }}

# BatchRequestLimit is the maximum number of calls in a single JSON-RPC batch request (0 = unlimited).
batch-request-limit = {{ .JSONRPC.BatchRequestLimit }}

# BatchResponseMaxSize is the maximum size in bytes of a JSON-RPC batch response (0 = unlimited).
batch-response-max-size = {{ .JSONRPC.BatchResponseMaxSize }}

# MaxOpenConnections sets the maximum number of simultaneous connections
# for the server listener.
max-open-connections = {{ .JSONRPC.MaxOpenConnections }}

# EnableIndexer enables the custom transaction indexer for the EVM (ethereum transactions).
enable-indexer = {{ .JSONRPC.EnableIndexer }}

# MetricsAddress defines the EVM Metrics server address to bind to. Pass --metrics in CLI to enable
# Prometheus metrics path: /debug/metrics/prometheus
metrics-address = "{{ .JSONRPC.MetricsAddress }}"

# EnableProfiling enables the profiling endpoints in the 'debug' JSON-RPC namespace
# (debug_startCPUProfile, debug_blockProfile, ...). SHOULD NOT be enabled on
# publicly exposed nodes.
enable-profiling = {{ .JSONRPC.EnableProfiling }}

###############################################################################
###                             TLS Configuration                           ###
###############################################################################

[tls]

# Certificate path defines the cert.pem file path for the TLS configuration.
certificate-path = "{{ .TLS.CertificatePath }}"

# Key path defines the key.pem file path for the TLS configuration.
key-path = "{{ .TLS.KeyPath }}"
`

var DefaultCustomAppTemplate = `
###############################################################################
###                           PaymentCheck Config                           ###
###############################################################################
[payment-check]
# enabled - the flag to enable/disable payment check
enabled = {{ .PaymentCheck.Enabled }}
# interval - the block interval run check payment
interval = {{ .PaymentCheck.Interval }}

###############################################################################
###                                Hardforks                                ###
###############################################################################
[hardforks]
# Map of "height" = { name = "...", info = "..." } used to schedule x/upgrade plans
# automatically at BeginBlock when the configured height is reached.
#
# This is intended for operator-managed / localnet / emergency upgrades where governance
# is unavailable.
#
# Fields:
#   name - (required) the upgrade plan name
#   info - (optional) application-specific upgrade info, e.g. JSON for Cosmovisor
#
# Example:
# "1200" = { name = "testnet-gov-param-fix", info = '{"binaries":{"linux/amd64":"url..."}}' }
# "5000" = { name = "another-upgrade" }
`
