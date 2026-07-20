package distribution

import (
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/mocachain/moca/v2/precompiles/types"
)

const (
	SetWithdrawAddressGas          = 60_000
	WithdrawDelegatorRewardGas     = 60_000
	WithdrawDelegatorAllRewardsGas = 100_000
	WithdrawValidatorCommissionGas = 60_000
	FundCommunityPoolGas           = 60_000

	SetWithdrawAddressMethodName          = "setWithdrawAddress"
	WithdrawDelegatorRewardMethodName     = "withdrawDelegatorReward"
	WithdrawDelegatorAllRewardsMethodName = "withdrawDelegatorAllRewards"
	WithdrawValidatorCommissionMethodName = "withdrawValidatorCommission"
	FundCommunityPoolMethodName           = "fundCommunityPool"

	SetWithdrawAddressEventName          = "SetWithdrawAddress"
	WithdrawDelegatorRewardEventName     = "WithdrawDelegatorReward"
	WithdrawDelegatorAllRewardsEventName = "WithdrawDelegatorAllRewards"
	WithdrawValidatorCommissionEventName = "WithdrawValidatorCommission"
	FundCommunityPoolEventName           = "FundCommunityPool"
)

func (c *Contract) SetWithdrawAddress(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	caller := contract.Caller()
	if readonly {
		return nil, types.ErrReadOnly
	}

	method := MustMethod(SetWithdrawAddressMethodName)

	var args SetWithdrawAddressArgs
	err := types.ParseMethodArgs(method, &args, contract.Input[4:])
	if err != nil {
		return nil, err
	}

	msg := &distributiontypes.MsgSetWithdrawAddress{
		DelegatorAddress: sdk.AccAddress(caller.Bytes()).String(),
		WithdrawAddress:  sdk.AccAddress(args.WithdrawAddress.Bytes()).String(),
	}

	server := distributionkeeper.NewMsgServerImpl(c.distributionKeeper)
	_, err = server.SetWithdrawAddress(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := c.AddLog(
		evm,
		MustEvent(SetWithdrawAddressEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes()), common.BytesToHash(args.WithdrawAddress.Bytes())},
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (c *Contract) WithdrawDelegatorReward(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	caller := contract.Caller()
	if readonly {
		return nil, types.ErrReadOnly
	}

	method := MustMethod(WithdrawDelegatorRewardMethodName)

	var args ValidatorAddressArgs
	err := types.ParseMethodArgs(method, &args, contract.Input[4:])
	if err != nil {
		return nil, err
	}

	msg := &distributiontypes.MsgWithdrawDelegatorReward{
		DelegatorAddress: sdk.AccAddress(caller.Bytes()).String(),
		ValidatorAddress: args.ValidatorAddress.String(),
	}

	server := distributionkeeper.NewMsgServerImpl(c.distributionKeeper)
	res, err := server.WithdrawDelegatorReward(ctx, msg)
	if err != nil {
		return nil, err
	}

	// topic[1] must be withdrawAddress, not validatorAddress
	querier := distributionkeeper.Querier{Keeper: c.distributionKeeper}
	withdrawRes, err := querier.DelegatorWithdrawAddress(ctx, &distributiontypes.QueryDelegatorWithdrawAddressRequest{
		DelegatorAddress: sdk.AccAddress(caller.Bytes()).String(),
	})
	if err != nil {
		return nil, err
	}
	withdrawAddr := common.HexToAddress(withdrawRes.WithdrawAddress)
	if err := c.AddLog(
		evm,
		MustEvent(WithdrawDelegatorRewardEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes()), common.BytesToHash(withdrawAddr.Bytes())},
		res.Amount.String(),
	); err != nil {
		return nil, err
	}

	var rewards []Coin
	for _, amount := range res.Amount {
		rewards = append(rewards, Coin{
			Denom:  amount.Denom,
			Amount: amount.Amount.BigInt(),
		})
	}

	return method.Outputs.Pack(rewards)
}

func (c *Contract) WithdrawDelegatorAllRewards(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	caller := contract.Caller()
	if readonly {
		return nil, types.ErrReadOnly
	}

	method := MustMethod(WithdrawDelegatorAllRewardsMethodName)
	delegator := caller.String()
	msgQuery := &distributiontypes.QueryDelegatorValidatorsRequest{
		DelegatorAddress: delegator,
	}

	querier := distributionkeeper.Querier{Keeper: c.distributionKeeper}
	resQuery, err := querier.DelegatorValidators(ctx, msgQuery)
	if err != nil {
		return nil, err
	}

	withdrawRes, err := querier.DelegatorWithdrawAddress(ctx, &distributiontypes.QueryDelegatorWithdrawAddressRequest{
		DelegatorAddress: delegator,
	})
	if err != nil {
		return nil, err
	}
	withdrawAddr := common.HexToAddress(withdrawRes.WithdrawAddress)

	var total sdk.Coins
	server := distributionkeeper.NewMsgServerImpl(c.distributionKeeper)
	for _, validator := range resQuery.Validators {
		msg := &distributiontypes.MsgWithdrawDelegatorReward{
			DelegatorAddress: delegator,
			ValidatorAddress: validator,
		}

		res, err := server.WithdrawDelegatorReward(ctx, msg)
		if err != nil {
			return nil, err
		}
		if err := c.AddLog(
			evm,
			MustEvent(WithdrawDelegatorRewardEventName),
			[]common.Hash{common.BytesToHash(caller.Bytes()), common.BytesToHash(withdrawAddr.Bytes())},
			res.Amount.String(),
		); err != nil {
			return nil, err
		}
		total = total.Add(res.Amount...)
	}

	if err := c.AddLog(
		evm,
		MustEvent(WithdrawDelegatorAllRewardsEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())},
		total.String(),
	); err != nil {
		return nil, err
	}

	var rewards []Coin
	for _, amount := range total {
		rewards = append(rewards, Coin{
			Denom:  amount.Denom,
			Amount: amount.Amount.BigInt(),
		})
	}

	return method.Outputs.Pack(rewards)
}

