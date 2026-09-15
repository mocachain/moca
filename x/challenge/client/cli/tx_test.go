package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"cosmossdk.io/math"
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
	"github.com/mocachain/moca/v2/x/challenge/client/cli"
	"github.com/mocachain/moca/v2/x/challenge/types"
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
	require.Len(t, cmd.Commands(), 2)
}

// TestTxCmd drives CmdSubmit and CmdAttest far enough to prove their argument
// handling and, on a well-formed row, all the way through a successful
// broadcast: the mocked CometBFT RPC (clitestutil.NewMockCometRPC) answers
// BroadcastTxSync with a canned OK response regardless of content, and the
// keyring holds a real key, so tx.GenerateOrBroadcastTxCLI needs no live
// node to complete (same recipe as x/virtualgroup/client/cli/tx_test.go).
func (s *CLITestSuite) TestTxCmd() {
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagFrom, s.clientCtx.GetFromAddress().String()),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin("amoca", math.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}

	spOperatorAddr := sample.RandAccAddressHex()
	// validSig is a well-formed (64-byte) BLS signature hex string: long
	// enough to pass MsgAttest.ValidateBasic's length check, so the happy
	// path row reaches the broadcast call.
	validSig := strings.Repeat("ab", types.BlsSignatureLength)

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectErrMsg string
	}{
		{
			"submit bad sp-operator-address",
			append([]string{
				cmdSubmit,
				argNotAnAddress, argTestBucketName, argTestObjectName, argTrue, "0",
			}, commonFlags...),
			true, "please input a valid sp-operator-address",
		},
		{
			"submit bad bucket-name",
			append([]string{
				cmdSubmit,
				spOperatorAddr, "ab", argTestObjectName, argTrue, "0",
			}, commonFlags...),
			true, "please input a valid bucket-name",
		},
		{
			"submit bad object-name",
			append([]string{
				cmdSubmit,
				spOperatorAddr, argTestBucketName, "a//b", argTrue, "0",
			}, commonFlags...),
			true, "please input a valid object-name",
		},
		{
			"submit bad random-index",
			append([]string{
				cmdSubmit,
				spOperatorAddr, argTestBucketName, argTestObjectName, "not-a-bool", "0",
			}, commonFlags...),
			true, "please input a valid random-index",
		},
		{
			"submit bad segment-index",
			append([]string{
				cmdSubmit,
				spOperatorAddr, argTestBucketName, argTestObjectName, "false", argNotANumber,
			}, commonFlags...),
			true, "please input a valid segment-index",
		},
		{
			"submit bad node",
			append([]string{
				cmdSubmit,
				spOperatorAddr, argTestBucketName, argTestObjectName, argTrue, "0",
				fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
			}, commonFlags...),
			true, errInvalidNodeURL,
		},
		{
			"submit happy path",
			append([]string{
				cmdSubmit,
				spOperatorAddr, argTestBucketName, argTestObjectName, "false", "3",
			}, commonFlags...),
			false, "",
		},
		{
			"attest bad challenge-id",
			append([]string{
				cmdAttest,
				argNotANumber, "1", spOperatorAddr, "0", "", "1", validSig,
			}, commonFlags...),
			true, "please input a valid challenge-id",
		},
		{
			"attest bad sp-operator-address",
			append([]string{
				cmdAttest,
				"1", "1", argNotAnAddress, "0", "", "1", validSig,
			}, commonFlags...),
			true, "please input a valid sp-operator-address",
		},
		{
			"attest bad vote-result",
			append([]string{
				cmdAttest,
				"1", "1", spOperatorAddr, "5", "", "1", validSig,
			}, commonFlags...),
			true, "please input a valid vote-result",
		},
		{
			"attest bad challenger-address",
			append([]string{
				cmdAttest,
				"1", "1", spOperatorAddr, "0", argNotAnAddress, "1", validSig,
			}, commonFlags...),
			true, "please input a valid challenger-address",
		},
		{
			"attest bad vote-validator-set entry",
			append([]string{
				cmdAttest,
				"1", "1", spOperatorAddr, "0", "", "1,x", validSig,
			}, commonFlags...),
			true, "please input a valid vote-validator-set",
		},
		{
			"attest bad vote-agg-signature",
			append([]string{
				cmdAttest,
				"1", "1", spOperatorAddr, "0", "", "1", "not-hex",
			}, commonFlags...),
			true, "please input a valid vote-agg-signature",
		},
		{
			"attest bad node",
			append([]string{
				cmdAttest,
				"1", "1", spOperatorAddr, "0", "", "1", validSig,
				fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
			}, commonFlags...),
			true, errInvalidNodeURL,
		},
		{
			// "ab" hex-decodes fine (a single 0xAB byte) so it clears the
			// CLI's own hex check, but fails MsgAttest.ValidateBasic's
			// BlsSignatureLength check, which the CLI never pre-validates.
			"attest vote-agg-signature wrong length",
			append([]string{
				cmdAttest,
				"1", "1", spOperatorAddr, "0", "", "1", "ab",
			}, commonFlags...),
			true, "length of aggregated signature is invalid",
		},
		{
			"attest happy path",
			append([]string{
				cmdAttest,
				"1", "1", spOperatorAddr, "0", "", "1,2", validSig,
			}, commonFlags...),
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			out, err := clitestutil.ExecTestCLICmd(s.clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErrMsg)
			} else {
				s.Require().NoError(err)
				resp := &sdk.TxResponse{}
				s.Require().NoError(s.clientCtx.Codec.UnmarshalJSON(out.Bytes(), resp), out.String())
			}
		})
	}
}
