// Package evm previously held moca's bespoke EVM-tx AnteHandler decorators
// (modeled on evmos v12's app/ante/evm package). With the cosmos/evm v0.6.0
// migration the decorators themselves are gone: the EVM ante pipeline is now
// the upstream Mono decorator from github.com/cosmos/evm/ante/evm. This file
// retains just the keeper-interface aliases so the few moca packages that
// still hold a reference to the old type names (app/ante/cosmos/min_price.go,
// testutil/statedb.go) compile unchanged.
package evm

import (
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
)

// EVMKeeper is the cosmos/evm v0.6.0 ante EVMKeeper surface. The previous
// moca-specific interface accepted a vm.EVMLogger tracer; the v0.6.0 keeper
// uses *tracing.Hooks instead, matching geth v1.15.
type EVMKeeper = anteinterfaces.EVMKeeper

// FeeMarketKeeper is the cosmos/evm v0.6.0 ante FeeMarketKeeper surface.
type FeeMarketKeeper = anteinterfaces.FeeMarketKeeper
