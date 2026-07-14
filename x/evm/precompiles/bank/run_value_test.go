package bank

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// newTestEVM builds a minimal EVM good enough to exercise the precompile's value
// guard. The value guard reverts before any StateDB access, so a nil StateDB is
// fine; NewEVM still wires an interpreter, which the revert path needs.
func newTestEVM() *vm.EVM {
	return vm.NewEVM(
		vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1},
		nil,
		&params.ChainConfig{ChainID: big.NewInt(1)},
		vm.Config{},
	)
}

// TestRun_RejectsNonzeroValue verifies the value guard still fires under the
// native runtime. This is part of the precompile migration baseline: the move to
// cosmos/evm's RunNativeAction must keep rejecting native value, now surfaced as a
// proper EVM revert whose reason is ABI-encoded in the return data.
func TestRun_RejectsNonzeroValue(t *testing.T) {
	c := &Contract{}
	caller := common.HexToAddress("0x1000000000000000000000000000000000000001")
	evm := newTestEVM()

	t.Run("nonzero value reverts with the value-rejection reason", func(t *testing.T) {
		contract := vm.NewContract(caller, bankAddress, uint256.NewInt(1), 0, nil)
		contract.Input = MustMethod(SendMethodName).ID // bank.send selector

		ret, err := c.Run(evm, contract, false)
		require.ErrorIs(t, err, vm.ErrExecutionReverted, "value-bearing precompile call must revert")
		reason, uErr := abi.UnpackRevert(ret)
		require.NoError(t, uErr)
		require.Equal(t, "precompile does not accept value", reason)
	})

	t.Run("zero value passes the value guard", func(t *testing.T) {
		contract := vm.NewContract(caller, bankAddress, uint256.NewInt(0), 0, nil)
		contract.Input = MustMethod(SendMethodName).ID

		ret, err := c.Run(evm, contract, false)
		// With a nil StateDB the call still reverts (not run within the cosmos/evm
		// StateDB), but crucially NOT with the value-rejection reason: the zero-value
		// call got past the guard.
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, uErr := abi.UnpackRevert(ret)
		require.NoError(t, uErr)
		require.NotEqual(t, "precompile does not accept value", reason,
			"a zero-value call must get past the value guard")
	})
}
