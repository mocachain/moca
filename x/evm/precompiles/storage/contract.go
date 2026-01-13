package storage

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	storagekeeper "github.com/evmos/evmos/v12/x/storage/keeper"

	"github.com/evmos/evmos/v12/x/evm/types"
)

type (
	precompiledContractFunc func(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error)
	Contract                struct {
		ctx           sdk.Context
		storageKeeper storagekeeper.Keeper
		handlers      map[string]precompiledContractFunc
		gasMeters     map[string]uint64
		events        map[string]string
	}
)

func NewPrecompiledContract(ctx sdk.Context, storageKeeper storagekeeper.Keeper) *Contract {
	c := &Contract{
		ctx:           ctx,
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

	// Special handling for dynamic gas methods
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

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) (ret []byte, err error) {
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}
	ctx, commit := c.ctx.CacheContext()
	snapshot := evm.StateDB.Snapshot()
	defer func() {
		if err != nil {
			evm.StateDB.RevertToSnapshot(snapshot)
		}
	}()
	method, err := GetMethodByID(contract.Input)
	if err != nil {
		return types.PackRetError(err.Error())
	}
	handler, ok := c.handlers[method.Name]
	if !ok {
		return types.PackRetError("method not handled")
	}
	ret, err = handler(ctx, evm, contract, readonly)
	if err != nil {
		return nil, err
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

// calculatePutPolicyGas calculates gas cost based on statements and their nested fields
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

	// Calculate dynamic gas: base + per_statement * num_statements + per_action * total_actions + per_resource * total_resources
	numStatements := uint64(len(args.Statements))
	if numStatements > MaxPolicyStatements {
		numStatements = MaxPolicyStatements
	}

	// Count total actions and resources across all statements
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

// calculateRenewGroupMemberGas calculates gas cost based on number of members
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

	// Calculate dynamic gas: base + per_member * num_members
	numMembers := uint64(len(args.Members))
	if numMembers > MaxRenewGroupMembers {
		numMembers = MaxRenewGroupMembers
	}

	return RenewGroupMemberBaseGas + (numMembers * RenewGroupMemberPerMemberGas)
}

// calculateUpdateGroupGas calculates gas cost based on total members to add/delete
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

	// Calculate dynamic gas: base + per_member * (num_add + num_delete)
	totalMembers := uint64(len(args.MembersToAdd) + len(args.MembersToDelete))
	if totalMembers > MaxUpdateGroupMembers {
		totalMembers = MaxUpdateGroupMembers
	}

	return UpdateGroupBaseGas + (totalMembers * UpdateGroupPerMemberGas)
}

// calculateDiscontinueObjectGas calculates gas cost based on number of object IDs
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

	// Calculate dynamic gas: base + per_id * num_ids
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
