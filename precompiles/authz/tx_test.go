package authz_test

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/precompiles/authz"
	"github.com/mocachain/moca/v2/testutil"
	"github.com/mocachain/moca/v2/utils"
)

// fixture wires the authz precompile to a real app so exec and grant run against
// the live message router and authz keeper.
type fixture struct {
	app      *app.Moca
	ctx      sdk.Context
	caller   common.Address
	evm      *vm.EVM
	contract *vm.Contract
	p        *authz.Precompile
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	a := app.EthSetup(false, nil)
	ctx := a.NewContext(false)
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	require.NoError(t, testutil.FundAccountWithBaseDenom(ctx, a.BankKeeper, sdk.AccAddress(caller.Bytes()), 1_000_000_000_000))

	stateDB := statedb.New(ctx, a.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: caller})

	return &fixture{
		app:      a,
		ctx:      ctx,
		caller:   caller,
		evm:      evm,
		contract: vm.NewContract(caller, authz.GetAddress(), uint256.NewInt(0), 10_000_000, nil),
		p:        authz.NewPrecompile(a.AuthzKeeper, a.BankKeeper),
	}
}

// execArgs encodes msg the way the exec method expects it on the ABI boundary.
func (f *fixture) execArgs(t *testing.T, msg sdk.Msg) []interface{} {
	t.Helper()
	bz, err := f.app.AppCodec().MarshalInterfaceJSON(msg)
	require.NoError(t, err)
	return []interface{}{[]string{string(bz)}}
}

func (f *fixture) sendMsg() *banktypes.MsgSend {
	return banktypes.NewMsgSend(
		sdk.AccAddress(f.caller.Bytes()),
		sdk.AccAddress(common.HexToAddress("0x2222222222222222222222222222222222222222").Bytes()),
		sdk.NewCoins(sdk.NewInt64Coin(utils.BaseDenom, 1)),
	)
}

// TestExecUnsupportedMessageTypes pins that exec only carries the message types the
// precompile supports: an EVM message and a nested authz exec are both refused
// before they reach the message router.
func TestExecUnsupportedMessageTypes(t *testing.T) {
	f := newFixture(t)
	method := authz.MustMethod(authz.ExecMethodName)

	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	evmMsg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		ChainID:  big.NewInt(1),
		Nonce:    0,
		GasLimit: 21_000,
		GasPrice: big.NewInt(1),
		To:       &to,
		Amount:   big.NewInt(0),
	})
	evmMsg.From = f.caller.Bytes()

	nested := authztypes.NewMsgExec(sdk.AccAddress(f.caller.Bytes()), []sdk.Msg{f.sendMsg()})

	testCases := []struct {
		name       string
		msg        sdk.Msg
		msgTypeURL string
	}{
		{"evm message", evmMsg, sdk.MsgTypeURL(evmMsg)},
		{"nested exec", &nested, sdk.MsgTypeURL(&nested)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.p.Exec(f.ctx, f.evm, f.contract, &method, f.execArgs(t, tc.msg))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.msgTypeURL)
		})
	}
}

// TestExecSupportedMessageType is the counterpart: a message type the precompile
// supports still executes.
func TestExecSupportedMessageType(t *testing.T) {
	f := newFixture(t)
	method := authz.MustMethod(authz.ExecMethodName)

	send := f.sendMsg()
	_, err := f.p.Exec(f.ctx, f.evm, f.contract, &method, f.execArgs(t, send))
	require.NoError(t, err)

	recipient := sdk.MustAccAddressFromHex(send.ToAddress)
	require.Equal(t, "1", f.app.BankKeeper.GetBalance(f.ctx, recipient, utils.BaseDenom).Amount.String())
}

// TestGrantUnsupportedMessageType pins that a generic grant cannot authorize a
// message type the precompile does not support.
func TestGrantUnsupportedMessageType(t *testing.T) {
	f := newFixture(t)
	method := authz.MustMethod(authz.GrantMethodName)

	grantee := common.HexToAddress("0x3333333333333333333333333333333333333333")
	msgTypeURL := sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{})

	_, err := f.p.Grant(f.ctx, f.evm, f.contract, &method,
		[]interface{}{grantee, authz.AuthzTypeGeneric, msgTypeURL, []authz.Coin{}, int64(0)})
	require.Error(t, err)
	require.Contains(t, err.Error(), msgTypeURL)

	grant, _ := f.app.AuthzKeeper.GetAuthorization(f.ctx, sdk.AccAddress(grantee.Bytes()), sdk.AccAddress(f.caller.Bytes()), msgTypeURL)
	require.Nil(t, grant)
}

// TestGrantSupportedMessageType is the counterpart: a generic grant for a
// supported message type is still stored.
func TestGrantSupportedMessageType(t *testing.T) {
	f := newFixture(t)
	method := authz.MustMethod(authz.GrantMethodName)

	grantee := common.HexToAddress("0x3333333333333333333333333333333333333333")
	msgTypeURL := sdk.MsgTypeURL(&banktypes.MsgSend{})

	_, err := f.p.Grant(f.ctx, f.evm, f.contract, &method,
		[]interface{}{grantee, authz.AuthzTypeGeneric, msgTypeURL, []authz.Coin{}, int64(0)})
	require.NoError(t, err)

	grant, _ := f.app.AuthzKeeper.GetAuthorization(f.ctx, sdk.AccAddress(grantee.Bytes()), sdk.AccAddress(f.caller.Bytes()), msgTypeURL)
	require.NotNil(t, grant)
	require.Equal(t, msgTypeURL, grant.MsgTypeURL())
}
