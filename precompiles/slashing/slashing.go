package slashing

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the slashing precompile. It follows the cosmos/evm precompile
// layout — Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch. The
// moca-specific surface is the hex (0x) address encoding and the non-payable
// RejectValue guard.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	slashingMsgServer slashingtypes.MsgServer
	slashingQuerier   slashingtypes.QueryServer
}

// NewPrecompile creates a new slashing Precompile as a vm.PrecompiledContract.
// The msg server is built from the slashing keeper at wiring time and the keeper
// itself serves the queries; the bank keeper drives StateDB balance reconciliation.
func NewPrecompile(
	slashingMsgServer slashingtypes.MsgServer,
	slashingQuerier slashingtypes.QueryServer,
	bankKeeper bankkeeper.Keeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      slashingAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:               slashingABI,
		slashingMsgServer: slashingMsgServer,
		slashingQuerier:   slashingQuerier,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return slashingAddress
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

// Run dispatches the call through cosmos/evm's native-action protocol so keeper
// coin moves stay reconciled with the EVM StateDB. moca precompiles are not
// payable, so any attached value is rejected up front.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
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
	// Slashing transactions
	case UnjailMethod:
		bz, err = p.Unjail(ctx, evm, contract, method, args)
	// Slashing queries
	case SigningInfoMethod:
		bz, err = p.SigningInfo(ctx, method, args)
	case SigningInfosMethod:
		bz, err = p.SigningInfos(ctx, method, args)
	case ParamsMethod:
		bz, err = p.Params(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case UnjailMethod:
		return true
	default:
		return false
	}
}
