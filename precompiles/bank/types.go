package bank

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	bankAddress = common.HexToAddress(types.BankAddress)
	bankABI     = types.MustABIJson(IBankMetaData.ABI)
)

// GetAddress returns the bank precompile's fixed hex address.
func GetAddress() common.Address {
	return bankAddress
}

// GetMethod resolves an ABI method by name.
func GetMethod(name string) (abi.Method, error) {
	method := bankABI.Methods[name]
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
	event := bankABI.Events[name]
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
	CoinJson        = Coin
	PageRequestJson = PageRequest
)

// The arg structs below are decode targets for cmn.SetupABI's positional args via
// abi.Arguments.Copy; their fields carry the ABI names (and hex address types).

type SendArgs struct {
	ToAddress common.Address `abi:"toAddress"`
	Amount    []CoinJson     `abi:"amount"`
}

type MultiSendArgs struct {
	Outputs []SendArgs `abi:"outputs"`
}

type BalanceArgs struct {
	AccountAddress common.Address `abi:"accountAddress"`
	Denom          string         `abi:"denom"`
}

type AllBalancesArgs struct {
	AccountAddress common.Address  `abi:"accountAddress"`
	PageRequest    PageRequestJson `abi:"pageRequest"`
}

type SpendableBalancesArgs = AllBalancesArgs

type SpendableBalanceByDenomArgs struct {
	AccountAddress common.Address `abi:"accountAddress"`
	Denom          string         `abi:"denom"`
}

type TotalSupplyArgs struct {
	PageRequest PageRequestJson `abi:"pageRequest"`
}

type SupplyOfArgs struct {
	Denom string `abi:"denom"`
}

type DenomMetadataArgs = SupplyOfArgs

type DenomsMetadataArgs struct {
	PageRequest PageRequestJson `abi:"pageRequest"`
}

type DenomOwnersArgs struct {
	Denom       string          `abi:"denom"`
	PageRequest PageRequestJson `abi:"pageRequest"`
}

type SendEnabledArgs struct {
	Denoms      []string        `abi:"denoms"`
	PageRequest PageRequestJson `abi:"pageRequest"`
}
