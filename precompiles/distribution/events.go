package distribution

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	// EventTypeSetWithdrawAddress is the event emitted on a SetWithdrawAddress transaction.
	EventTypeSetWithdrawAddress = "SetWithdrawAddress"
	// EventTypeWithdrawDelegatorReward is the event emitted on a WithdrawDelegatorReward transaction.
	EventTypeWithdrawDelegatorReward = "WithdrawDelegatorReward"
	// EventTypeWithdrawDelegatorAllRewards is the event emitted on a WithdrawDelegatorAllRewards transaction.
	EventTypeWithdrawDelegatorAllRewards = "WithdrawDelegatorAllRewards"
	// EventTypeWithdrawValidatorCommission is the event emitted on a WithdrawValidatorCommission transaction.
	EventTypeWithdrawValidatorCommission = "WithdrawValidatorCommission"
	// EventTypeFundCommunityPool is the event emitted on a FundCommunityPool transaction.
	EventTypeFundCommunityPool = "FundCommunityPool"
)

// EmitSetWithdrawAddressEvent emits the SetWithdrawAddress event with the caller and
// new withdraw address as indexed topics.
func (p Precompile) EmitSetWithdrawAddressEvent(evm *vm.EVM, caller, withdrawAddr common.Address) error {
	return p.AddLog(evm, MustEvent(EventTypeSetWithdrawAddress), []common.Hash{
		common.BytesToHash(caller.Bytes()),
		common.BytesToHash(withdrawAddr.Bytes()),
	})
}

// EmitWithdrawDelegatorRewardEvent emits the WithdrawDelegatorReward event with the
// caller and withdraw address as indexed topics and the amount as data.
func (p Precompile) EmitWithdrawDelegatorRewardEvent(evm *vm.EVM, caller, withdrawAddr common.Address, amount string) error {
	return p.AddLog(evm, MustEvent(EventTypeWithdrawDelegatorReward), []common.Hash{
		common.BytesToHash(caller.Bytes()),
		common.BytesToHash(withdrawAddr.Bytes()),
	}, amount)
}

// EmitWithdrawDelegatorAllRewardsEvent emits the WithdrawDelegatorAllRewards event with
// the caller as an indexed topic and the total amount as data.
func (p Precompile) EmitWithdrawDelegatorAllRewardsEvent(evm *vm.EVM, caller common.Address, amount string) error {
	return p.AddLog(evm, MustEvent(EventTypeWithdrawDelegatorAllRewards), []common.Hash{
		common.BytesToHash(caller.Bytes()),
	}, amount)
}

// EmitWithdrawValidatorCommissionEvent emits the WithdrawValidatorCommission event with
// the caller as an indexed topic and the amount as data.
func (p Precompile) EmitWithdrawValidatorCommissionEvent(evm *vm.EVM, caller common.Address, amount string) error {
	return p.AddLog(evm, MustEvent(EventTypeWithdrawValidatorCommission), []common.Hash{
		common.BytesToHash(caller.Bytes()),
	}, amount)
}

// EmitFundCommunityPoolEvent emits the FundCommunityPool event with the caller as an
// indexed topic and the amount as data.
func (p Precompile) EmitFundCommunityPoolEvent(evm *vm.EVM, caller common.Address, amount string) error {
	return p.AddLog(evm, MustEvent(EventTypeFundCommunityPool), []common.Hash{
		common.BytesToHash(caller.Bytes()),
	}, amount)
}

// AddLog packs the given event and appends it to the StateDB logs at the precompile address.
func (p Precompile) AddLog(evm *vm.EVM, event abi.Event, topics []common.Hash, args ...interface{}) error {
	data, packedTopics, err := types.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      packedTopics,
		Data:        data,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	})
	return nil
}
