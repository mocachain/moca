package virtualgroup

import (
	"math/big"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	mocacommon "github.com/mocachain/moca/v2/types/common"
	virtualgrouptypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

const (
	// CreateGlobalVirtualGroupMethodName is the ABI name for the createGlobalVirtualGroup transaction.
	CreateGlobalVirtualGroupMethodName = "createGlobalVirtualGroup"
	// DeleteGlobalVirtualGroupMethodName is the ABI name for the deleteGlobalVirtualGroup transaction.
	DeleteGlobalVirtualGroupMethodName = "deleteGlobalVirtualGroup"
	// SwapOutMethodName is the ABI name for the swapOut transaction.
	SwapOutMethodName = "swapOut"
	// CompleteSwapOutMethodName is the ABI name for the completeSwapOut transaction.
	CompleteSwapOutMethodName = "completeSwapOut"
	// SPExitMethodName is the ABI name for the spExit transaction.
	SPExitMethodName = "spExit"
	// CompleteSPExitMethodName is the ABI name for the completeSPExit transaction.
	CompleteSPExitMethodName = "completeSPExit"
	// DepositMethodName is the ABI name for the deposit transaction.
	DepositMethodName = "deposit"
	// ReserveSwapInMethodName is the ABI name for the reserveSwapIn transaction.
	ReserveSwapInMethodName = "reserveSwapIn"
	// CompleteSwapInMethodName is the ABI name for the completeSwapIn transaction.
	CompleteSwapInMethodName = "completeSwapIn"
	// CancelSwapInMethodName is the ABI name for the cancelSwapIn transaction.
	CancelSwapInMethodName = "cancelSwapIn"
)

// CreateGlobalVirtualGroup defines a method for sp create a global virtual group.
func (p Precompile) CreateGlobalVirtualGroup(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input CreateGlobalVirtualGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgCreateGlobalVirtualGroup{
		StorageProvider: contract.Caller().String(),
		FamilyId:        input.FamilyID,
		SecondarySpIds:  input.SecondarySpIDs,
		Deposit: sdk.Coin{
			Denom:  input.Deposit.Denom,
			Amount: math.NewIntFromBigInt(input.Deposit.Amount),
		},
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.CreateGlobalVirtualGroup(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCreateGlobalVirtualGroupEvent(evm, contract.Caller(), big.NewInt(int64(input.FamilyID))); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DeleteGlobalVirtualGroup defines a method for sp delete a global virtual group.
func (p Precompile) DeleteGlobalVirtualGroup(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DeleteGlobalVirtualGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgDeleteGlobalVirtualGroup{
		StorageProvider:      contract.Caller().String(),
		GlobalVirtualGroupId: input.GlobalVirtualGroupID,
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.DeleteGlobalVirtualGroup(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDeleteGlobalVirtualGroupEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// SwapOut defines a method for sp to remove itself from all Virtual Groups.
func (p Precompile) SwapOut(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input SwapOutArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgSwapOut{
		StorageProvider:            contract.Caller().String(),
		GlobalVirtualGroupFamilyId: input.GvgFamilyID,
		GlobalVirtualGroupIds:      input.GvgIDs,
		SuccessorSpId:              input.SuccessorSpID,
		SuccessorSpApproval: &mocacommon.Approval{
			ExpiredHeight:              input.SuccessorSpApproval.ExpiredHeight,
			GlobalVirtualGroupFamilyId: input.SuccessorSpApproval.GlobalVirtualGroupFamilyId,
			Sig:                        input.SuccessorSpApproval.Sig,
		},
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.SwapOut(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSwapOutEvent(evm, contract.Caller(), big.NewInt(int64(input.GvgFamilyID))); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CompleteSwapOut defines a method for sp somplete to remove itself from all Virtual Groups.
func (p Precompile) CompleteSwapOut(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input CompleteSwapOutArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgCompleteSwapOut{
		StorageProvider:            contract.Caller().String(),
		GlobalVirtualGroupFamilyId: input.GvgFamilyID,
		GlobalVirtualGroupIds:      input.GvgIDs,
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.CompleteSwapOut(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCompleteSwapOutEvent(evm, contract.Caller(), big.NewInt(int64(input.GvgFamilyID))); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// SPExit defines a method for sp to exit.
func (p Precompile) SPExit(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input SPExitArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgStorageProviderExit{
		StorageProvider: contract.Caller().String(),
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.StorageProviderExit(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitSPExitEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CompleteSPExit defines a method for sp complete to exit.
func (p Precompile) CompleteSPExit(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input CompleteSPExitArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgCompleteStorageProviderExit{
		StorageProvider: contract.Caller().String(),
		// Operator is the signer per GetSigners(), so it must be the caller too (#365).
		Operator: contract.Caller().String(),
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.CompleteStorageProviderExit(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCompleteSPExitEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Deposit defines a method to deposit more tokens for the objects stored on it.
func (p Precompile) Deposit(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DepositArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgDeposit{
		StorageProvider:      contract.Caller().String(),
		GlobalVirtualGroupId: input.GlobalVirtualGroupID,
		Deposit: sdk.Coin{
			Denom:  input.Deposit.Denom,
			Amount: math.NewIntFromBigInt(input.Deposit.Amount),
		},
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.Deposit(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitDepositEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// ReserveSwapIn defines a method for sp to reserve a swap in.
func (p Precompile) ReserveSwapIn(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ReserveSwapInArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgReserveSwapIn{
		StorageProvider:            contract.Caller().String(),
		TargetSpId:                 input.TargetSpID,
		GlobalVirtualGroupFamilyId: input.GvgFamilyID,
		GlobalVirtualGroupId:       input.GlobalVirtualGroupID,
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.ReserveSwapIn(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitReserveSwapInEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CompleteSwapIn defines a method for sp to complete a swap in.
func (p Precompile) CompleteSwapIn(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input CompleteSwapInArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgCompleteSwapIn{
		StorageProvider:            contract.Caller().String(),
		GlobalVirtualGroupFamilyId: input.GvgFamilyID,
		GlobalVirtualGroupId:       input.GlobalVirtualGroupID,
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.CompleteSwapIn(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCompleteSwapInEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// CancelSwapIn defines a method for sp to cancel a swap in.
func (p Precompile) CancelSwapIn(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input CancelSwapInArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &virtualgrouptypes.MsgCancelSwapIn{
		StorageProvider:            contract.Caller().String(),
		GlobalVirtualGroupFamilyId: input.GvgFamilyID,
		GlobalVirtualGroupId:       input.GlobalVirtualGroupID,
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.virtualGroupMsgServer.CancelSwapIn(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitCancelSwapInEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
