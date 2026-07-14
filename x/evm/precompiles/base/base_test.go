package base_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/evm/precompiles/base"
)

// testABI has one state-changing method (foo) and one query method (bar) so the
// dispatch / read-only tests can exercise both branches.
const testABIJSON = `[
	{"type":"function","name":"foo","stateMutability":"nonpayable","inputs":[{"name":"x","type":"uint256"}],"outputs":[]},
	{"type":"function","name":"bar","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"uint256"}]}
]`

var testAddr = common.HexToAddress("0x0000000000000000000000000000000000009999")

func mustABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(testABIJSON))
	require.NoError(t, err)
	return parsed
}

// isTx marks foo as the only transaction method.
func isTx(m *abi.Method) bool { return m.Name == "foo" }

func mustContract(t *testing.T, a abi.ABI, method string, args ...interface{}) *vm.Contract {
	t.Helper()
	m := a.Methods[method]
	packed, err := m.Inputs.Pack(args...)
	require.NoError(t, err)
	input := append(append([]byte{}, m.ID...), packed...)
	c := vm.NewContract(testAddr, testAddr, uint256.NewInt(0), 100_000, nil)
	c.Input = input
	return c
}

func TestNew_Wiring(t *testing.T) {
	p := base.New(testAddr, mustABI(t))

	require.Equal(t, testAddr, p.Address(), "ContractAddress should be wired")
	require.Equal(t, uint64(0), p.KvGasConfig.ReadCostFlat, "KV gas config should be empty (per-method RequiredGas charges cost)")
	require.Equal(t, uint64(0), p.KvGasConfig.WriteCostFlat)
	require.Nil(t, p.BalanceHandlerFactory, "balance handler must be opt-in")
}

func TestNew_WithBalanceHandler(t *testing.T) {
	// A nil BankKeeper is fine here: we only assert the factory is wired in.
	p := base.New(testAddr, mustABI(t), base.WithBalanceHandler(nil))
	require.NotNil(t, p.BalanceHandlerFactory, "WithBalanceHandler should install a balance handler factory")
}

func TestDispatch_RoutesAndUnpacksTxMethod(t *testing.T) {
	a := mustABI(t)
	p := base.New(testAddr, a)

	method, args, err := p.Dispatch(mustContract(t, a, "foo", big.NewInt(7)), false, isTx)
	require.NoError(t, err)
	require.Equal(t, "foo", method.Name)
	require.Len(t, args, 1)
	require.Equal(t, big.NewInt(7), args[0])
}

func TestDispatch_AllowsQueryInReadonly(t *testing.T) {
	a := mustABI(t)
	p := base.New(testAddr, a)

	method, _, err := p.Dispatch(mustContract(t, a, "bar"), true, isTx)
	require.NoError(t, err)
	require.Equal(t, "bar", method.Name)
}

func TestDispatch_RejectsTxInReadonly(t *testing.T) {
	a := mustABI(t)
	p := base.New(testAddr, a)

	_, _, err := p.Dispatch(mustContract(t, a, "foo", big.NewInt(7)), true, isTx)
	require.ErrorIs(t, err, vm.ErrWriteProtection, "a state-changing method must be rejected in a read-only call")
}
