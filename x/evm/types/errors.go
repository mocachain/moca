// Copyright 2022 Evmos Foundation
// This file is part of the Evmos Network packages.
//
// Evmos is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Evmos packages are distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Evmos packages. If not, see https://github.com/evmos/evmos/blob/main/LICENSE
package types

import (
	"errors"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	codeErrInvalidState = uint32(iota) + 2 // NOTE: code 1 is reserved for internal errors
	codeErrInvalidChainConfig
	codeErrZeroAddress
	codeErrCreateDisabled
	codeErrCallDisabled
	codeErrInvalidAmount
	codeErrInvalidGasPrice
	codeErrInvalidGasFee
	codeErrVMExecution
	codeErrInvalidRefund
	codeErrInvalidGasCap
	codeErrInvalidBaseFee
	codeErrGasOverflow
	codeErrInvalidAccount
	codeErrInvalidGasLimit
	codeErrInvalidCaller
	codeErrReadOnly
	codeErrMaxInitCodeSizeExceeded
)

// errorCodespace is a dedicated SDK error codespace for moca's in-tree x/evm
// package. It must differ from "evm" (ModuleName): the binary also links
// cosmos/evm's x/vm module, whose types register error codes under the "evm"
// codespace, and the global error registry panics on a duplicate
// (codespace, code) registration. The in-tree x/evm is no longer a registered
// app module — it only backs the precompiles — so this codespace is internal
// and not consensus-relevant.
const errorCodespace = "mocaevmcore"

var ErrPostTxProcessing = errors.New("failed to execute post processing")

var (
	// ErrInvalidState returns an error resulting from an invalid Storage State.
	ErrInvalidState = errorsmod.Register(errorCodespace, codeErrInvalidState, "invalid storage state")

	// ErrInvalidChainConfig returns an error resulting from an invalid ChainConfig.
	ErrInvalidChainConfig = errorsmod.Register(errorCodespace, codeErrInvalidChainConfig, "invalid chain configuration")

	// ErrZeroAddress returns an error resulting from an zero (empty) ethereum Address.
	ErrZeroAddress = errorsmod.Register(errorCodespace, codeErrZeroAddress, "invalid zero address")

	// ErrCreateDisabled returns an error if the EnableCreate parameter is false.
	ErrCreateDisabled = errorsmod.Register(errorCodespace, codeErrCreateDisabled, "EVM Create operation is disabled")

	// ErrCallDisabled returns an error if the EnableCall parameter is false.
	ErrCallDisabled = errorsmod.Register(errorCodespace, codeErrCallDisabled, "EVM Call operation is disabled")

	// ErrInvalidAmount returns an error if a tx contains an invalid amount.
	ErrInvalidAmount = errorsmod.Register(errorCodespace, codeErrInvalidAmount, "invalid transaction amount")

	// ErrInvalidGasPrice returns an error if an invalid gas price is provided to the tx.
	ErrInvalidGasPrice = errorsmod.Register(errorCodespace, codeErrInvalidGasPrice, "invalid gas price")

	// ErrInvalidGasFee returns an error if the tx gas fee is out of bound.
	ErrInvalidGasFee = errorsmod.Register(errorCodespace, codeErrInvalidGasFee, "invalid gas fee")

	// ErrVMExecution returns an error resulting from an error in EVM execution.
	ErrVMExecution = errorsmod.Register(errorCodespace, codeErrVMExecution, "evm transaction execution failed")

	// ErrInvalidRefund returns an error if a the gas refund value is invalid.
	ErrInvalidRefund = errorsmod.Register(errorCodespace, codeErrInvalidRefund, "invalid gas refund amount")

	// ErrInvalidGasCap returns an error if a the gas cap value is negative or invalid
	ErrInvalidGasCap = errorsmod.Register(errorCodespace, codeErrInvalidGasCap, "invalid gas cap")

	// ErrInvalidBaseFee returns an error if a the base fee cap value is invalid
	ErrInvalidBaseFee = errorsmod.Register(errorCodespace, codeErrInvalidBaseFee, "invalid base fee")

	// ErrGasOverflow returns an error if gas computation overlow/underflow
	ErrGasOverflow = errorsmod.Register(errorCodespace, codeErrGasOverflow, "gas computation overflow/underflow")

	// ErrInvalidAccount returns an error if the account is not an EVM compatible account
	ErrInvalidAccount = errorsmod.Register(errorCodespace, codeErrInvalidAccount, "account type is not a valid ethereum account")

	// ErrInvalidGasLimit returns an error if gas limit value is invalid
	ErrInvalidGasLimit = errorsmod.Register(errorCodespace, codeErrInvalidGasLimit, "invalid gas limit")

	// ErrInvalidCaller returns an error if the caller is contract
	ErrInvalidCaller = errorsmod.Register(errorCodespace, codeErrInvalidCaller, "only be called directly to the precompile forbid from a smart contract")

	// ErrReadOnly returns an error if the precompile contract method is readonly
	ErrReadOnly = errorsmod.Register(errorCodespace, codeErrReadOnly, "precompile contract method readonly")

	// ErrMaxInitCodeSizeExceeded is returned if creation transaction provides the init code bigger
	// than init code size limit.
	ErrMaxInitCodeSizeExceeded = errorsmod.Register(errorCodespace, codeErrMaxInitCodeSizeExceeded, "max initcode size exceeded")
)

// NewExecErrorWithReason unpacks the revert return bytes and returns a wrapped error
// with the return reason.
func NewExecErrorWithReason(revertReason []byte) *RevertError {
	result := common.CopyBytes(revertReason)
	reason, errUnpack := abi.UnpackRevert(result)
	err := errors.New("execution reverted")
	if errUnpack == nil {
		err = fmt.Errorf("execution reverted: %v", reason)
	}
	return &RevertError{
		error:  err,
		reason: hexutil.Encode(result),
	}
}

// RevertError is an API error that encompass an EVM revert with JSON error
// code and a binary data blob.
type RevertError struct {
	error
	reason string // revert reason hex encoded
}

// ErrorCode returns the JSON error code for a revert.
// See: https://github.com/ethereum/wiki/wiki/JSON-RPC-Error-Codes-Improvement-Proposal
func (e *RevertError) ErrorCode() int {
	return 3
}

// ErrorData returns the hex encoded revert reason.
func (e *RevertError) ErrorData() interface{} {
	return e.reason
}

func PackRetError(str string) ([]byte, error) {
	pack, _ := abi.Arguments{{Type: TypeString}}.Pack(str)
	return pack, errors.New(str)
}
