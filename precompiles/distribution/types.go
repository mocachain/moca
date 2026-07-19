package distribution

import (
	"bytes"
	"errors"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	distributionAddress = common.HexToAddress(types.DistributionAddress)
	distributionABI     = types.MustABIJson(IDistributionMetaData.ABI)
)

// GetAddress returns the distribution precompile's fixed hex address.
func GetAddress() common.Address {
	return distributionAddress
}

// GetEvent resolves an ABI event by name.
func GetEvent(name string) (abi.Event, error) {
	event := distributionABI.Events[name]
	if event.ID == (common.Hash{}) {
		return abi.Event{}, fmt.Errorf("event %s does not exist", name)
	}
	return event, nil
}

// MustEvent resolves an ABI event by name and panics if it does not exist.
func MustEvent(name string) abi.Event {
	event, err := GetEvent(name)
	if err != nil {
		panic(err)
	}
	return event
}

// ValidatorSlashesInput is the decode target for the validatorSlashes query args.
// The field names match the ABI argument names (camelCased) so abi.Arguments.Copy
// can populate them, including the nested pagination tuple.
type ValidatorSlashesInput struct {
	ValidatorAddress common.Address
	StartingHeight   uint64
	EndingHeight     uint64
	Pagination       PageRequest
}

// hexAddressArg asserts a single hex-address argument (validator or delegator).
func hexAddressArg(args []interface{}, name string) (common.Address, error) {
	if len(args) != 1 {
		return common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}
	addr, ok := args[0].(common.Address)
	if !ok || addr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("invalid %s: %v", name, args[0])
	}
	return addr, nil
}

// NewMsgSetWithdrawAddress builds a MsgSetWithdrawAddress. The delegator is the
// caller; the withdraw address is the sole hex argument.
func NewMsgSetWithdrawAddress(args []interface{}, caller common.Address) (*distributiontypes.MsgSetWithdrawAddress, common.Address, error) {
	withdrawAddr, err := hexAddressArg(args, "withdraw address")
	if err != nil {
		return nil, common.Address{}, err
	}
	msg := &distributiontypes.MsgSetWithdrawAddress{
		DelegatorAddress: sdk.AccAddress(caller.Bytes()).String(),
		WithdrawAddress:  sdk.AccAddress(withdrawAddr.Bytes()).String(),
	}
	return msg, withdrawAddr, nil
}

// NewMsgWithdrawDelegatorReward builds a MsgWithdrawDelegatorReward for the caller
// against the validator hex argument.
func NewMsgWithdrawDelegatorReward(args []interface{}, caller common.Address) (*distributiontypes.MsgWithdrawDelegatorReward, common.Address, error) {
	validatorAddr, err := hexAddressArg(args, "validator address")
	if err != nil {
		return nil, common.Address{}, err
	}
	msg := &distributiontypes.MsgWithdrawDelegatorReward{
		DelegatorAddress: sdk.AccAddress(caller.Bytes()).String(),
		ValidatorAddress: validatorAddr.String(),
	}
	return msg, validatorAddr, nil
}

// NewMsgWithdrawValidatorCommission builds a MsgWithdrawValidatorCommission for the
// caller acting as its own validator.
func NewMsgWithdrawValidatorCommission(caller common.Address) *distributiontypes.MsgWithdrawValidatorCommission {
	return &distributiontypes.MsgWithdrawValidatorCommission{
		ValidatorAddress: sdk.ValAddress(caller.Bytes()).String(),
	}
}

// NewMsgFundCommunityPool builds a MsgFundCommunityPool from the caller (depositor)
// and the coins argument.
func NewMsgFundCommunityPool(args []interface{}, caller common.Address) (*distributiontypes.MsgFundCommunityPool, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}
	coins, err := cmn.ToCoins(args[0])
	if err != nil {
		return nil, fmt.Errorf(ErrInvalidAmount, err)
	}
	// Preserve the pre-conversion FundCommunityPoolArgs.Validate checks: the keeper and
	// sdk.Coins.Validate accept an empty (and zero-amount) coin set, so reject them here.
	if len(coins) == 0 {
		return nil, errors.New("no coin send")
	}
	for _, coin := range coins {
		if coin.Amount.Sign() <= 0 {
			return nil, fmt.Errorf("coin amount is %s less than or equal 0", coin.Amount.String())
		}
	}
	amount, err := cmn.NewSdkCoinsFromCoins(coins)
	if err != nil {
		return nil, fmt.Errorf(ErrInvalidAmount, err)
	}
	return &distributiontypes.MsgFundCommunityPool{
		Depositor: sdk.AccAddress(caller.Bytes()).String(),
		Amount:    amount,
	}, nil
}

// NewValidatorSlashesRequest decodes the validatorSlashes query args, translating the
// ABI pagination tuple into a query.PageRequest.
func NewValidatorSlashesRequest(method *abi.Method, args []interface{}) (*distributiontypes.QueryValidatorSlashesRequest, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	var input ValidatorSlashesInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to ValidatorSlashesInput struct: %s", err)
	}
	if input.StartingHeight > input.EndingHeight {
		return nil, fmt.Errorf("startingHeight %d is greater than endingHeight %d", input.StartingHeight, input.EndingHeight)
	}
	// An empty pagination key can arrive ABI-encoded as a single zero byte.
	key := input.Pagination.Key
	if bytes.Equal(key, []byte{0}) {
		key = nil
	}

	return &distributiontypes.QueryValidatorSlashesRequest{
		ValidatorAddress: input.ValidatorAddress.String(),
		StartingHeight:   input.StartingHeight,
		EndingHeight:     input.EndingHeight,
		Pagination: &query.PageRequest{
			Key:        key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	}, nil
}

// newDecCoins converts sdk.DecCoins into the ABI DecCoin tuple, preserving moca's
// full-precision Dec representation (amount is the 10^18-scaled integer with an
// explicit precision) rather than truncating to an integer.
func newDecCoins(decCoins sdk.DecCoins) []DecCoin {
	coins := make([]DecCoin, 0, len(decCoins))
	for _, c := range decCoins {
		coins = append(coins, DecCoin{
			Denom:     c.Denom,
			Amount:    c.Amount.BigInt(),
			Precision: uint8(math.LegacyPrecision),
		})
	}
	return coins
}
