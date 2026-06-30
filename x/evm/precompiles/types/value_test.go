package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// TestRejectValue verifies that nil/zero values are accepted and nonzero values are rejected.
func TestRejectValue(t *testing.T) {
	addr := common.HexToAddress("0x1000000000000000000000000000000000000001")

	t.Run("nil value is accepted", func(t *testing.T) {
		c := vm.NewContract(addr, addr, nil, 0, nil)
		require.NoError(t, RejectValue(c))
	})

	t.Run("zero value is accepted", func(t *testing.T) {
		c := vm.NewContract(addr, addr, uint256.NewInt(0), 0, nil)
		require.NoError(t, RejectValue(c))
	})

	t.Run("one wei is rejected", func(t *testing.T) {
		c := vm.NewContract(addr, addr, uint256.NewInt(1), 0, nil)
		require.ErrorIs(t, RejectValue(c), ErrValueNotAccepted)
	})

	t.Run("large value is rejected", func(t *testing.T) {
		c := vm.NewContract(addr, addr, uint256.NewInt(1_000_000_000_000_000_000), 0, nil)
		require.ErrorIs(t, RejectValue(c), ErrValueNotAccepted)
	})
}
