package app

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// precompileBuilder constructs a moca precompile instance bound to a specific
// sdk.Context. moca's precompiles capture the context at construction time.
type precompileBuilder func(ctx sdk.Context) vm.PrecompiledContract

// ctxBoundPrecompile adapts moca's per-context precompile model to cosmos/evm's
// static precompile registry.
//
// cosmos/evm's x/vm keeper holds a single vm.PrecompiledContract instance per
// address (registered once via WithStaticPrecompiles). moca's precompiles, by
// contrast, must be rebuilt with the live sdk.Context for each invocation. This
// adapter bridges the gap: on every call it recovers the active context from
// the EVM's StateDB and rebuilds the underlying precompile, preserving moca's
// original per-context execution semantics without modifying any precompile.
type ctxBoundPrecompile struct {
	addr  common.Address
	build precompileBuilder
}

func newCtxBoundPrecompile(addr common.Address, build precompileBuilder) *ctxBoundPrecompile {
	return &ctxBoundPrecompile{addr: addr, build: build}
}

// Address implements vm.ContractRef.
func (p *ctxBoundPrecompile) Address() common.Address { return p.addr }

// RequiredGas implements vm.PrecompiledContract. moca's precompile gas
// schedules are derived from the call input alone and do not depend on the
// context, so a throwaway context is sufficient to build the instance here.
func (p *ctxBoundPrecompile) RequiredGas(input []byte) uint64 {
	return p.build(sdk.Context{}).RequiredGas(input)
}

// Run implements vm.PrecompiledContract. The active sdk.Context is recovered
// from the EVM StateDB so the precompile executes against current chain state.
func (p *ctxBoundPrecompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return nil, errors.New("evm precompile: StateDB is not a cosmos/evm *statedb.StateDB")
	}
	return p.build(stateDB.GetContext()).Run(evm, contract, readonly)
}
