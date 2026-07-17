package permission

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
	permissiontypes "github.com/mocachain/moca/v2/x/permission/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the permission precompile. It follows the cosmos/evm precompile layout —
// Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch. The moca-specific surface
// is the hex (0x) address encoding and the non-payable RejectValue guard. The module
// exposes only the params query (its tx methods were removed) and emits no events.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	permissionQuerier permissiontypes.QueryServer
}

// NewPrecompile creates a new permission Precompile as a vm.PrecompiledContract. The
// querier serves the params query; the bank keeper drives the StateDB balance
// reconciliation.
func NewPrecompile(permissionQuerier permissiontypes.QueryServer, bankKeeper bankkeeper.Keeper) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      permissionAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:               permissionABI,
		permissionQuerier: permissionQuerier,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return permissionAddress
}

// RequiredGas calculates the base gas via the cosmos/evm common flat+per-byte model.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}

	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run dispatches the call through cosmos/evm's native-action protocol so keeper coin
// moves stay reconciled with the EVM StateDB. moca precompiles are not payable, so any
// attached value is rejected up front.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm, contract, readonly)
	})
}

// Execute parses the calldata against the ABI and routes to the matching handler.
func (p Precompile) Execute(ctx sdk.Context, _ *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte
	switch method.Name {
	// Permission queries
	case ParamsMethodName:
		bz, err = p.Params(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
// The permission module has no transaction methods, so this is always false.
func (Precompile) IsTransaction(_ *abi.Method) bool {
	return false
}
