package storageprovider

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	spkeeper "github.com/mocachain/moca/v2/x/sp/keeper"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type (
	precompiledContractFunc func(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error)
	Contract                struct {
		cmn.Precompile
		spKeeper  spkeeper.Keeper
		handlers  map[string]precompiledContractFunc
		gasMeters map[string]uint64
		events    map[string]string
	}
)

// NewPrecompiledContract returns a new static precompile instance.
func NewPrecompiledContract(spKeeper spkeeper.Keeper, bankKeeper bankkeeper.Keeper) *Contract {
	c := &Contract{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      spAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		spKeeper:  spKeeper,
		handlers:  make(map[string]precompiledContractFunc),
		gasMeters: make(map[string]uint64),
		events:    make(map[string]string),
	}
	c.registerTx()
	c.registerQuery()
	return c
}

func (c *Contract) Address() common.Address {
	return spAddress
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}
	return c.gasMeters[method.Name]
}

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	// Route dispatch through cosmos/evm's native-action protocol so keeper coin
	// moves stay reconciled with the EVM StateDB: FlushToCacheCtx + the
	// BalanceHandler translate the bank coin_spent/coin_received events into
	// StateDB SubBalance/AddBalance, the multistore is snapshotted for atomic
	// revert (AddPrecompileFn), and store gas is metered against contract.Gas.
	// Without this, StateDB.Commit's balance reconciliation would mint a debited
	// amount back to a 7702-dirtied caller (native-token inflation).
	return c.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return c.execute(ctx, evm, contract, readonly)
	})
}

func (c *Contract) execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	method, err := GetMethodByID(contract.Input)
	if err != nil {
		return nil, err
	}
	handler, ok := c.handlers[method.Name]
	if !ok {
		return nil, fmt.Errorf("method %s is not handled", method.Name)
	}
	return handler(ctx, evm, contract, readonly)
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

func (c *Contract) AddOtherLog(evm *vm.EVM, event abi.Event, address common.Address, topics []common.Hash, args ...interface{}) error {
	data, newTopic, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     address,
		Topics:      newTopic,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}

func (c *Contract) registerMethod(methodName string, gas uint64, handler precompiledContractFunc, eventName string) {
	method, ok := spABI.Methods[methodName]
	if !ok {
		panic(fmt.Errorf("method %s is not exist", methodName))
	}
	c.handlers[method.Name] = handler
	c.gasMeters[method.Name] = gas
	if eventName != "" {
		if _, ok := spABI.Events[eventName]; !ok {
			panic(fmt.Errorf("event %s is not exist", eventName))
		}
		c.events[method.Name] = eventName
	}
}
