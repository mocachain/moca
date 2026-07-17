package staking

import (
	"fmt"
	"math/big"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	stakingAddress = common.HexToAddress(types.StakingAddress)
	stakingABI     = types.MustABIJson(IStakingMetaData.ABI)
)

// GetAddress returns the staking precompile's fixed hex address.
func GetAddress() common.Address {
	return stakingAddress
}

// GetMethod resolves an ABI method by name.
func GetMethod(name string) (abi.Method, error) {
	method := stakingABI.Methods[name]
	if method.ID == nil {
		return abi.Method{}, fmt.Errorf("method %s is not exist", name)
	}
	return method, nil
}

// MustMethod resolves an ABI method by name and panics if it does not exist.
func MustMethod(name string) abi.Method {
	method, err := GetMethod(name)
	if err != nil {
		panic(err)
	}
	return method
}

// GetEvent resolves an ABI event by name.
func GetEvent(name string) (abi.Event, error) {
	event := stakingABI.Events[name]
	if event.ID == (common.Hash{}) {
		return abi.Event{}, fmt.Errorf("event %s is not exist", name)
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

type (
	DescriptionJson     = Description
	CommissionRatesJson = CommissionRates
	PageRequestJson     = PageRequest
)

// The arg structs below are decode targets for cmn.SetupABI's positional args via
// abi.Arguments.Copy; their fields carry the ABI names (and hex address types). The
// keeper validates message contents, so these carry only moca PoA/BLS logic helpers.

type EditValidatorArgs struct {
	Description       DescriptionJson `abi:"description"`
	CommissionRate    *big.Int        `abi:"commissionRate"`
	MinSelfDelegation *big.Int        `abi:"minSelfDelegation"`
	RelayerAddress    common.Address  `abi:"relayerAddress"`
	ChallengerAddress common.Address  `abi:"challengerAddress"`
	BlsKey            string          `abi:"blsKey"`
	BlsProof          string          `abi:"blsProof"`
}

// GetCommissionRate returns the dec commission rate
func (args *EditValidatorArgs) GetCommissionRate() *math.LegacyDec {
	var commissionRate *math.LegacyDec
	// if is less than 0, represents the user's unwillingness to modify this value
	if args.CommissionRate.Cmp(big.NewInt(-1)) > 0 {
		tmp := math.LegacyNewDecFromBigIntWithPrec(args.CommissionRate, math.LegacyPrecision)
		commissionRate = &tmp
	}

	return commissionRate
}

// GetMinSelfDelegation returns the sdk.Int minSelfDelegation
func (args *EditValidatorArgs) GetMinSelfDelegation() *math.Int {
	var minSelfDelegation *math.Int
	// if is less than 0, represents the user's unwillingness to modify this value
	if args.MinSelfDelegation.Cmp(big.NewInt(-1)) > 0 {
		tmp := math.NewIntFromBigInt(args.MinSelfDelegation)
		minSelfDelegation = &tmp
	}

	return minSelfDelegation
}

// GetRelayerAddress returns the relayer address
func (args *EditValidatorArgs) GetRelayerAddress() string {
	if args.RelayerAddress == (common.Address{}) {
		return ""
	}

	return args.RelayerAddress.String()
}

// GetChallengerAddress returns the challenger address
func (args *EditValidatorArgs) GetChallengerAddress() string {
	if args.ChallengerAddress == (common.Address{}) {
		return ""
	}

	return args.ChallengerAddress.String()
}

type DelegateArgs struct {
	ValidatorAddress common.Address `abi:"validatorAddress"`
	Amount           *big.Int       `abi:"amount"`
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *DelegateArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddress.Bytes())
	return valAddr
}

type DelegationArgs struct {
	DelegatorAddr common.Address `abi:"delegatorAddr"`
	ValidatorAddr common.Address `abi:"validatorAddr"`
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *DelegationArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddr.Bytes())
	return valAddr
}

// GetDelegator returns the Delegator address, caller must ensure the delegator address is valid
func (args *DelegationArgs) GetDelegator() sdk.AccAddress {
	accAddr := sdk.AccAddress(args.DelegatorAddr.Bytes())
	return accAddr
}

type UnbondingDelegationArgs struct {
	DelegatorAddr common.Address `abi:"delegatorAddr"`
	ValidatorAddr common.Address `abi:"validatorAddr"`
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *UnbondingDelegationArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddr.Bytes())
	return valAddr
}

// GetDelegator returns the Delegator address, caller must ensure the delegator address is valid
func (args *UnbondingDelegationArgs) GetDelegator() sdk.AccAddress {
	accAddr := sdk.AccAddress(args.DelegatorAddr.Bytes())
	return accAddr
}

type UndelegateArgs struct {
	ValidatorAddress common.Address `abi:"validatorAddress"`
	Amount           *big.Int       `abi:"amount"`
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *UndelegateArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddress.Bytes())
	return valAddr
}

type RedelegateArgs struct {
	ValidatorSrcAddress common.Address `abi:"validatorSrcAddress"`
	ValidatorDstAddress common.Address `abi:"validatorDstAddress"`
	Amount              *big.Int       `abi:"amount"`
}

// GetSrcValidator returns the validator src address, caller must ensure the validator address is valid
func (args *RedelegateArgs) GetSrcValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorSrcAddress.Bytes())
	return valAddr
}

