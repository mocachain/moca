package distribution

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/mocachain/moca/v2/x/evm/precompiles/types"
)

type Contract struct {
	cmn.Precompile
	distributionKeeper distributionkeeper.Keeper
}

// NewPrecompiledContract returns a static precompile; sdk.Context is sourced per-call via the EVM StateDB.
func NewPrecompiledContract(distributionKeeper distributionkeeper.Keeper, bankKeeper bankkeeper.Keeper) *Contract {
	return &Contract{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      distributionAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		distributionKeeper: distributionKeeper,
	}
}

func (c *Contract) Address() common.Address {
	return distributionAddress
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case SetWithdrawAddressMethodName:
		return SetWithdrawAddressGas
	case WithdrawDelegatorRewardMethodName:
		return WithdrawDelegatorRewardGas
	case WithdrawDelegatorAllRewardsMethodName:
		return WithdrawDelegatorAllRewardsGas
	case WithdrawValidatorCommissionMethodName:
		return WithdrawValidatorCommissionGas
	case FundCommunityPoolMethodName:
		return FundCommunityPoolGas
	case ValidatorDistributionInfoMethodName:
		return ValidatorDistributionInfoGas
	case ValidatorOutstandingRewardsMethodName:
		return ValidatorOutstandingRewardsGas
	case ValidatorCommissionMethodName:
		return ValidatorCommissionGas
	case DelegationRewardsMethodName:
		return DelegationRewardsGas
	case DelegationTotalRewardsMethodName:
		return DelegationTotalRewardsGas
	case CommunityPoolMethodName:
		return CommunityPoolGas
	case ParamsMethodName:
		return ParamsGas
	case ValidatorSlashesMethodName:
		return ValidatorSlashesGas
	case DelegatorValidatorsMethodName:
		return DelegatorValidatorsGas
	case delegatorWithdrawAddressMethodName:
		return delegatorWithdrawAddressGas
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
	case SetWithdrawAddressMethodName:
		return c.SetWithdrawAddress(ctx, evm, contract, readonly)
	case WithdrawDelegatorRewardMethodName:
		return c.WithdrawDelegatorReward(ctx, evm, contract, readonly)
	case WithdrawDelegatorAllRewardsMethodName:
		return c.WithdrawDelegatorAllRewards(ctx, evm, contract, readonly)
	case WithdrawValidatorCommissionMethodName:
		return c.WithdrawValidatorCommission(ctx, evm, contract, readonly)
	case FundCommunityPoolMethodName:
		return c.FundCommunityPool(ctx, evm, contract, readonly)
	case ValidatorDistributionInfoMethodName:
		return c.ValidatorDistributionInfo(ctx, evm, contract, readonly)
	case ValidatorOutstandingRewardsMethodName:
		return c.ValidatorOutstandingRewards(ctx, evm, contract, readonly)
	case ValidatorCommissionMethodName:
		return c.ValidatorCommission(ctx, evm, contract, readonly)
	case DelegationRewardsMethodName:
		return c.DelegationRewards(ctx, evm, contract, readonly)
	case DelegationTotalRewardsMethodName:
		return c.DelegationTotalRewards(ctx, evm, contract, readonly)
	case CommunityPoolMethodName:
		return c.CommunityPool(ctx, evm, contract, readonly)
	case ParamsMethodName:
		return c.Params(ctx, evm, contract, readonly)
	case ValidatorSlashesMethodName:
		return c.ValidatorSlashes(ctx, evm, contract, readonly)
	case DelegatorValidatorsMethodName:
		return c.DelegatorValidators(ctx, evm, contract, readonly)
	case delegatorWithdrawAddressMethodName:
		return c.DelegatorWithdrawAddress(ctx, evm, contract, readonly)
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
