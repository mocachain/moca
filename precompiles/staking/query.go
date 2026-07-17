package staking

import (
	"bytes"
	"encoding/base64"

	"cosmossdk.io/math"
	"github.com/0xPolygon/polygon-edge/helper/hex"

	cometbfttypes "github.com/cometbft/cometbft/proto/tendermint/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	DelegationMethodName                    = "delegation"
	UnbondingDelegationMethodName           = "unbondingDelegation"
	ValidatorMethodName                     = "validator"
	ValidatorsMethodName                    = "validators"
	ValidatorDelegationsMethodName          = "validatorDelegations"
	ValidatorUnbondingDelegationsMethodName = "validatorUnbondingDelegations"
	DelegatorDelegationsMethodName          = "delegatorDelegations"
	DelegatorUnbondingDelegationsMethodName = "delegatorUnbondingDelegations"
	RedelegationsMethodName                 = "redelegations"
	DelegatorValidatorsMethodName           = "delegatorValidators"
	DelegatorValidatorMethodName            = "delegatorValidator"
	HistoricalInfoMethodName                = "historicalInfo"
	PoolMethodName                          = "pool"
	ParamsMethodName                        = "params"
)

// Delegation queries the delegation info for a given delegator/validator pair.
func (p Precompile) Delegation(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DelegationArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: input.GetDelegator().String(),
		ValidatorAddr: input.GetValidator().String(),
	}

	res, err := p.stakingQuerier.Delegation(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(OutputsDelegation(*res.DelegationResponse))
}

