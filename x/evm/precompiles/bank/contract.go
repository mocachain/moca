package bank

import (
	"fmt"

	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
	paymentkeeper "github.com/mocachain/moca/v2/x/payment/keeper"
)

type Contract struct {
	bankKeeper    bankkeeper.Keeper
	paymentKeeper paymentkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(bankKeeper bankkeeper.Keeper, paymentKeeper paymentkeeper.Keeper) *Contract {
	return &Contract{
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

func (c *Contract) Address() common.Address {
	return bankAddress
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

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) (ret []byte, err error) {
	if err = types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	// Pull the live SDK context from the EVM StateDB (static precompiles don't bind ctx at construction).
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return types.PackRetError("bank precompile must run within the cosmos/evm StateDB")
	}
	cacheCtx, err := stateDB.GetCacheContext()
	if err != nil {
		return types.PackRetError(err.Error())
	}
	ctx, commit := cacheCtx.CacheContext()
	snapshot := evm.StateDB.Snapshot()

	method, err := GetMethodByID(contract.Input)
	if err == nil {
		switch method.Name {
		case SendMethodName:
			ret, err = c.Send(ctx, evm, contract, readonly)
		case MultiSendMethodName:
			ret, err = c.MultiSend(ctx, evm, contract, readonly)
		case BalanceMethodName:
			ret, err = c.Balance(ctx, evm, contract, readonly)
		case AllBalancesMethodName:
			ret, err = c.AllBalances(ctx, evm, contract, readonly)
		case TotalSupplyMethodName:
			ret, err = c.TotalSupply(ctx, evm, contract, readonly)
		case SpendableBalancesMethodName:
			ret, err = c.SpendableBalances(ctx, evm, contract, readonly)
		case SpendableBalanceByDenomMethodName:
			ret, err = c.SpendableBalanceByDenom(ctx, evm, contract, readonly)
		case SupplyOfMethodName:
			ret, err = c.SupplyOf(ctx, evm, contract, readonly)
		case ParamsMethodName:
			ret, err = c.Params(ctx, evm, contract, readonly)
		case DenomMetadataMethodName:
			ret, err = c.DenomMetadata(ctx, evm, contract, readonly)
		case DenomsMetadataMethodName:
			ret, err = c.DenomsMetadata(ctx, evm, contract, readonly)
		case DenomOwnersMethodName:
			ret, err = c.DenomOwners(ctx, evm, contract, readonly)
		case SendEnabledMethodName:
			ret, err = c.SendEnabled(ctx, evm, contract, readonly)
		default:
			err = fmt.Errorf("method %s is not handled", method.Name)
		}
	}

	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		return types.PackRetError(err.Error())
	}

	commit()
	return ret, nil
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
