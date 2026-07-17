package authz

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the authz precompile. It follows the cosmos/evm precompile layout —
// Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch. The moca-specific
// surface is the hex (0x) address encoding, moca's method set, the custom
// authorization-parsing and exec-message codec, and the non-payable RejectValue
// guard. The authz keeper backs both the tx (Grant/Revoke/Exec) and query
// (Grants/GranterGrants/GranteeGrants) handlers directly.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	authzKeeper authzkeeper.Keeper
}

// NewPrecompile creates a new authz Precompile as a vm.PrecompiledContract. The authz
// keeper serves both the msg-server and querier handlers; the bank keeper backs the
// StateDB balance handler so coin moves stay reconciled.
func NewPrecompile(authzKeeper authzkeeper.Keeper, bankKeeper bankkeeper.Keeper) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      authzAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:         authzABI,
		authzKeeper: authzKeeper,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return authzAddress
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
// moves stay reconciled with the EVM StateDB. moca precompiles are not payable, so
// any attached value is rejected up front.
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
func (p Precompile) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte
	switch method.Name {
	// Authz transactions
	case GrantMethodName:
		bz, err = p.Grant(ctx, evm, contract, method, args)
	case RevokeMethodName:
		bz, err = p.Revoke(ctx, evm, contract, method, args)
	case ExecMethodName:
		bz, err = p.Exec(ctx, evm, contract, method, args)
	// Authz queries
	case GrantsMethodName:
		bz, err = p.Grants(ctx, method, args)
	case GranterGrantsMethodName:
		bz, err = p.GranterGrants(ctx, method, args)
	case GranteeGrantsMethodName:
		bz, err = p.GranteeGrants(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case GrantMethodName, RevokeMethodName, ExecMethodName:
		return true
	default:
		return false
	}
}
