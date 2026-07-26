package bank

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the bank precompile. It follows the cosmos/evm precompile layout —
// Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch. The moca-specific
// surface is the hex (0x) address encoding, moca's method set, and the non-payable
// RejectValue guard. The bank msg server carries moca's payment keeper so that coin
// sends settle stream payments.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	bankMsgServer banktypes.MsgServer
	bankKeeper    bankkeeper.Keeper
}

// NewPrecompile creates a new bank Precompile as a vm.PrecompiledContract. The msg
// server is built from the bank and payment keepers at wiring time; the bank keeper
// serves queries and the StateDB balance handler.
func NewPrecompile(bankMsgServer banktypes.MsgServer, bankKeeper bankkeeper.Keeper) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      bankAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:           bankABI,
		bankMsgServer: bankMsgServer,
		bankKeeper:    bankKeeper,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return bankAddress
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
	// Bank transactions
	case SendMethodName:
		bz, err = p.Send(ctx, evm, contract, method, args)
	case MultiSendMethodName:
		bz, err = p.MultiSend(ctx, evm, contract, method, args)
	// Bank queries
	case BalanceMethodName:
		bz, err = p.Balance(ctx, method, args)
	case AllBalancesMethodName:
		bz, err = p.AllBalances(ctx, method, args)
	case TotalSupplyMethodName:
		bz, err = p.TotalSupply(ctx, method, args)
	case SpendableBalancesMethodName:
		bz, err = p.SpendableBalances(ctx, method, args)
	case SpendableBalanceByDenomMethodName:
		bz, err = p.SpendableBalanceByDenom(ctx, method, args)
	case SupplyOfMethodName:
		bz, err = p.SupplyOf(ctx, method, args)
	case ParamsMethodName:
		bz, err = p.Params(ctx, method, args)
	case DenomMetadataMethodName:
		bz, err = p.DenomMetadata(ctx, method, args)
	case DenomsMetadataMethodName:
		bz, err = p.DenomsMetadata(ctx, method, args)
	case DenomOwnersMethodName:
		bz, err = p.DenomOwners(ctx, method, args)
	case SendEnabledMethodName:
		bz, err = p.SendEnabled(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case SendMethodName, MultiSendMethodName:
		return true
	default:
		return false
	}
}
