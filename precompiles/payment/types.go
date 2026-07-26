package payment

import (
	"fmt"
	"math/big"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	paymentAddress = common.HexToAddress(types.PaymentAddress)
	paymentABI     = types.MustABIJson(IPaymentMetaData.ABI)
)

// GetAddress returns the payment precompile's fixed hex address.
func GetAddress() common.Address {
	return paymentAddress
}

// GetMethod resolves an ABI method by name.
func GetMethod(name string) (abi.Method, error) {
	method := paymentABI.Methods[name]
	if method.ID == nil {
		return abi.Method{}, fmt.Errorf("method %s does not exist", name)
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
	event := paymentABI.Events[name]
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
// abi.Arguments.Copy; their fields carry the ABI names.

// DepositArgs are the inputs to the deposit transaction.
type DepositArgs struct {
	To     string   `abi:"to"`
	Amount *big.Int `abi:"amount"`
}

// DisableRefundArgs are the inputs to the disableRefund transaction.
type DisableRefundArgs struct {
	Addr string `abi:"addr"`
}

// WithdrawArgs are the inputs to the withdraw transaction.
type WithdrawArgs struct {
	From   string   `abi:"from"`
	Amount *big.Int `abi:"amount"`
}

// PaymentAccountsByOwnerArgs are the inputs to the paymentAccountsByOwner query.
type PaymentAccountsByOwnerArgs struct {
	Owner string `abi:"owner"`
}

// PaymentAccountArgs are the inputs to the paymentAccount query.
type PaymentAccountArgs struct {
	Addr string `abi:"addr"`
}

// ParamsByTimestampArgs are the inputs to the paramsByTimestamp query.
type ParamsByTimestampArgs struct {
	Timestamp int64 `abi:"timestamp"`
}

// OutFlowsArgs are the inputs to the outFlows query.
type OutFlowsArgs struct {
	Account string `abi:"account"`
}

// StreamRecordArgs are the inputs to the streamRecord query.
type StreamRecordArgs struct {
	Account string `abi:"account"`
}

// StreamRecordsArgs are the inputs to the streamRecords query.
type StreamRecordsArgs struct {
	Pagination PageRequest `abi:"pagination"`
}

// PaymentAccountCountArgs are the inputs to the paymentAccountCount query.
type PaymentAccountCountArgs struct {
	Owner string `abi:"owner"`
}

// PaymentAccountCountsArgs are the inputs to the paymentAccountCounts query.
type PaymentAccountCountsArgs struct {
	Pagination PageRequest `abi:"pagination"`
}

// PaymentAccountsArgs are the inputs to the paymentAccounts query.
type PaymentAccountsArgs struct {
	Pagination PageRequest `abi:"pagination"`
}

// DynamicBalanceArgs are the inputs to the dynamicBalance query.
type DynamicBalanceArgs struct {
	Account string `abi:"account"`
}

// AutoSettleRecordsArgs are the inputs to the autoSettleRecords query.
type AutoSettleRecordsArgs struct {
	Pagination PageRequest `abi:"pagination"`
}

// DelayedWithdrawalArgs are the inputs to the delayedWithdrawal query.
type DelayedWithdrawalArgs struct {
	Account string `abi:"account"`
}

// pageRequest builds a query.PageRequest from the ABI pagination tuple.
func pageRequest(page PageRequest) *query.PageRequest {
	return &query.PageRequest{
		Key:        page.Key,
		Offset:     page.Offset,
		Limit:      page.Limit,
		CountTotal: page.CountTotal,
		Reverse:    page.Reverse,
	}
}

// pageResponse maps a query.PageResponse into the ABI tuple.
func pageResponse(res *query.PageResponse) *PageResponse {
	return &PageResponse{
		NextKey: res.NextKey,
		Total:   res.Total,
	}
}