// UnbondingDelegation queries the unbonding delegation for a delegator/validator pair.
func (p Precompile) UnbondingDelegation(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input UnbondingDelegationArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryUnbondingDelegationRequest{
		DelegatorAddr: input.GetDelegator().String(),
		ValidatorAddr: input.GetValidator().String(),
	}

	res, err := p.stakingQuerier.UnbondingDelegation(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(OutputsUnbondingDelegation(res.Unbond))
}

// Validators queries all validators matching the given status.
func (p Precompile) Validators(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ValidatorsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &stakingtypes.QueryValidatorsRequest{
		Status:     input.GetStatus(),
		Pagination: pageRequest(input.Pagination),
	}

	res, err := p.stakingQuerier.Validators(ctx, msg)
	if err != nil {
		return nil, err
	}

	var validators []Validator
	for _, validator := range res.Validators {
		validators = append(validators, OutputsValidator(validator))
	}

	return method.Outputs.Pack(validators, pageResponse(res.Pagination))
}

// Validator queries a single validator by address.
func (p Precompile) Validator(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ValidatorArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryValidatorRequest{
		ValidatorAddr: input.GetValidator().String(),
	}

	res, err := p.stakingQuerier.Validator(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(OutputsValidator(res.Validator))
}

// ValidatorDelegations queries delegate info for given validator.
func (p Precompile) ValidatorDelegations(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ValidatorDelegationsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryValidatorDelegationsRequest{
		ValidatorAddr: input.GetValidator().String(),
		Pagination:    pageRequest(input.Pagination),
	}

	res, err := p.stakingQuerier.ValidatorDelegations(ctx, msg)
	if err != nil {
		return nil, err
	}

	var delegations []DelegationResponse
	for _, delegation := range res.DelegationResponses {
		delegations = append(delegations, OutputsDelegation(delegation))
	}

	return method.Outputs.Pack(delegations, pageResponse(res.Pagination))
}

// ValidatorUnbondingDelegations queries unbonding delegations of a validator.
func (p Precompile) ValidatorUnbondingDelegations(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ValidatorDelegationsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryValidatorUnbondingDelegationsRequest{
		ValidatorAddr: input.GetValidator().String(),
		Pagination:    pageRequest(input.Pagination),
	}

	res, err := p.stakingQuerier.ValidatorUnbondingDelegations(ctx, msg)
	if err != nil {
		return nil, err
	}

	var unbondingDelegations []UnbondingDelegation
	for _, unbondingDelegation := range res.UnbondingResponses {
		unbondingDelegations = append(unbondingDelegations, OutputsUnbondingDelegation(unbondingDelegation))
	}

	return method.Outputs.Pack(unbondingDelegations, pageResponse(res.Pagination))
}

// DelegatorDelegations queries all delegations of a given delegator address.
func (p Precompile) DelegatorDelegations(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DelegatorDelegationsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryDelegatorDelegationsRequest{
		DelegatorAddr: input.GetDelegator().String(),
		Pagination:    pageRequest(input.Pagination),
	}

	res, err := p.stakingQuerier.DelegatorDelegations(ctx, msg)
	if err != nil {
		return nil, err
	}

	var delegations []DelegationResponse
	for _, delegation := range res.DelegationResponses {
		delegations = append(delegations, OutputsDelegation(delegation))
	}

	return method.Outputs.Pack(delegations, pageResponse(res.Pagination))
}

// DelegatorUnbondingDelegations queries all unbonding delegations of a given
// delegator address.
func (p Precompile) DelegatorUnbondingDelegations(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DelegatorDelegationsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryDelegatorUnbondingDelegationsRequest{
		DelegatorAddr: input.GetDelegator().String(),
		Pagination:    pageRequest(input.Pagination),
	}

	res, err := p.stakingQuerier.DelegatorUnbondingDelegations(ctx, msg)
	if err != nil {
		return nil, err
	}

	var unbondingDelegations []UnbondingDelegation
	for _, unbondingDelegation := range res.UnbondingResponses {
		unbondingDelegations = append(unbondingDelegations, OutputsUnbondingDelegation(unbondingDelegation))
	}

	return method.Outputs.Pack(unbondingDelegations, pageResponse(res.Pagination))
}

// Redelegations queries redelegations of given address.
func (p Precompile) Redelegations(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input Redelegations
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryRedelegationsRequest{
		Pagination: pageRequest(input.Pagination),
	}

	if input.DelegatorAddr == (common.Address{}) {
		msg.DelegatorAddr = ""
	} else {
		msg.DelegatorAddr = input.GetDelegator().String()
	}
	if input.SrcValidatorAddr == (common.Address{}) {
		msg.SrcValidatorAddr = ""
	} else {
		msg.SrcValidatorAddr = input.GetSrcValidator().String()
	}

	if input.DstValidatorAddr == (common.Address{}) {
		msg.DstValidatorAddr = ""
	} else {
		msg.DstValidatorAddr = input.GetDstValidator().String()
	}

	res, err := p.stakingQuerier.Redelegations(ctx, msg)
	if err != nil {
		return nil, err
	}

	var redelegationResponses []RedelegationResponse
	for _, redelegationResponse := range res.RedelegationResponses {
		redelegationResponses = append(redelegationResponses, OutputsRedelegation(redelegationResponse))
	}

	return method.Outputs.Pack(redelegationResponses, pageResponse(res.Pagination))
}

// DelegatorValidators queries all validators info for given delegator address.
func (p Precompile) DelegatorValidators(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DelegatorValidators
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &stakingtypes.QueryDelegatorValidatorsRequest{
		DelegatorAddr: input.GetDelegator().String(),
		Pagination:    pageRequest(input.Pagination),
	}

	res, err := p.stakingQuerier.DelegatorValidators(ctx, msg)
	if err != nil {
		return nil, err
	}

	var validators []Validator
	for _, validator := range res.Validators {
		validators = append(validators, OutputsValidator(validator))
	}

	return method.Outputs.Pack(validators, pageResponse(res.Pagination))
}

// DelegatorValidator queries validator info for given delegator validator pair.
func (p Precompile) DelegatorValidator(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DelegatorValidator
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &stakingtypes.QueryDelegatorValidatorRequest{
		DelegatorAddr: input.GetDelegator().String(),
		ValidatorAddr: input.GetValidator().String(),
	}

	res, err := p.stakingQuerier.DelegatorValidator(ctx, msg)
	if err != nil {
		return nil, err
	}

	validator := OutputsValidator(res.Validator)

	return method.Outputs.Pack(validator)
}

// HistoricalInfo queries the historical info for given height.
func (p Precompile) HistoricalInfo(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HistoricalInfoRequest
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &stakingtypes.QueryHistoricalInfoRequest{Height: input.GetHeight()}

	res, err := p.stakingQuerier.HistoricalInfo(ctx, msg)
	if err != nil {
		return nil, err
	}

	var valsets []Validator
	for _, validator := range res.Hist.Valset {
		valsets = append(valsets, OutputsValidator(validator))
	}
	header := OutputsHeader(res.Hist.Header)
	historicalInfo := HistoricalInfo{
		Header: header,
		Valset: valsets,
	}

	return method.Outputs.Pack(historicalInfo)
}

// Pool queries the pool info.
func (p Precompile) Pool(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.stakingQuerier.Pool(ctx, &stakingtypes.QueryPoolRequest{})
	if err != nil {
		return nil, err
	}

	pool := Pool{
		NotBondedTokens: res.Pool.NotBondedTokens.BigInt(),
		BondedTokens:    res.Pool.BondedTokens.BigInt(),
	}

	return method.Outputs.Pack(pool)
}

// Params queries the staking parameters.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.stakingQuerier.Params(ctx, &stakingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	params := Params{
		UnbondingTime:     int64(res.Params.UnbondingTime),
		MaxValidators:     res.Params.MaxValidators,
		MaxEntries:        res.Params.MaxEntries,
		HistoricalEntries: res.Params.HistoricalEntries,
		BondDenom:         res.Params.BondDenom,
		MinCommissionRate: res.Params.MinCommissionRate.BigInt(),
	}

	return method.Outputs.Pack(params)
}

