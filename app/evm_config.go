package app

import (
	"fmt"
	"path/filepath"

	cmttypes "github.com/cometbft/cometbft/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// evmConfigSealed guards the one-time global EVM configuration. NewEvmos may be
// invoked more than once in a single process (e.g. the autocli bootstrap app
// and the real application), but cosmos/evm's EVMConfigurator can only be
// applied once.
var evmConfigSealed bool

// mocaEVMCoinInfo describes moca's EVM gas token: 1 MOCA = 10^18 amoca.
var mocaEVMCoinInfo = evmtypes.EvmCoinInfo{
	Denom:         "amoca",
	ExtendedDenom: "amoca",
	DisplayDenom:  "moca",
	Decimals:      evmtypes.EighteenDecimals,
}

// resolveEVMChainID determines the chain id used to configure cosmos/evm.
//
// baseapp's chain id is preferred, but it is empty until InitChain on the node
// start path, so this falls back to the chain_id recorded in the on-disk
// genesis file (which is authoritative for both fresh starts and restarts).
func resolveEVMChainID(homePath, baseappChainID string) string {
	if baseappChainID != "" {
		return baseappChainID
	}
	if homePath == "" {
		return ""
	}
	genDoc, err := cmttypes.GenesisDocFromFile(filepath.Join(homePath, "config", "genesis.json"))
	if err != nil {
		return ""
	}
	return genDoc.ChainID
}

// configureEVM applies cosmos/evm's global configuration — the EVM chain config
// and coin info — that the x/vm module, keeper and ante handlers depend on.
//
// It must run before any transaction is processed: cosmos/evm's
// GetEthChainConfig panics with a nil dereference if the chain config has not
// been set. The configuration is sealed after the first successful call.
//
// An empty chainID (e.g. the autocli bootstrap app) is skipped without sealing,
// so the real application can still apply the configuration afterwards.
func configureEVM(chainID string) error {
	if evmConfigSealed || chainID == "" {
		return nil
	}

	ethCfg := evmtypes.DefaultChainConfig(chainID)
	if err := evmtypes.NewEVMConfigurator().
		WithChainConfig(ethCfg).
		WithEVMCoinInfo(mocaEVMCoinInfo).
		Configure(); err != nil {
		return fmt.Errorf("failed to configure cosmos/evm: %w", err)
	}

	evmConfigSealed = true
	return nil
}
