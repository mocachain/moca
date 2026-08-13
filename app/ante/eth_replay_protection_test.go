package ante_test

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmantetypes "github.com/cosmos/evm/ante/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/app/ante"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/utils"
)

// signTx builds a legacy transfer signed under the given signer, wrapped as the
// MsgEthereumTx the ante sees.
func signTx(t *testing.T, signer ethtypes.Signer) *evmtypes.MsgEthereumTx {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	to := crypto.PubkeyToAddress(key.PublicKey)

	ethTx, err := ethtypes.SignNewTx(key, signer, &ethtypes.LegacyTx{
		Nonce: 0, To: &to, Value: big.NewInt(1), Gas: 21000, GasPrice: big.NewInt(1),
	})
	require.NoError(t, err)

	msg := &evmtypes.MsgEthereumTx{}
	require.NoError(t, msg.FromSignedEthereumTx(ethTx, signer))
	return msg
}

// buildTx wraps the message the way the production path does, so the decorator
// is handed the same sdk.Tx shape it sees in the ante chain.
func buildTx(t *testing.T, msg *evmtypes.MsgEthereumTx) sdk.Tx {
	t.Helper()
	txConfig := encoding.MakeConfig().TxConfig
	// explicit denoms: BuildTx reads them from global EVM config a unit test has not set up
	tx, err := msg.BuildTxWithEvmParams(txConfig.NewTxBuilder(), evmtypes.Params{
		EvmDenom:             utils.BaseDenom,
		ExtendedDenomOptions: &evmtypes.ExtendedDenomOptions{ExtendedDenom: utils.BaseDenom},
	})
	require.NoError(t, err)
	return tx
}

// TestRejectsUnprotected is the regression: a transaction with no chain id in its
// signature is valid on any chain, and nothing else in the pipeline turns it away.
func TestRejectsUnprotected(t *testing.T) {
	msg := signTx(t, ethtypes.HomesteadSigner{})
	require.False(t, msg.AsTransaction().Protected(), "fixture must be unprotected or the test proves nothing")

	d := ante.NewEthReplayProtectionDecorator()
	_, err := d.AnteHandle(sdk.Context{}, buildTx(t, msg), false, passthrough)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rejected unprotected Ethereum transaction")
}

// TestAllowsProtected keeps the decorator from rejecting ordinary traffic. The
// chain id it names is not this chain's; binding it is the signer's job, not this
// decorator's, and conflating the two would reject every valid transaction.
func TestAllowsProtected(t *testing.T) {
	msg := signTx(t, ethtypes.NewEIP155Signer(big.NewInt(2288)))
	require.True(t, msg.AsTransaction().Protected())

	d := ante.NewEthReplayProtectionDecorator()
	_, err := d.AnteHandle(sdk.Context{}, buildTx(t, msg), false, passthrough)
	require.NoError(t, err)
}

func passthrough(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return ctx, nil }

// TestAnteHandlerRejectsUnprotected pins the wiring, not just the decorator.
// The gap this closes was created by a check being removed from the chain while
// the rest of the pipeline kept working, so a test that only exercises the
// decorator in isolation would not have caught it: unwire the decorator and the
// two cases above still pass. This one drives the real handler that app.go
// builds, so it fails if the decorator is ever dropped from the chain.
func (suite *AnteTestSuite) TestAnteHandlerRejectsUnprotected() {
	handler := ante.NewAnteHandler(ante.HandlerOptions{
		Cdc:                    suite.app.AppCodec(),
		AccountKeeper:          suite.app.AccountKeeper,
		BankKeeper:             suite.app.BankKeeper,
		ExtensionOptionChecker: evmantetypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              suite.app.EvmKeeper,
		FeegrantKeeper:         suite.app.FeeGrantKeeper,
		DistributionKeeper:     suite.app.DistrKeeper,
		FeeMarketKeeper:        suite.app.FeeMarketKeeper,
		SignModeHandler:        encoding.MakeConfig().TxConfig.SignModeHandler(),
		SigGasConsumer:         ante.SigVerificationGasConsumer,
		MaxTxGasWanted:         0,
	})

	msg := signTx(suite.T(), ethtypes.HomesteadSigner{})
	suite.Require().False(msg.AsTransaction().Protected())

	_, err := handler(suite.ctx, buildTx(suite.T(), msg), false)
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "rejected unprotected Ethereum transaction",
		"the EVM ante chain must reject an unprotected transaction")
}
