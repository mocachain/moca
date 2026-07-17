package storageprovider

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	spAddress = common.HexToAddress(types.SpAddress)
	spABI     = types.MustABIJson(IStorageProviderMetaData.ABI)
)

type (
	PageRequestJson = PageRequest
)

// GetAddress returns the storage provider precompile's fixed hex address.
func GetAddress() common.Address {
	return spAddress
}

// GetAbiMethod resolves an ABI method by name.
func GetAbiMethod(name string) abi.Method {
	return spABI.Methods[name]
}

// GetEvent resolves an ABI event by name.
func GetEvent(name string) (abi.Event, error) {
	event := spABI.Events[name]
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

// The arg structs below are decode targets for cmn.SetupABI's positional args via
// abi.Arguments.Copy; their fields carry the ABI names (and hex address types).

type UpdateSPPriceArgs struct {
	ReadPrice     *big.Int `abi:"readPrice"`
	FreeReadQuota uint64   `abi:"freeReadQuota"`
	StorePrice    *big.Int `abi:"storePrice"`
}

type StorageProviderArgs struct {
	Id uint32 `abi:"id"`
}

type StorageProvidersArgs struct {
	Pagination PageRequestJson `abi:"pagination"`
}

type StorageProviderByOperatorAddressArgs struct {
	OperatorAddress common.Address `abi:"operatorAddress"`
}

type StorageProviderPriceArgs struct {
	OperatorAddress common.Address `abi:"operatorAddress"`
}
