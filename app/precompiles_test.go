package app

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"math/big"
	"slices"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// forwarders call the address in the first calldata word with the rest of the
// calldata, by CALL or STATICCALL, and return, or revert with, its return data.
var forwarders = []struct {
	name string
	addr common.Address
	code []byte
}{
	{"call", common.HexToAddress("0x7777777777777777777777777777777777777777"), common.FromHex("6020360360206000376000600060203603600060006000355af13d600060003e6027573d6000fd5b3d6000f3")},
	{"staticcall", common.HexToAddress("0x7777777777777777777777777777777777777778"), common.FromHex("602036036020600037600060006020360360006000355afa3d600060003e6025573d6000fd5b3d6000f3")},
}

// TestStaticPrecompilesResolve checks that every standard Ethereum precompile
// and each of moca's resolves from the EVM keeper at its own address.
func TestStaticPrecompilesResolve(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	params := evmtypes.Params{ActiveStaticPrecompiles: MocaActiveStaticPrecompiles()}
	// The keeper's map is fixed, so its standard entries must match the
	// precompiles of the latest fork the chain config enables.
	latest := evmtypes.GetEthChainConfig().Rules(big.NewInt(math.MaxInt64), true, math.MaxUint64)
	standard := vm.ActivePrecompiledContracts(latest)

	for _, addr := range slices.Concat(vm.PrecompiledAddressesPrague, params.GetActiveStaticPrecompilesAddrs()) {
		t.Run(addr.Hex(), func(t *testing.T) {
			var (
				precompile vm.PrecompiledContract
				found      bool
				err        error
			)
			require.NotPanics(t, func() {
				precompile, found, err = mocaApp.EvmKeeper.GetStaticPrecompileInstance(&params, addr)
			})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, addr, precompile.Address())
			if want, ok := standard[addr]; ok {
				require.Equal(t, want, precompile)
			}
		})
	}
}

// TestStandardPrecompilesCallable runs sha256 and ecrecover through eth_call,
// both directly and from a contract by CALL and by STATICCALL.
func TestStandardPrecompilesCallable(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)
	proposer := firstValidatorConsAddr(t, mocaApp, ctx)

	stateDB := statedb.New(ctx, mocaApp.EvmKeeper, statedb.NewEmptyTxConfig())
	for _, f := range forwarders {
		stateDB.SetCode(f.addr, f.code)
	}
	require.NoError(t, stateDB.Commit())

	msg := []byte("moca")
	digest := sha256.Sum256(msg)
	hash := crypto.Keccak256(msg)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	sig, err := crypto.Sign(hash, key)
	require.NoError(t, err)

	tests := []struct {
		name       string
		precompile common.Address
		input      []byte
		want       []byte
	}{
		{"sha256", common.BytesToAddress([]byte{0x02}), msg, digest[:]},
		// ecrecover takes (hash, v, r, s) and returns the signer as a 32-byte word.
		{
			"ecrecover", common.BytesToAddress([]byte{0x01}),
			slices.Concat(hash, common.LeftPadBytes([]byte{sig[64] + 27}, 32), sig[:64]),
			common.LeftPadBytes(crypto.PubkeyToAddress(key.PublicKey).Bytes(), 32),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name+"/eoa", func(t *testing.T) {
			require.Equal(t, tc.want, ethCall(t, mocaApp, ctx, proposer, tc.precompile, tc.input))
		})
		for _, f := range forwarders {
			t.Run(tc.name+"/"+f.name, func(t *testing.T) {
				input := slices.Concat(common.LeftPadBytes(tc.precompile.Bytes(), 32), tc.input)
				require.Equal(t, tc.want, ethCall(t, mocaApp, ctx, proposer, f.addr, input))
			})
		}
	}
}

// ethCall sends input to `to` through the eth_call query and returns the output.
func ethCall(t *testing.T, app *Moca, ctx sdk.Context, proposer sdk.ConsAddress, to common.Address, input []byte) []byte {
	t.Helper()
	from := common.HexToAddress("0x00000000000000000000000000000000000000ff")
	args, err := json.Marshal(evmtypes.TransactionArgs{From: &from, To: &to, Input: (*hexutil.Bytes)(&input)})
	require.NoError(t, err)

	var res *evmtypes.MsgEthereumTxResponse
	require.NotPanics(t, func() {
		res, err = app.EvmKeeper.EthCall(ctx, &evmtypes.EthCallRequest{Args: args, GasCap: 25_000_000, ProposerAddress: proposer})
	}, to.Hex())
	require.NoError(t, err)
	require.False(t, res.Failed(), res.VmError)
	return res.Ret
}