// pageRequest builds a query.PageRequest from the ABI pagination tuple, treating a
// single zero byte key as empty.
func pageRequest(page PageRequest) *query.PageRequest {
	key := page.Key
	if bytes.Equal(key, []byte{0}) {
		key = nil
	}
	return &query.PageRequest{
		Key:        key,
		Offset:     page.Offset,
		Limit:      page.Limit,
		CountTotal: page.CountTotal,
		Reverse:    page.Reverse,
	}
}

// pageResponse maps a query.PageResponse into the ABI tuple, tolerating a nil response.
func pageResponse(res *query.PageResponse) PageResponse {
	if res == nil {
		return PageResponse{}
	}
	return PageResponse{NextKey: res.NextKey, Total: res.Total}
}

func OutputsValidator(validator stakingtypes.Validator) Validator {
	return Validator{
		OperatorAddress: common.HexToAddress(validator.OperatorAddress),
		ConsensusPubkey: FormatConsensusPubkey(validator.ConsensusPubkey),
		Jailed:          validator.Jailed,
		Status:          uint8(validator.Status),
		Tokens:          validator.Tokens.BigInt(),
		DelegatorShares: validator.DelegatorShares.BigInt(),
		Description:     Description(validator.Description),
		UnbondingHeight: validator.UnbondingHeight,
		UnbondingTime:   validator.UnbondingTime.Unix(),
		Commission: Commission{
			CommissionRates: CommissionRates{
				Rate:          validator.Commission.Rate.BigInt(),
				MaxRate:       validator.Commission.MaxRate.BigInt(),
				MaxChangeRate: validator.Commission.MaxChangeRate.BigInt(),
			},
			UpdateTime: validator.Commission.UpdateTime.Unix(),
		},
		MinSelfDelegation:       validator.MinSelfDelegation.BigInt(),
		UnbondingOnHoldRefCount: validator.UnbondingOnHoldRefCount,
		UnbondingIds:            validator.UnbondingIds,
		SelfDelAddress:          validator.SelfDelAddress,
		RelayerAddress:          validator.RelayerAddress,
		ChallengerAddress:       validator.ChallengerAddress,
		BlsKey:                  hex.EncodeToHex(validator.BlsKey)[2:],
	}
}

func OutputsDelegation(delegationResponse stakingtypes.DelegationResponse) DelegationResponse {
	deletation := delegationResponse.Delegation
	balance := delegationResponse.Balance

	return DelegationResponse{
		Delegation: Delegation{
			DelegatorAddress: common.HexToAddress(deletation.DelegatorAddress),
			ValidatorAddress: common.HexToAddress(deletation.ValidatorAddress),
			Shares: Dec{
				Amount:    deletation.Shares.BigInt(),
				Precision: math.LegacyPrecision,
			},
		},
		Balance: Coin{
			Denom:  balance.Denom,
			Amount: balance.Amount.BigInt(),
		},
	}
}

