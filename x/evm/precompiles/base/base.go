// Package base provides the shared runtime skeleton for moca's static
// precompiles. It is a thin wrapper over cosmos/evm's precompiles/common
// runtime (the official native execution model introduced with cosmos/evm
// v0.6.0), so that moca precompiles stop hand-rolling their own
// GetCacheContext / Snapshot / RevertToSnapshot / commit templates and instead
// share one snapshot, gas-metering, and balance-sync implementation.
//
// A moca precompile embeds base.Precompile, then implements the standard
// cosmos/evm shape:
//
//	func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
//		return c.RunPrecompile(evm, contract, readonly, c.Execute)
//	}
//
//	func (c *Contract) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
//		method, args, err := c.Dispatch(contract, readonly, c.IsTransaction)
//		if err != nil {
//			return nil, err
//		}
//		switch method.Name { ... }
//	}
package base

import (
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

// ExecuteFn dispatches decoded calldata to a precompile method and runs its
// business logic inside the native action's cache context. It receives the EVM
// so handlers can emit logs (moca precompiles log via evm.StateDB.AddLog).
type ExecuteFn func(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error)

// Precompile is moca's shared precompile runtime base. It embeds cosmos/evm's
// common.Precompile (the official snapshot / gas / balance-handler model) and
// the precompile ABI, and enforces moca's invariant that precompiles never
// accept native value.
type Precompile struct {
	cmn.Precompile
	abi.ABI
}

// Option customizes a Precompile at construction.
type Option func(*Precompile)

// WithBalanceHandler enables cosmos/evm's bank-event -> StateDB reconciliation.
// State-changing precompiles that move native balances opt in; query-only and
// non-balance precompiles leave it unset.
func WithBalanceHandler(bankKeeper cmn.BankKeeper) Option {
	return func(p *Precompile) {
		p.BalanceHandlerFactory = cmn.NewBalanceHandlerFactory(bankKeeper)
	}
}

// New builds a moca precompile base at the given address with the given ABI.
// The KV gas configs are intentionally empty: like cosmos/evm's own precompiles,
// moca charges a flat per-method cost via each precompile's RequiredGas, so the
// base must not additionally charge store gas inside RunNativeAction.
func New(address common.Address, contractABI abi.ABI, opts ...Option) Precompile {
	p := Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.GasConfig{},
			TransientKVGasConfig: storetypes.GasConfig{},
			ContractAddress:      address,
		},
		ABI: contractABI,
	}
	for _, opt := range opts {
		opt(&p)
	}
	return p
}

// RunPrecompile is the unified entrypoint for a moca precompile's Run. It rejects
// native value up front (moca precompiles are not payable) and then delegates to
// cosmos/evm's RunNativeAction, which manages the cache context, multistore
// snapshot/revert, gas metering, and the optional balance handler, converting any
// action error into an EVM revert.
func (p Precompile) RunPrecompile(evm *vm.EVM, contract *vm.Contract, readonly bool, exec ExecuteFn) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return cmn.ReturnRevertError(evm, err)
	}
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return exec(ctx, evm, contract, readonly)
	})
}

// Dispatch decodes calldata to an ABI method, enforcing read-only write
// protection, and unpacks its arguments. It is a thin pass-through to
// cosmos/evm's SetupABI. isTx reports whether a method mutates state.
func (p Precompile) Dispatch(contract *vm.Contract, readonly bool, isTx func(*abi.Method) bool) (*abi.Method, []interface{}, error) {
	return cmn.SetupABI(p.ABI, contract, readonly, isTx)
}
