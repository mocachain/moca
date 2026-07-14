package bank

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/x/evm/precompiles/base"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
	paymentkeeper "github.com/mocachain/moca/v2/x/payment/keeper"
)

type Contract struct {
	base.Precompile

	bankKeeper    bankkeeper.Keeper
	paymentKeeper paymentkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
// It wires the balance handler so bank coin movements reconcile back to the EVM StateDB.
func NewPrecompiledContract(bankKeeper bankkeeper.Keeper, paymentKeeper paymentkeeper.Keeper) *Contract {
	return &Contract{
		Precompile:    base.New(bankAddress, bankABI, base.WithBalanceHandler(bankKeeper)),
		bankKeeper:    bankKeeper,
		paymentKeeper: paymentKeeper,
	}
}

func (c *Contract) calculateMultiSendGas(input []byte) uint64 {
	if len(input) < 4 {
		return MultiSendBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return MultiSendBaseGas
	}

	var args MultiSendArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return MultiSendBaseGas
	}

	numOutputs := uint64(len(args.Outputs))
	if numOutputs > MaxMultiSendOutputs {
		numOutputs = MaxMultiSendOutputs
	}

	totalCoins := uint64(0)
	for i, output := range args.Outputs {
		if i >= MaxMultiSendOutputs {
			break
		}
		totalCoins += uint64(len(output.Amount))
	}

	return MultiSendBaseGas + (numOutputs * MultiSendPerOutputGas) + (totalCoins * MultiSendPerCoinGas)
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case SendMethodName:
		return SendGas
	case MultiSendMethodName:
		return c.calculateMultiSendGas(input)
	case BalanceMethodName:
		return BalanceGas
	case AllBalancesMethodName:
		return AllBalancesGas
	case TotalSupplyMethodName:
		return TotalSupplyGas
	case SpendableBalancesMethodName:
		return SpendableBalancesGas
	case SpendableBalanceByDenomMethodName:
		return SpendableBalanceByDenomGas
	case SupplyOfMethodName:
		return SupplyOfGas
	case ParamsMethodName:
		return ParamsGas
	case DenomMetadataMethodName:
		return DenomMetadataGas
	case DenomsMetadataMethodName:
		return DenomsMetadataGas
	case DenomOwnersMethodName:
		return DenomOwnersGas
	case SendEnabledMethodName:
		return SendEnabledGas
	default:
		return 0
	}
}

// Run is the precompile entrypoint. The base rejects native value, sets up the
// native cache context / snapshot / gas metering / balance handler, and reverts
// on error; the per-method business logic runs in Execute.
func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return c.RunPrecompile(evm, contract, readonly, c.Execute)
}

// Execute dispatches the ABI method to its handler. Read-only write protection is
// enforced by the base Dispatch (SetupABI) using IsTransaction.
func (c *Contract) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	method, _, err := c.Dispatch(contract, readonly, c.IsTransaction)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case SendMethodName:
		return c.Send(ctx, evm, contract, readonly)
	case MultiSendMethodName:
		return c.MultiSend(ctx, evm, contract, readonly)
	case BalanceMethodName:
		return c.Balance(ctx, evm, contract, readonly)
	case AllBalancesMethodName:
		return c.AllBalances(ctx, evm, contract, readonly)
	case TotalSupplyMethodName:
		return c.TotalSupply(ctx, evm, contract, readonly)
	case SpendableBalancesMethodName:
		return c.SpendableBalances(ctx, evm, contract, readonly)
	case SpendableBalanceByDenomMethodName:
		return c.SpendableBalanceByDenom(ctx, evm, contract, readonly)
	case SupplyOfMethodName:
		return c.SupplyOf(ctx, evm, contract, readonly)
	case ParamsMethodName:
		return c.Params(ctx, evm, contract, readonly)
	case DenomMetadataMethodName:
		return c.DenomMetadata(ctx, evm, contract, readonly)
	case DenomsMetadataMethodName:
		return c.DenomsMetadata(ctx, evm, contract, readonly)
	case DenomOwnersMethodName:
		return c.DenomOwners(ctx, evm, contract, readonly)
	case SendEnabledMethodName:
		return c.SendEnabled(ctx, evm, contract, readonly)
	default:
		return nil, fmt.Errorf("method %s is not handled", method.Name)
	}
}

// IsTransaction reports whether a method mutates state. It drives read-only write
// protection in the base Dispatch.
func (Contract) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case SendMethodName, MultiSendMethodName:
		return true
	default:
		return false
	}
}

func (c *Contract) AddLog(evm *vm.EVM, event abi.Event, topics []common.Hash, args ...interface{}) error {
	data, newTopic, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     c.Address(),
		Topics:      newTopic,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}
