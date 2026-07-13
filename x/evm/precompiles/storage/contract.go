package storage

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type (
	precompiledContractFunc func(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error)
	Contract                struct {
		cmn.Precompile
		storageKeeper storagekeeper.Keeper
		handlers      map[string]precompiledContractFunc
		gasMeters     map[string]uint64
		events        map[string]string
	}
)

// NewPrecompiledContract returns a new static precompile instance.
func NewPrecompiledContract(storageKeeper storagekeeper.Keeper, bankKeeper bankkeeper.Keeper) *Contract {
	c := &Contract{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      storageAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		storageKeeper: storageKeeper,
		handlers:      make(map[string]precompiledContractFunc),
		gasMeters:     make(map[string]uint64),
		events:        make(map[string]string),
	}
	c.registerQuery()
	c.registerTx()
	return c
}

func (c *Contract) Address() common.Address {
	return storageAddress
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case PutPolicyMethodName:
		return c.calculatePutPolicyGas(input)
	case RenewGroupMemberMethodName:
		return c.calculateRenewGroupMemberGas(input)
	case UpdateGroupMethodName:
		return c.calculateUpdateGroupGas(input)
	case DiscontinueObjectMethodName:
		return c.calculateDiscontinueObjectGas(input)
	default:
		return c.gasMeters[method.Name]
	}
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

func (c *Contract) calculatePutPolicyGas(input []byte) uint64 {
	if len(input) < 4 {
		return PutPolicyBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return PutPolicyBaseGas
	}

	var args PutPolicyArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return PutPolicyBaseGas
	}

	numStatements := uint64(len(args.Statements))
	if numStatements > MaxPolicyStatements {
		numStatements = MaxPolicyStatements
	}

	totalActions := uint64(0)
	totalResources := uint64(0)
	for i, statement := range args.Statements {
		if i >= MaxPolicyStatements {
			break
		}
		totalActions += uint64(len(statement.Actions))
		totalResources += uint64(len(statement.Resources))
	}

	return PutPolicyBaseGas + (numStatements * PutPolicyPerStatementGas) +
		(totalActions * PutPolicyPerActionGas) + (totalResources * PutPolicyPerResourceGas)
}

func (c *Contract) calculateRenewGroupMemberGas(input []byte) uint64 {
	if len(input) < 4 {
		return RenewGroupMemberBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return RenewGroupMemberBaseGas
	}

	var args RenewGroupMemberArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return RenewGroupMemberBaseGas
	}

	numMembers := uint64(len(args.Members))
	if numMembers > MaxRenewGroupMembers {
		numMembers = MaxRenewGroupMembers
	}

	return RenewGroupMemberBaseGas + (numMembers * RenewGroupMemberPerMemberGas)
}

func (c *Contract) calculateUpdateGroupGas(input []byte) uint64 {
	if len(input) < 4 {
		return UpdateGroupBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return UpdateGroupBaseGas
	}

	var args UpdateGroupArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return UpdateGroupBaseGas
	}

	totalMembers := uint64(len(args.MembersToAdd) + len(args.MembersToDelete))
	if totalMembers > MaxUpdateGroupMembers {
		totalMembers = MaxUpdateGroupMembers
	}

	return UpdateGroupBaseGas + (totalMembers * UpdateGroupPerMemberGas)
}

func (c *Contract) calculateDiscontinueObjectGas(input []byte) uint64 {
	if len(input) < 4 {
		return DiscontinueObjectBaseGas
	}

	method, err := GetMethodByID(input)
	if err != nil {
		return DiscontinueObjectBaseGas
	}

	var args DiscontinueObjectArgs
	err = types.ParseMethodArgs(method, &args, input[4:])
	if err != nil {
		return DiscontinueObjectBaseGas
	}

	numIds := uint64(len(args.ObjectIds))
	if numIds > MaxDiscontinueObjectIds {
		numIds = MaxDiscontinueObjectIds
	}

	return DiscontinueObjectBaseGas + (numIds * DiscontinueObjectPerIdGas)
}

func (c *Contract) registerMethod(methodName string, gas uint64, handler precompiledContractFunc, eventName string) {
	method, ok := storageABI.Methods[methodName]
	if !ok {
		panic(fmt.Errorf("method %s is not exist", methodName))
	}
	c.handlers[method.Name] = handler
	c.gasMeters[method.Name] = gas
	if eventName != "" {
		if _, ok := storageABI.Events[eventName]; !ok {
			panic(fmt.Errorf("event %s is not exist", eventName))
		}
		c.events[method.Name] = eventName
	}
}
