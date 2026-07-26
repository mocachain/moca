package virtualgroup

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	virtualGroupAddress = common.HexToAddress(types.VirtualGroupAddress)
	virtualGroupABI     = types.MustABIJson(IVirtualGroupMetaData.ABI)
)

// GetAddress returns the virtualgroup precompile's fixed hex address.
func GetAddress() common.Address {
	return virtualGroupAddress
}

// GetMethod resolves an ABI method by name.
func GetMethod(name string) (abi.Method, error) {
	method := virtualGroupABI.Methods[name]
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
	event := virtualGroupABI.Events[name]
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
// abi.Arguments.Copy; their fields carry the ABI names. The keeper's msg
// ValidateBasic validates message contents, so these carry no extra validation.

// CreateGlobalVirtualGroupArgs is the decode target for the createGlobalVirtualGroup calldata.
type CreateGlobalVirtualGroupArgs struct {
	FamilyID       uint32   `abi:"familyId"`
	SecondarySpIDs []uint32 `abi:"secondarySpIds"`
	Deposit        Coin     `abi:"deposit"`
}

// DeleteGlobalVirtualGroupArgs is the decode target for the deleteGlobalVirtualGroup calldata.
type DeleteGlobalVirtualGroupArgs struct {
	GlobalVirtualGroupID uint32 `abi:"globalVirtualGroupId"`
}

// GlobalVirtualGroupFamiliesArgs is the decode target for the globalVirtualGroupFamilies calldata.
type GlobalVirtualGroupFamiliesArgs struct {
	Pagination PageRequest `abi:"pagination"`
}

// SwapOutArgs is the decode target for the swapOut calldata.
type SwapOutArgs struct {
	GvgFamilyID         uint32   `abi:"gvgFamilyId"`
	GvgIDs              []uint32 `abi:"gvgIds"`
	SuccessorSpID       uint32   `abi:"successorSpId"`
	SuccessorSpApproval Approval `abi:"successorSpApproval"`
}

// CompleteSwapOutArgs is the decode target for the completeSwapOut calldata.
type CompleteSwapOutArgs struct {
	GvgFamilyID uint32   `abi:"gvgFamilyId"`
	GvgIDs      []uint32 `abi:"gvgIds"`
}

// SPExitArgs is the decode target for the spExit calldata.
type SPExitArgs struct{}

// CompleteSPExitArgs is the decode target for the completeSPExit calldata.
type CompleteSPExitArgs struct {
	Operator string `abi:"operator"`
}

// DepositArgs is the decode target for the deposit calldata.
type DepositArgs struct {
	GlobalVirtualGroupID uint32 `abi:"globalVirtualGroupId"`
	Deposit              Coin   `abi:"deposit"`
}

// ReserveSwapInArgs is the decode target for the reserveSwapIn calldata.
type ReserveSwapInArgs struct {
	TargetSpID           uint32 `abi:"targetSpId"`
	GvgFamilyID          uint32 `abi:"gvgFamilyId"`
	GlobalVirtualGroupID uint32 `abi:"globalVirtualGroupId"`
}

// CompleteSwapInArgs is the decode target for the completeSwapIn calldata.
type CompleteSwapInArgs struct {
	GvgFamilyID          uint32 `abi:"gvgFamilyId"`
	GlobalVirtualGroupID uint32 `abi:"globalVirtualGroupId"`
}

// CancelSwapInArgs is the decode target for the cancelSwapIn calldata.
type CancelSwapInArgs struct {
	GvgFamilyID          uint32 `abi:"gvgFamilyId"`
	GlobalVirtualGroupID uint32 `abi:"globalVirtualGroupId"`
}

// GlobalVirtualGroupFamilyArgs is the decode target for the globalVirtualGroupFamily calldata.
type GlobalVirtualGroupFamilyArgs struct {
	FamilyID uint32 `abi:"familyId"`
}
