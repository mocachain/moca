// Package v2 holds the committed, per-network parameter snapshots loaded by the
// moca v2.0.0 in-place upgrade handler (app.migrateToV2). The in-tree x/evm +
// x/feemarket params are wire-incompatible with cosmos/evm's, so they are
// overwritten wholesale during the upgrade; rather than hard-coding the values
// in Go, the handler reads the snapshot for the network it is running on
// (selected by chain-id) so a single binary serves devnet/testnet/mainnet and
// each network's values are reviewable in version control.
//
// The snapshots are canonical proto-JSON and are fed verbatim to the codec.
// Regenerate a network's snapshot from `mocad query evm params` /
// `mocad query feemarket params` against the live chain before that network's
// upgrade if its params were governance-tuned away from these defaults.
package v2

import (
	"embed"
	"fmt"
)

// Network is one of moca's deployment networks.
type Network string

const (
	Devnet  Network = "devnet"
	Testnet Network = "testnet"
	Mainnet Network = "mainnet"
)

// chainIDToNetwork maps the cosmos chain-id of each moca network to its Network.
// Unknown chain-ids (local/test chains use ad-hoc ids) fall back to Devnet — see
// NetworkForChainID.
var chainIDToNetwork = map[string]Network{
	"moca_5151-1":   Devnet,
	"moca_222888-1": Testnet,
	"moca_2288-1":   Mainnet,
}

// NetworkForChainID maps a chain-id to its moca Network. Unknown chain-ids
// (local devnets, in-process test chains with ad-hoc ids) fall back to Devnet so
// the upgrade handler never panics on an unrecognised id.
func NetworkForChainID(chainID string) Network {
	if n, ok := chainIDToNetwork[chainID]; ok {
		return n
	}
	return Devnet
}

//go:embed params/*/evm.json params/*/feemarket.json
var paramsFS embed.FS

// EVMParamsJSON returns the canonical proto-JSON for the x/vm (EVM) params of the
// given network, ready to be fed to codec.UnmarshalJSON into evmtypes.Params.
func EVMParamsJSON(n Network) ([]byte, error) {
	return readParam(n, "evm.json")
}

// FeeMarketParamsJSON returns the canonical proto-JSON for the x/feemarket params
// of the given network, ready to be fed to codec.UnmarshalJSON into
// feemarkettypes.Params.
func FeeMarketParamsJSON(n Network) ([]byte, error) {
	return readParam(n, "feemarket.json")
}

func readParam(n Network, file string) ([]byte, error) {
	bz, err := paramsFS.ReadFile(fmt.Sprintf("params/%s/%s", n, file))
	if err != nil {
		return nil, fmt.Errorf("v2 params: read %s/%s: %w", n, file, err)
	}
	return bz, nil
}
