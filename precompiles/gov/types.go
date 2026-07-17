package gov

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	govAddress = common.HexToAddress(types.GovAddress)
	govABI     = types.MustABIJson(IGovMetaData.ABI)
)

// GetAddress returns the gov precompile's fixed hex address.
func GetAddress() common.Address {
	return govAddress
}

// GetEvent resolves an ABI event by name.
func GetEvent(name string) (abi.Event, error) {
	event := govABI.Events[name]
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

// The arg structs below are decode targets for cmn.SetupABI's positional args via
// abi.Arguments.Copy; their fields carry the ABI names (and hex address types).

type LegacySubmitProposalArgs struct {
	Title          string `abi:"title"`
	Description    string `abi:"description"`
	InitialDeposit []Coin `abi:"initialDeposit"`
}

type SubmitProposalArgs struct {
	Messages       string `abi:"messages"`
	InitialDeposit []Coin `abi:"initialDeposit"`
	Metadata       string `abi:"metadata"`
	Title          string `abi:"title"`
	Summary        string `abi:"summary"`
	Expedited      bool   `abi:"expedited"`
}

type VoteArgs struct {
	ProposalID uint64 `abi:"proposalId"`
	Option     uint8  `abi:"option"`
	Metadata   string `abi:"metadata"`
}

type VoteWeightedArgs struct {
	ProposalID uint64               `abi:"proposalId"`
	Options    []WeightedVoteOption `abi:"options"`
	Metadata   string               `abi:"metadata"`
}

type DepositArgs struct {
	ProposalID uint64   `abi:"proposalId"`
	Amount     *big.Int `abi:"amount"`
}

type ProposalArgs struct {
	ProposalID uint64 `abi:"proposalId"`
}

type ProposalsArgs struct {
	Status     uint8          `abi:"status"`
	Voter      common.Address `abi:"voter"`
	Depositor  common.Address `abi:"depositor"`
	Pagination PageRequest    `abi:"pagination"`
}

type VoteQueryArgs struct {
	ProposalID uint64         `abi:"proposalId"`
	Voter      common.Address `abi:"voter"`
}

type VotesArgs struct {
	ProposalID uint64      `abi:"proposalId"`
	Pagination PageRequest `abi:"pagination"`
}

type DepositQueryArgs struct {
	ProposalID uint64         `abi:"proposalId"`
	Depositor  common.Address `abi:"depositor"`
}

type DepositsArgs struct {
	ProposalID uint64      `abi:"proposalId"`
	Pagination PageRequest `abi:"pagination"`
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

func pageResponse(res *query.PageResponse) PageResponse {
	return PageResponse{NextKey: res.NextKey, Total: res.Total}
}
