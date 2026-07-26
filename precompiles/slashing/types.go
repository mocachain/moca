package slashing

import (
	"bytes"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	slashingAddress = common.HexToAddress(types.SlashingAddress)
	slashingABI     = types.MustABIJson(ISlashingMetaData.ABI)
)

// GetAddress returns the slashing precompile's fixed hex address.
func GetAddress() common.Address {
	return slashingAddress
}

// GetEvent resolves an ABI event by name.
func GetEvent(name string) (abi.Event, error) {
	event := slashingABI.Events[name]
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

// SigningInfosInput is the decode target for the signingInfos query args, letting
// abi.Arguments.Copy translate the ABI pagination tuple.
type SigningInfosInput struct {
	Pagination PageRequest
}

// NewSigningInfoRequest builds a QuerySigningInfoRequest from the hex consensus
// address argument.
func NewSigningInfoRequest(args []interface{}) (*slashingtypes.QuerySigningInfoRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}
	consAddr, ok := args[0].(common.Address)
	if !ok || consAddr == (common.Address{}) {
		return nil, fmt.Errorf("invalid consensus address: %v", args[0])
	}
	return &slashingtypes.QuerySigningInfoRequest{
		ConsAddress: sdk.ConsAddress(consAddr.Bytes()).String(),
	}, nil
}

// NewSigningInfosRequest decodes the signingInfos query args, translating the ABI
// pagination tuple into a query.PageRequest.
func NewSigningInfosRequest(method *abi.Method, args []interface{}) (*slashingtypes.QuerySigningInfosRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}
	var input SigningInfosInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to SigningInfosInput struct: %s", err)
	}
	// An empty pagination key can arrive ABI-encoded as a single zero byte.
	key := input.Pagination.Key
	if bytes.Equal(key, []byte{0}) {
		key = nil
	}
	return &slashingtypes.QuerySigningInfosRequest{
		Pagination: &query.PageRequest{
			Key:        key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	}, nil
}

// newValidatorSigningInfo maps a slashing ValidatorSigningInfo into the ABI tuple,
// rendering the consensus address as a hex address.
func newValidatorSigningInfo(info slashingtypes.ValidatorSigningInfo) ValidatorSigningInfo {
	return ValidatorSigningInfo{
		ConsAddress:         common.HexToAddress(info.Address),
		StartHeight:         info.StartHeight,
		IndexOffset:         info.IndexOffset,
		JailedUntil:         info.JailedUntil.Unix(),
		Tombstoned:          info.Tombstoned,
		MissedBlocksCounter: info.MissedBlocksCounter,
	}
}
