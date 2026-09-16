package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/sdk/client/test"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/client/cli"
	"github.com/mocachain/moca/v2/x/payment/types"
)

const (
	// errShortPrivateKey is returned by keys.NewPrivateKeyManager for any
	// --privatekey value that does not decode to exactly 32 bytes; "" always
	// hits it deterministically, without needing a live EVM/CometBFT backend.
	errShortPrivateKey = "len of Keybytes is not equal to 32"
	errInvalidAmount   = "invalid amount"
	cmdDeposit         = "deposit"
	cmdWithdraw        = "withdraw"
)

type CLITestSuite struct {
	suite.Suite

	kr        keyring.Keyring
	baseCtx   client.Context
	encCfg    sdktestutil.TestEncodingConfig
	clientCtx client.Context
}

func TestCLITestSuite(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}

func (s *CLITestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	s.encCfg = encoding.MakeConfig()
	s.kr = keyring.NewInMemory(s.encCfg.Codec)
	s.baseCtx = client.Context{}.
		WithKeyring(s.kr).
		WithTxConfig(s.encCfg.TxConfig).
		WithCodec(s.encCfg.Codec).
		WithClient(clitestutil.MockCometRPC{Client: rpcclientmock.Client{}}).
		WithAccountRetriever(client.MockAccountRetriever{}).
		WithOutput(io.Discard).
		WithChainID(test.TestChainID)

	accounts := testutil.CreateKeyringAccounts(s.T(), s.kr, 1)
	s.baseCtx = s.baseCtx.WithFrom(accounts[0].Address.String())
	s.baseCtx = s.baseCtx.WithFromName(accounts[0].Name)
	s.baseCtx = s.baseCtx.WithFromAddress(accounts[0].Address)

	var outBuf bytes.Buffer
	ctxGen := func() client.Context {
		bz, _ := s.encCfg.Codec.Marshal(&sdk.TxResponse{})
		c := clitestutil.NewMockCometRPC(abci.ResponseQuery{
			Value: bz,
		})

		return s.baseCtx.WithClient(c)
	}
	s.clientCtx = ctxGen().WithOutput(&outBuf)

	if testing.Short() {
		s.T().Skip("skipping test in unit-tests mode.")
	}
}

func TestGetTxCmd(t *testing.T) {
	cmd := cli.GetTxCmd()
	require.Equal(t, types.ModuleName, cmd.Name())
	require.Len(t, cmd.Commands(), 4)
}

// TestTxCmds drives every CmdXxx transaction command far enough to prove its
// argument handling, then forces a deterministic, network-free failure:
//   - a malformed --node URL fails client.GetClientTxContext while parsing
//     persistent client flags, before any key material is touched.
//   - an empty --privatekey value fails hex-decoding to exactly 32 bytes
//     inside keys.NewPrivateKeyManager, well before any EVM/CometBFT dial is
//     attempted (same recipe as x/storage/client/cli/tx_test.go).
//   - a non-numeric amount fails sdkmath.NewIntFromString before the command
//     even reaches client.GetClientTxContext.
//
// Reaching an actual broadcast (sdkclient.NewMocaClient onward) needs a live
// EVM JSON-RPC endpoint, so that remainder of each function is out of scope
// for this PR (see PR body).
func (s *CLITestSuite) TestTxCmds() {
	addr := sample.RandAccAddressHex()

	testCases := []struct {
		name         string
		args         []string
		expectErrMsg string
	}{
		{
			"create-payment-account bad node",
			[]string{
				"create-payment-account",
				fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
			},
			errInvalidNodeURL,
		},
		{
			"create-payment-account short private key",
			[]string{
				"create-payment-account",
				"--" + cli.FlagPrivateKey, "",
			},
			errShortPrivateKey,
		},
		{
			"deposit bad node",
			[]string{
				cmdDeposit,
				addr,
				"100",
				fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
			},
			errInvalidNodeURL,
		},
		{
			"deposit short private key",
			[]string{
				cmdDeposit,
				addr,
				"100",
				"--" + cli.FlagPrivateKey, "",
			},
			errShortPrivateKey,
		},
		{
			"deposit invalid amount",
			[]string{
				cmdDeposit,
				addr,
				"not-a-number",
				"--" + cli.FlagPrivateKey, "",
			},
			errInvalidAmount,
		},
		{
			"withdraw bad node",
			[]string{
				cmdWithdraw,
				addr,
				"100",
				fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
			},
			errInvalidNodeURL,
		},
		{
			"withdraw short private key",
			[]string{
				cmdWithdraw,
				addr,
				"100",
				"--" + cli.FlagPrivateKey, "",
			},
			errShortPrivateKey,
		},
		{
			"withdraw invalid amount",
			[]string{
				cmdWithdraw,
				addr,
				"not-a-number",
				"--" + cli.FlagPrivateKey, "",
			},
			errInvalidAmount,
		},
		{
			"disable-refund bad node",
			[]string{
				"disable-refund",
				addr,
				fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
			},
			errInvalidNodeURL,
		},
		{
			"disable-refund short private key",
			[]string{
				"disable-refund",
				addr,
				"--" + cli.FlagPrivateKey, "",
			},
			errShortPrivateKey,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()

			s.Require().NotPanics(func() {
				_, err := clitestutil.ExecTestCLICmd(s.clientCtx, cmd, tc.args)
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErrMsg)
			})
		})
	}
}
