package authz_test

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/testutil"
	"github.com/evmos/evmos/v12/utils"
	"github.com/evmos/evmos/v12/x/evm/precompiles/authz"
	"github.com/evmos/evmos/v12/x/evm/statedb"
	evmtypes "github.com/evmos/evmos/v12/x/evm/types"
)

// fixture wires the authz precompile to a real app so exec and grant run against
// the live message router and authz keeper.
type fixture struct {
	app    *app.Evmos
	ctx    sdk.Context
	caller common.Address
	evm    *vm.EVM
	c      *authz.Contract
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	a := app.EthSetup(false, nil)
	ctx := a.BaseApp.NewContext(false)
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	require.NoError(t, testutil.FundAccountWithBaseDenom(ctx, a.BankKeeper, sdk.AccAddress(caller.Bytes()), 1_000_000_000_000))

	evm := &vm.EVM{
		Context:   vm.BlockContext{BlockNumber: big.NewInt(1)},
		TxContext: vm.TxContext{Origin: caller},
		StateDB:   statedb.New(ctx, a.EvmKeeper, statedb.NewEmptyTxConfig(common.Hash{})),
	}

	return &fixture{
		app:    a,
		ctx:    ctx,
		caller: caller,
		evm:    evm,
		c:      authz.NewPrecompiledContract(ctx, a.AuthzKeeper),
	}
}

// contractFor packs args the way the precompile expects them on the ABI boundary.
func (f *fixture) contractFor(t *testing.T, methodName string, args ...interface{}) *vm.Contract {
	t.Helper()

	method := authz.MustMethod(methodName)
	packed, err := method.Inputs.Pack(args...)
	require.NoError(t, err)

	contract := vm.NewContract(vm.AccountRef(f.caller), vm.AccountRef(authz.GetAddress()), big.NewInt(0), 10_000_000)
	contract.Input = append(append([]byte{}, method.ID...), packed...)
	return contract
}

// execContractFor JSON-encodes msg and packs it as the single exec payload.
func (f *fixture) execContractFor(t *testing.T, msg sdk.Msg) *vm.Contract {
	t.Helper()

	bz, err := f.app.AppCodec().MarshalInterfaceJSON(msg)
	require.NoError(t, err)
	return f.contractFor(t, authz.ExecMethodName, []string{string(bz)})
}

func (f *fixture) sendMsg() *banktypes.MsgSend {
	return banktypes.NewMsgSend(
		sdk.AccAddress(f.caller.Bytes()),
		sdk.AccAddress(common.HexToAddress("0x2222222222222222222222222222222222222222").Bytes()),
		sdk.NewCoins(sdk.NewInt64Coin(utils.BaseDenom, 1)),
	)
}

// TestExecRejectsUnsupportedMessageTypes pins that exec only carries the message
// types the precompile supports: an EVM message and authz's own wrapping messages
// are refused before they reach the message router.
func TestExecRejectsUnsupportedMessageTypes(t *testing.T) {
	f := newFixture(t)

	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	evmMsg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		ChainID:  big.NewInt(1),
		Nonce:    0,
		GasLimit: 21_000,
		GasPrice: big.NewInt(1),
		To:       &to,
		Amount:   big.NewInt(0),
	})
	evmMsg.From = f.caller.Hex()

	nestedExec := authztypes.NewMsgExec(sdk.AccAddress(f.caller.Bytes()), []sdk.Msg{f.sendMsg()})

	testCases := []struct {
		name string
		msg  sdk.Msg
	}{
		{"evm message", evmMsg},
		{"nested exec", &nestedExec},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.c.Exec(f.ctx, f.evm, f.execContractFor(t, tc.msg), false)
			require.Error(t, err)
			require.Contains(t, err.Error(), sdk.MsgTypeURL(tc.msg))
		})
	}
}

// TestExecSupportedMessageType is the counterpart: a message type the precompile
// supports still executes.
func TestExecSupportedMessageType(t *testing.T) {
	f := newFixture(t)

	send := f.sendMsg()
	_, err := f.c.Exec(f.ctx, f.evm, f.execContractFor(t, send), false)
	require.NoError(t, err)

	recipient := sdk.MustAccAddressFromHex(send.ToAddress)
	require.Equal(t, "1", f.app.BankKeeper.GetBalance(f.ctx, recipient, utils.BaseDenom).Amount.String())
}

// TestGrantRejectsUnsupportedMessageType pins that a generic grant cannot
// authorize a message type the precompile does not support.
func TestGrantRejectsUnsupportedMessageType(t *testing.T) {
	f := newFixture(t)

	grantee := common.HexToAddress("0x3333333333333333333333333333333333333333")
	msgTypeURL := sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{})
	contract := f.contractFor(t, authz.GrantMethodName, grantee, authz.AuthzTypeGeneric, msgTypeURL, []authz.Coin{}, int64(0))

	_, err := f.c.Grant(f.ctx, f.evm, contract, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), msgTypeURL)

	grant, _ := f.app.AuthzKeeper.GetAuthorization(f.ctx, sdk.AccAddress(grantee.Bytes()), sdk.AccAddress(f.caller.Bytes()), msgTypeURL)
	require.Nil(t, grant)
}

// TestGrantSupportedMessageType is the counterpart: a generic grant for a
// supported message type is still stored.
func TestGrantSupportedMessageType(t *testing.T) {
	f := newFixture(t)

	grantee := common.HexToAddress("0x3333333333333333333333333333333333333333")
	msgTypeURL := sdk.MsgTypeURL(&banktypes.MsgSend{})
	contract := f.contractFor(t, authz.GrantMethodName, grantee, authz.AuthzTypeGeneric, msgTypeURL, []authz.Coin{}, int64(0))

	_, err := f.c.Grant(f.ctx, f.evm, contract, false)
	require.NoError(t, err)

	grant, _ := f.app.AuthzKeeper.GetAuthorization(f.ctx, sdk.AccAddress(grantee.Bytes()), sdk.AccAddress(f.caller.Bytes()), msgTypeURL)
	require.NotNil(t, grant)
	require.Equal(t, msgTypeURL, grant.MsgTypeURL())
}