func OutputsUnbondingDelegation(unbondingDelegation stakingtypes.UnbondingDelegation) UnbondingDelegation {
	var entries []UnbondingDelegationEntry
	for _, entry := range unbondingDelegation.Entries {
		entries = append(entries, UnbondingDelegationEntry{
			CreationHeight: entry.CreationHeight,
			CompletionTime: entry.CompletionTime.Unix(),
			InitialBalance: entry.InitialBalance.BigInt(),
			Balance:        entry.Balance.BigInt(),
		})
	}

	return UnbondingDelegation{
		DelegatorAddress: common.HexToAddress(unbondingDelegation.DelegatorAddress),
		ValidatorAddress: common.HexToAddress(unbondingDelegation.ValidatorAddress),
		Entries:          entries,
	}
}

func OutputsRedelegation(redelegationResponse stakingtypes.RedelegationResponse) RedelegationResponse {
	var entries []RedelegationEntryResponse
	for _, entry := range redelegationResponse.Entries {
		entries = append(entries, RedelegationEntryResponse{
			RedelegationEntry: RedelegationEntry{
				CreationHeight: entry.RedelegationEntry.CreationHeight,
				CompletionTime: entry.RedelegationEntry.CompletionTime.Unix(),
				InitialBalance: entry.RedelegationEntry.InitialBalance.BigInt(),
				ShareDst:       entry.RedelegationEntry.SharesDst.BigInt(),
			},
			Balance: entry.Balance.BigInt(),
		})
	}

	var redelegationEntries []RedelegationEntry
	for _, entry := range redelegationResponse.Redelegation.Entries {
		redelegationEntries = append(redelegationEntries, RedelegationEntry{
			CreationHeight: entry.CreationHeight,
			CompletionTime: entry.CompletionTime.Unix(),
			InitialBalance: entry.InitialBalance.BigInt(),
			ShareDst:       entry.SharesDst.BigInt(),
		})
	}

	redelegation := Redelegation{
		DelegatorAddress:    common.HexToAddress(redelegationResponse.Redelegation.DelegatorAddress),
		ValidatorSrcAddress: common.HexToAddress(redelegationResponse.Redelegation.ValidatorSrcAddress),
		ValidatorDstAddress: common.HexToAddress(redelegationResponse.Redelegation.ValidatorDstAddress),
		Entries:             redelegationEntries,
	}

	return RedelegationResponse{
		Redelegation: redelegation,
		Entries:      entries,
	}
}

func OutputsHeader(header cometbfttypes.Header) Header {
	return Header{
		Version: Consensus{Block: header.Version.Block, App: header.Version.App},
		ChainId: header.ChainID,
		Height:  header.Height,
		Time:    header.Time.Unix(),
		LastBlockId: BlockID{
			Hash: hexutil.Encode(header.LastBlockId.Hash),
			PartSetHeader: PartSetHeader{
				Total: header.LastBlockId.PartSetHeader.Total,
				Hash:  hexutil.Encode(header.LastBlockId.PartSetHeader.Hash),
			},
		},
		LastCommitHash:     hexutil.Encode(header.LastCommitHash),
		DataHash:           hexutil.Encode(header.DataHash),
		ValidatorsHash:     hexutil.Encode(header.ValidatorsHash),
		NextValidatorsHash: hexutil.Encode(header.NextValidatorsHash),
		ConsensusHash:      hexutil.Encode(header.ConsensusHash),
		AppHash:            hexutil.Encode(header.AppHash),
		LastResultsHash:    hexutil.Encode(header.LastResultsHash),
		EvidenceHash:       hexutil.Encode(header.EvidenceHash),
		ProposerAddress:    hexutil.Encode(header.ProposerAddress),
	}
}

// FormatConsensusPubkey format ConsensusPubkey into a base64 string.
func FormatConsensusPubkey(consensusPubkey *codectypes.Any) string {
	ed25519pk, ok := consensusPubkey.GetCachedValue().(cryptotypes.PubKey)
	if ok {
		return base64.StdEncoding.EncodeToString(ed25519pk.Bytes())
	}
	return consensusPubkey.String()
}
