package staking

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	cmn.Precompile
	stakingKeeper *stakingkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(stakingKeeper *stakingkeeper.Keeper, bankKeeper bankkeeper.Keeper) *Contract {
	return &Contract{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      stakingAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		stakingKeeper: stakingKeeper,
	}
}

func (c *Contract) Address() common.Address {
	return stakingAddress
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case EditValidatorMethodName:
		return EditValidatorGas
	case DelegateMethodName:
		return DelegateGas
	case UndelegateMethodName:
		return UndelegateGas
	case RedelegateMethodName:
		return RedelegateGas
	case CancelUnbondingDelegationMethodName:
		return CancelUnbondingDelegationGas
	case DelegationMethodName:
		return DelegationGas
	case UnbondingDelegationMethodName:
		return UnbondingDelegationGas
	case ValidatorMethodName:
		return ValidatorGas
	case ValidatorsMethodName:
		return ValidatorsGas
	case ValidatorDelegationsMethodName:
		return ValidatorDelegationsGas
	case ValidatorUnbondingDelegationsMethodName:
		return ValidatorUnbondingDelegationsGas
	case DelegatorDelegationsMethodName:
		return DelegatorDelegationsGas
	case DelegatorUnbondingDelegationsMethodName:
		return DelegatorUnbondingDelegationsGas
	case RedelegationsMethodName:
		return RedelegationsGas
	case DelegatorValidatorsMethodName:
		return DelegatorValidatorsGas
	case DelegatorValidatorMethodName:
		return DelegatorValidatorGas
	case HistoricalInfoMethodName:
		return HistoricalInfoGas
	case PoolMethodName:
		return PoolGas
	case ParamsMethodName:
		return ParamsGas
	default:
		return 0
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
	switch method.Name {
	case EditValidatorMethodName:
		return c.EditValidator(ctx, evm, contract, readonly)
	case DelegateMethodName:
		return c.Delegate(ctx, evm, contract, readonly)
	case UndelegateMethodName:
		return c.Undelegate(ctx, evm, contract, readonly)
	case RedelegateMethodName:
		return c.Redelegate(ctx, evm, contract, readonly)
	case CancelUnbondingDelegationMethodName:
		return c.CancelUnbondingDelegation(ctx, evm, contract, readonly)
	case DelegationMethodName:
		return c.Delegation(ctx, evm, contract, readonly)
	case UnbondingDelegationMethodName:
		return c.UnbondingDelegation(ctx, evm, contract, readonly)
	case ValidatorMethodName:
		return c.Validator(ctx, evm, contract, readonly)
	case ValidatorsMethodName:
		return c.Validators(ctx, evm, contract, readonly)
	case ValidatorDelegationsMethodName:
		return c.ValidatorDelegations(ctx, evm, contract, readonly)
	case ValidatorUnbondingDelegationsMethodName:
		return c.ValidatorUnbondingDelegations(ctx, evm, contract, readonly)
	case DelegatorDelegationsMethodName:
		return c.DelegatorDelegations(ctx, evm, contract, readonly)
	case DelegatorUnbondingDelegationsMethodName:
		return c.DelegatorUnbondingDelegations(ctx, evm, contract, readonly)
	case RedelegationsMethodName:
		return c.Redelegations(ctx, evm, contract, readonly)
	case DelegatorValidatorsMethodName:
		return c.DelegatorValidators(ctx, evm, contract, readonly)
	case DelegatorValidatorMethodName:
		return c.DelegatorValidator(ctx, evm, contract, readonly)
	case HistoricalInfoMethodName:
		return c.HistoricalInfo(ctx, evm, contract, readonly)
	case PoolMethodName:
		return c.Pool(ctx, evm, contract, readonly)
	case ParamsMethodName:
		return c.Params(ctx, evm, contract, readonly)
	default:
		return nil, fmt.Errorf("method %s is not handled", method.Name)
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
