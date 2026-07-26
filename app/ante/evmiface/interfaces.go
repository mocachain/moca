// Package evmiface re-exports the cosmos/evm v0.6.0 ante keeper interfaces
// under stable moca-local names. It exists only so the few moca packages that
// still refer to the old type names (app/ante/cosmos/min_price.go,
// testutil/statedb.go) compile unchanged after the cosmos/evm migration.
//
// Pre-migration moca shipped its own EVM-tx AnteHandler decorators (modeled on
// evmos v12's app/ante/evm package) together with these keeper interfaces. The
// decorators are gone: the EVM ante pipeline is now the upstream Mono decorator
// from github.com/cosmos/evm/ante/evm, wired up in app/ante/handler_options.go.
package evmiface

import (
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
)

// EVMKeeper is the cosmos/evm v0.6.0 ante EVMKeeper surface. The previous
// moca-specific interface accepted a vm.EVMLogger tracer; the v0.6.0 keeper
// uses *tracing.Hooks instead, matching geth v1.15.
type EVMKeeper = anteinterfaces.EVMKeeper

// FeeMarketKeeper is the cosmos/evm v0.6.0 ante FeeMarketKeeper surface.
type FeeMarketKeeper = anteinterfaces.FeeMarketKeeper