func (c *Contract) WithdrawValidatorCommission(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	caller := contract.Caller()
	if readonly {
		return nil, types.ErrReadOnly
	}

	method := MustMethod(WithdrawValidatorCommissionMethodName)

	msg := &distributiontypes.MsgWithdrawValidatorCommission{
		ValidatorAddress: sdk.ValAddress(caller.Bytes()).String(),
	}

	server := distributionkeeper.NewMsgServerImpl(c.distributionKeeper)
	res, err := server.WithdrawValidatorCommission(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := c.AddLog(
		evm,
		MustEvent(WithdrawValidatorCommissionEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())},
		res.Amount.String(),
	); err != nil {
		return nil, err
	}

	var rewards []Coin
	for _, amount := range res.Amount {
		rewards = append(rewards, Coin{
			Denom:  amount.Denom,
			Amount: amount.Amount.BigInt(),
		})
	}

	return method.Outputs.Pack(rewards)
}

func (c *Contract) FundCommunityPool(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	caller := contract.Caller()
	if readonly {
		return nil, types.ErrReadOnly
	}

	method := MustMethod(FundCommunityPoolMethodName)

	var args FundCommunityPoolArgs
	err := types.ParseMethodArgs(method, &args, contract.Input[4:])
	if err != nil {
		return nil, err
	}

	var amount []sdk.Coin
	for _, coin := range args.Amount {
		amount = append(amount, sdk.Coin{
			Denom:  coin.Denom,
			Amount: math.NewIntFromBigInt(coin.Amount),
		})
	}
	msg := &distributiontypes.MsgFundCommunityPool{
		Depositor: sdk.AccAddress(caller.Bytes()).String(),
		Amount:    amount,
	}

	server := distributionkeeper.NewMsgServerImpl(c.distributionKeeper)
	_, err = server.FundCommunityPool(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := c.AddLog(
		evm,
		MustEvent(FundCommunityPoolEventName),
		[]common.Hash{common.BytesToHash(caller.Bytes())},
		msg.Amount.String(),
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
