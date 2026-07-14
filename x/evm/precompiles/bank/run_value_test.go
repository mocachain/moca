package bank

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// TestRun_RejectsNonzeroValue verifies the value guard fires before any StateDB access.
// This is part of the precompile migration baseline: later runtime rewrites must
// still reject native value before touching the Cosmos/EVM execution context.
// A nil EVM is intentional: if the guard were absent the nil dereference would cause a panic.
func TestRun_RejectsNonzeroValue(t *testing.T) {
	c := &Contract{}
	caller := common.HexToAddress("0x1000000000000000000000000000000000000001")

	t.Run("nonzero value is rejected before dispatch", func(t *testing.T) {
		contract := vm.NewContract(caller, bankAddress, uint256.NewInt(1), 0, nil)
		contract.Input = MustMethod(SendMethodName).ID // bank.send selector

		var err error
		require.NotPanics(t, func() { _, err = c.Run(nil, contract, false) })
		require.Error(t, err, "value-bearing precompile call must be rejected")
		require.Equal(t, "precompile does not accept value", err.Error())
	})

	t.Run("zero value passes the guard", func(t *testing.T) {
		contract := vm.NewContract(caller, bankAddress, uint256.NewInt(0), 0, nil)
		contract.Input = []byte{}

		var err error
		require.NotPanics(t, func() { _, err = c.Run(nil, contract, false) })
		require.Error(t, err)
		require.Equal(t, "invalid input", err.Error())
	})
}
