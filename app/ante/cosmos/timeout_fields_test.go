package cosmos_test

import (
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	evmtestutil "github.com/cosmos/evm/testutil"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	cosmosante "github.com/mocachain/moca/v2/app/ante/cosmos"
	"github.com/mocachain/moca/v2/encoding"
)

// buildTimeoutTx builds a tx with timeout_timestamp/unordered set directly
// on the TxBuilder.
func buildTimeoutTx(t *testing.T, ts time.Time, unordered bool, msgs ...sdk.Msg) sdk.Tx {
	t.Helper()
	encodingConfig := encoding.MakeConfig()
	txBuilder := encodingConfig.TxConfig.NewTxBuilder()
	txBuilder.SetGasLimit(1000000)
	require.NoError(t, txBuilder.SetMsgs(msgs...))
	if !ts.IsZero() {
		txBuilder.SetTimeoutTimestamp(ts)
	}
	txBuilder.SetUnordered(unordered)
	return txBuilder.GetTx()
}

func TestRejectUnsupportedTxFieldsDecorator(t *testing.T) {
	_, testAddresses, err := generatePrivKeyAddressPairs(2)
	require.NoError(t, err)

	msg := banktypes.NewMsgSend(
		testAddresses[0],
		testAddresses[1],
		sdk.NewCoins(sdk.NewInt64Coin(evmtypes.DefaultEVMDenom, 100e6)),
	)

	decorator := cosmosante.RejectUnsupportedTxFieldsDecorator{}

	testCases := []struct {
		name        string
		timeoutTs   time.Time
		unordered   bool
		expectedErr error
	}{
		{
			name: "no timeout_timestamp and not unordered - passes",
		},
		{
			name:        "future timeout_timestamp - rejected",
			timeoutTs:   time.Now().Add(time.Hour),
			expectedErr: sdkerrors.ErrNotSupported,
		},
		{
			name:        "past timeout_timestamp - rejected",
			timeoutTs:   time.Now().Add(-time.Hour),
			expectedErr: sdkerrors.ErrNotSupported,
		},
		{
			name:        "unordered flag - rejected",
			unordered:   true,
			expectedErr: sdkerrors.ErrNotSupported,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := buildTimeoutTx(t, tc.timeoutTs, tc.unordered, msg)

			_, err := decorator.AnteHandle(sdk.Context{}, tx, false, evmtestutil.NoOpNextFn)
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestRejectUnsupportedTxFieldsFullChain proves the decorator is wired into
// the production cosmos-tx ante handler chain, not just unit-tested in
// isolation.
func (suite *AnteTestSuite) TestRejectUnsupportedTxFieldsFullChain() {
	_, testAddresses, err := generatePrivKeyAddressPairs(2)
	suite.Require().NoError(err)

	msg := banktypes.NewMsgSend(
		testAddresses[0],
		testAddresses[1],
		sdk.NewCoins(sdk.NewInt64Coin(evmtypes.DefaultEVMDenom, 100e6)),
	)

	tx := buildTimeoutTx(suite.T(), time.Now().Add(time.Hour), false, msg)

	txEncoder := suite.clientCtx.TxConfig.TxEncoder()
	bz, err := txEncoder(tx)
	suite.Require().NoError(err)

	resCheckTx, err := suite.app.CheckTx(
		&abci.RequestCheckTx{
			Tx:   bz,
			Type: abci.CheckTxType_New,
		},
	)
	suite.Require().NoError(err)
	suite.Require().Equal(sdkerrors.ErrNotSupported.ABCICode(), resCheckTx.Code, resCheckTx.Log)
}