// GetDstValidator returns the validator dest address, caller must ensure the validator address is valid
func (args *RedelegateArgs) GetDstValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorDstAddress.Bytes())
	return valAddr
}

type CancelUnbondingDelegationArgs struct {
	ValidatorAddress common.Address `abi:"validatorAddress"`
	Amount           *big.Int       `abi:"amount"`
	CreationHeight   *big.Int       `abi:"creationHeight"`
}

// GetValidator returns the validator address
func (args *CancelUnbondingDelegationArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddress.Bytes())
	return valAddr
}

// GetCreationHeight returns the creation height
func (args *CancelUnbondingDelegationArgs) GetCreationHeight() int64 {
	return args.CreationHeight.Int64()
}

type ValidatorsArgs struct {
	Status     uint8           `abi:"status"`
	Pagination PageRequestJson `abi:"pagination"`
}

// GetStatus returns the validator status string
func (args *ValidatorsArgs) GetStatus() string {
	switch args.Status {
	case 0:
		return ""
	case 1:
		return stakingtypes.Unbonded.String()
	case 2:
		return stakingtypes.Unbonding.String()
	case 3:
		return stakingtypes.Bonded.String()
	default:
		return ""
	}
}

type ValidatorArgs struct {
	ValidatorAddr common.Address `abi:"validatorAddr"`
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *ValidatorArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddr.Bytes())
	return valAddr
}

type ValidatorDelegationsArgs struct {
	ValidatorAddr common.Address  `abi:"validatorAddr"`
	Pagination    PageRequestJson `abi:"pagination"`
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *ValidatorDelegationsArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddr.Bytes())
	return valAddr
}

type ValidatorUnbondingDelegationsArgs struct {
	ValidatorAddr common.Address  `abi:"validatorAddr"`
	Pagination    PageRequestJson `abi:"pagination"`
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *ValidatorUnbondingDelegationsArgs) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddr.Bytes())
	return valAddr
}

type DelegatorDelegationsArgs struct {
	DelegatorAddr common.Address  `abi:"delegatorAddr"`
	Pagination    PageRequestJson `abi:"pagination"`
}

// GetDelegator returns the delegator address, caller must ensure the delegator address is valid
func (args *DelegatorDelegationsArgs) GetDelegator() sdk.AccAddress {
	valAddr := sdk.AccAddress(args.DelegatorAddr.Bytes())
	return valAddr
}

type DelegatorUnbondingDelegationsArgs struct {
	DelegatorAddr common.Address  `abi:"delegatorAddr"`
	Pagination    PageRequestJson `abi:"pagination"`
}

// GetDelegator returns the delegator address, caller must ensure the delegator address is valid
func (args *DelegatorUnbondingDelegationsArgs) GetDelegator() sdk.AccAddress {
	valAddr := sdk.AccAddress(args.DelegatorAddr.Bytes())
	return valAddr
}

type Redelegations struct {
	DelegatorAddr    common.Address  `abi:"delegatorAddr"`
	SrcValidatorAddr common.Address  `abi:"srcValidatorAddr"`
	DstValidatorAddr common.Address  `abi:"dstValidatorAddr"`
	Pagination       PageRequestJson `abi:"pagination"`
}

// GetDelegator returns the delegator address, caller must ensure the delegator address is valid
func (args *Redelegations) GetDelegator() sdk.AccAddress {
	delAddr := sdk.AccAddress(args.DelegatorAddr.Bytes())
	return delAddr
}

// GetSrcValidator returns the src validator address, caller must ensure the validator address is valid
func (args *Redelegations) GetSrcValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.SrcValidatorAddr.Bytes())
	return valAddr
}

// GetDstValidator returns the dst validator address, caller must ensure the validator address is valid
func (args *Redelegations) GetDstValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.DstValidatorAddr.Bytes())
	return valAddr
}

type DelegatorValidators struct {
	DelegatorAddr common.Address  `abi:"delegatorAddr"`
	Pagination    PageRequestJson `abi:"pagination"`
}

// GetDelegator returns the delegator address, caller must ensure the delegator address is valid
func (args *DelegatorValidators) GetDelegator() sdk.AccAddress {
	delAddr := sdk.AccAddress(args.DelegatorAddr.Bytes())
	return delAddr
}

type DelegatorValidator struct {
	DelegatorAddr common.Address `abi:"delegatorAddr"`
	ValidatorAddr common.Address `abi:"validatorAddr"`
}

// GetDelegator returns the delegator address, caller must ensure the delegator address is valid
func (args *DelegatorValidator) GetDelegator() sdk.AccAddress {
	delAddr := sdk.AccAddress(args.DelegatorAddr.Bytes())
	return delAddr
}

// GetValidator returns the validator address, caller must ensure the validator address is valid
func (args *DelegatorValidator) GetValidator() sdk.ValAddress {
	valAddr := sdk.ValAddress(args.ValidatorAddr.Bytes())
	return valAddr
}

type HistoricalInfoRequest struct {
	Height int64 `abi:"height"`
}

// GetHeight returns the block height, caller must ensure the block height is valid
func (args *HistoricalInfoRequest) GetHeight() int64 {
	return args.Height
}
