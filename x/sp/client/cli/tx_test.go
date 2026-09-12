package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/suite"

	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/sdk/client/test"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/client/cli"
	"github.com/mocachain/moca/v2/x/sp/types"
)

// badNodeURL/errInvalidNodeURL trigger client.GetClientTxContext's
// NewClientFromNode branch deterministically: a control character fails
// url.Parse before any keyring/EVM access is attempted, so every "bad node"
// case below fails the same documented way without a live node.
const (
	badNodeURL        = "://bad host \x00"
	errInvalidNodeURL = "invalid control character in URL"
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

// commonTxFlags returns the flags shared by every CmdXxx happy-path case
// below: --from/--fees/--broadcast-mode=sync/--skip-confirmation drive
// tx.GenerateOrBroadcastTxCLI all the way through signing and
// MockCometRPC.BroadcastTxSync (which always answers Code: 0) without a live
// node, and --output=json lets the result be decoded back into an
// sdk.TxResponse. Mirrors x/virtualgroup/client/cli/tx_test.go's commonFlags.
func (s *CLITestSuite) commonTxFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", flags.FlagFrom, s.clientCtx.GetFromAddress().String()),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin("amoca", math.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}
}

// requireTxSuccess asserts that cmd/args broadcasts successfully against ctx
// and that the output decodes as a real sdk.TxResponse, proving the RunE
// reached its final tx.GenerateOrBroadcastTxCLI call rather than merely not
// panicking.
func (s *CLITestSuite) requireTxSuccess(ctx client.Context, cmd *cobra.Command, args []string) {
	out, err := clitestutil.ExecTestCLICmd(ctx, cmd, args)
	s.Require().NoError(err, out.String())
	resp := &sdk.TxResponse{}
	s.Require().NoError(ctx.Codec.UnmarshalJSON(out.Bytes(), resp), out.String())
}

// requireTxError asserts that cmd/args fails against ctx with an error
// containing wantErr.
func (s *CLITestSuite) requireTxError(ctx client.Context, cmd *cobra.Command, args []string, wantErr string) {
	_, err := clitestutil.ExecTestCLICmd(ctx, cmd, args)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), wantErr)
}

// spProposalClientCtx returns a client.Context whose codec additionally
// registers x/sp's message types as Any implementations. The suite's shared
// s.clientCtx is built from encoding.MakeConfig() (same as every sibling
// module's CLI suite), which registers only the SDK/moca "std" interfaces;
// govcli.ParseSubmitProposal can never resolve a
// "/moca.sp.MsgCreateStorageProvider" type URL through it otherwise.
func (s *CLITestSuite) spProposalClientCtx() client.Context {
	cfg := encoding.MakeConfig()
	types.RegisterInterfaces(cfg.InterfaceRegistry)
	return s.clientCtx.WithCodec(cfg.Codec)
}

// paramsClientCtx returns a client.Context whose mocked ABCI query answers
// every query with a QueryParamsResponse carrying deposit denom "amoca",
// needed to drive CmdGrantDepositAuthorization's denom-match / spend-limit
// branches without a live query server.
func (s *CLITestSuite) paramsClientCtx() client.Context {
	bz, err := s.encCfg.Codec.Marshal(&types.QueryParamsResponse{Params: types.Params{DepositDenom: "amoca"}})
	s.Require().NoError(err)
	return s.baseCtx.WithClient(clitestutil.NewMockCometRPC(abci.ResponseQuery{Value: bz}))
}

// queryErrClientCtx returns a client.Context whose mocked ABCI query always
// answers with a non-OK code, used to reach
// CmdGrantDepositAuthorization's queryClient.Params error branch: CometBFT's
// ABCIQueryWithOptions never itself errors in these tests, but client.Context
// treats a non-zero response Code as a query failure.
func (s *CLITestSuite) queryErrClientCtx() client.Context {
	return s.baseCtx.WithClient(clitestutil.NewMockCometRPC(abci.ResponseQuery{Code: 1, Log: "boom"}))
}

// writeProposalFile writes a gov submit-proposal JSON document wrapping
// messagesJSON (one or more comma-separated Any-typed message objects) to a
// temp file and returns its path, for CmdCreateStorageProvider's
// govcli.ParseSubmitProposal argument.
func (s *CLITestSuite) writeProposalFile(messagesJSON string) string {
	body := fmt.Sprintf(`{
		"messages": [%s],
		"title": "create sp for test",
		"summary": "test",
		"metadata": "bW9jYQ==",
		"deposit": "1000000000000000000amoca"
	}`, messagesJSON)
	path := filepath.Join(s.T().TempDir(), "proposal.json")
	s.Require().NoError(os.WriteFile(path, []byte(body), 0o600))
	return path
}

// spCreateProposalMsgJSON renders one MsgCreateStorageProvider as the
// Any-typed JSON govcli.ParseSubmitProposal expects inside a proposal's
// "messages" array. moniker "" produces a zero-value Description, used to
// hit MsgCreateStorageProvider.ValidateBasic's empty-description branch; any
// other value builds an otherwise fully valid message.
func spCreateProposalMsgJSON(fundingAddr, moniker, blsKey, blsProof string) string {
	other := sample.RandAccAddressHex()
	return fmt.Sprintf(`{
		"@type": "/moca.sp.MsgCreateStorageProvider",
		"creator": %[1]q,
		"description": {"moniker": %[2]q},
		"sp_address": %[1]q,
		"funding_address": %[3]q,
		"seal_address": %[1]q,
		"approval_address": %[1]q,
		"gc_address": %[1]q,
		"maintenance_address": %[1]q,
		"endpoint": "https://sp0.moca.io",
		"deposit": {"denom": "amoca", "amount": "1000000000000000000000"},
		"read_price": "0.108",
		"store_price": "0.016",
		"free_read_quota": 1073741824,
		"bls_key": %[4]q,
		"bls_proof": %[5]q
	}`, other, moniker, fundingAddr, blsKey, blsProof)
}

// spDepositProposalMsgJSON renders one MsgDeposit as an Any-typed JSON
// message, used to exercise CmdCreateStorageProvider's wrong-message-type and
// too-many-messages branches (a MsgDeposit is a validly resolvable sp message
// that is not a MsgCreateStorageProvider).
func spDepositProposalMsgJSON(creator, spAddr string) string {
	return fmt.Sprintf(
		`{"@type": "/moca.sp.MsgDeposit", "creator": %q, "sp_address": %q, "deposit": {"denom": "amoca", "amount": "1"}}`,
		creator, spAddr,
	)
}

func (s *CLITestSuite) TestGetTxCmd() {
	cmd := cli.GetTxCmd()
	s.Require().Equal(types.ModuleName, cmd.Name())
	s.Require().Len(cmd.Commands(), 6)
}

func (s *CLITestSuite) TestCmdEditStorageProvider() {
	validAddr := sample.RandAccAddressHex()
	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectErrMsg string
	}{
		{
			"invalid sp address",
			append([]string{"edit-storage-provider", "not-a-valid-address"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid seal address flag",
			append([]string{"edit-storage-provider", validAddr, "--" + cli.FlagSealAddress, "bad"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid approval address flag",
			append([]string{"edit-storage-provider", validAddr, "--" + cli.FlagApprovalAddress, "bad"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid gc address flag",
			append([]string{"edit-storage-provider", validAddr, "--" + cli.FlagGcAddress, "bad"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid maintenance address flag",
			append([]string{"edit-storage-provider", validAddr, "--" + cli.FlagMaintenanceAddress, "bad"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid bls pub key length",
			append([]string{"edit-storage-provider", validAddr, "--" + cli.FlagBlsPubKey, "abcd"}, s.commonTxFlags()...),
			true, "invalid bls pubkey",
		},
		{
			"bls proof missing",
			append([]string{"edit-storage-provider", validAddr, "--" + cli.FlagBlsPubKey, blsKey}, s.commonTxFlags()...),
			true, "bls proof is not provided",
		},
		{
			"bad node",
			[]string{"edit-storage-provider", validAddr, fmt.Sprintf("--%s=%s", flags.FlagNode, badNodeURL)},
			true, errInvalidNodeURL,
		},
		{
			"happy path with no optional fields",
			append([]string{"edit-storage-provider", validAddr}, s.commonTxFlags()...),
			false, "",
		},
		{
			"happy path with every optional field",
			append([]string{
				"edit-storage-provider", validAddr,
				"--" + cli.FlagSealAddress, validAddr,
				"--" + cli.FlagApprovalAddress, validAddr,
				"--" + cli.FlagGcAddress, validAddr,
				"--" + cli.FlagMaintenanceAddress, validAddr,
				"--" + cli.FlagBlsPubKey, blsKey,
				"--" + cli.FlagBlsProof, blsProof,
				"--" + cli.FlagEndpoint, "https://sp0.moca.io",
			}, s.commonTxFlags()...),
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			if tc.expectErr {
				s.requireTxError(s.clientCtx, cmd, tc.args, tc.expectErrMsg)
			} else {
				s.requireTxSuccess(s.clientCtx, cmd, tc.args)
			}
		})
	}
}

func (s *CLITestSuite) TestCmdDeposit() {
	spAddr := sample.RandAccAddressHex()
	fundAddr := sample.RandAccAddressHex()

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectErrMsg string
	}{
		{
			"bad node",
			[]string{"deposit", spAddr, fundAddr, "100amoca", fmt.Sprintf("--%s=%s", flags.FlagNode, badNodeURL)},
			true, errInvalidNodeURL,
		},
		{
			"invalid sp address",
			append([]string{"deposit", "not-a-valid-address", fundAddr, "100amoca"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid fund address",
			append([]string{"deposit", spAddr, "not-a-valid-address", "100amoca"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid coin string",
			append([]string{"deposit", spAddr, fundAddr, "not-a-coin"}, s.commonTxFlags()...),
			true, "invalid decimal coin expression",
		},
		{
			"zero deposit amount",
			append([]string{"deposit", spAddr, fundAddr, "0amoca"}, s.commonTxFlags()...),
			true, "invalid deposit amount",
		},
		{
			"happy path",
			append([]string{"deposit", spAddr, fundAddr, "100amoca"}, s.commonTxFlags()...),
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			if tc.expectErr {
				s.requireTxError(s.clientCtx, cmd, tc.args, tc.expectErrMsg)
			} else {
				s.requireTxSuccess(s.clientCtx, cmd, tc.args)
			}
		})
	}
}

func (s *CLITestSuite) TestCmdGrantDepositAuthorization() {
	grantee := sample.RandAccAddressHex()
	spAddr := sample.RandAccAddressHex()

	s.Run("bad node", func() {
		cmd := cli.GetTxCmd()
		args := []string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "100amoca",
			fmt.Sprintf("--%s=%s", flags.FlagNode, badNodeURL),
		}
		s.requireTxError(s.clientCtx, cmd, args, errInvalidNodeURL)
	})

	s.Run("invalid grantee address", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", "not-a-valid-address",
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "100amoca",
		}, s.commonTxFlags()...)
		s.requireTxError(s.clientCtx, cmd, args, "invalid address hex length")
	})

	s.Run("invalid SPAddress flag", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, "bad",
			"--" + cli.FlagSpendLimit, "100amoca",
		}, s.commonTxFlags()...)
		s.requireTxError(s.clientCtx, cmd, args, "invalid address hex length")
	})

	s.Run("invalid spend limit", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "not-a-coin",
		}, s.commonTxFlags()...)
		s.requireTxError(s.clientCtx, cmd, args, "invalid decimal coin expression")
	})

	s.Run("query error", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "100amoca",
		}, s.commonTxFlags()...)
		s.requireTxError(s.queryErrClientCtx(), cmd, args, "boom")
	})

	s.Run("denom mismatch", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "100abc",
		}, s.commonTxFlags()...)
		s.requireTxError(s.paramsClientCtx(), cmd, args, "invalid denom")
	})

	s.Run("zero spend limit", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "0amoca",
		}, s.commonTxFlags()...)
		s.requireTxError(s.paramsClientCtx(), cmd, args, "spend-limit should be greater than zero")
	})

	s.Run("happy path with no expiration", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "100amoca",
		}, s.commonTxFlags()...)
		s.requireTxSuccess(s.paramsClientCtx(), cmd, args)
	})

	s.Run("happy path with expiration", func() {
		cmd := cli.GetTxCmd()
		args := append([]string{
			"grant", grantee,
			"--" + cli.FlagSpAddress, spAddr,
			"--" + cli.FlagSpendLimit, "100amoca",
			"--" + cli.FlagExpiration, "4102444800",
		}, s.commonTxFlags()...)
		s.requireTxSuccess(s.paramsClientCtx(), cmd, args)
	})
}

func (s *CLITestSuite) TestCmdUpdateStorageProviderStatus() {
	spAddr := sample.RandAccAddressHex()

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectErrMsg string
	}{
		{
			"bad node",
			[]string{"update-status", spAddr, "STATUS_IN_SERVICE", fmt.Sprintf("--%s=%s", flags.FlagNode, badNodeURL)},
			true, errInvalidNodeURL,
		},
		{
			"invalid sp address",
			append([]string{"update-status", "not-a-valid-address", "STATUS_IN_SERVICE"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"unexpected status",
			append([]string{"update-status", spAddr, "STATUS_IN_JAILED"}, s.commonTxFlags()...),
			true, "is not expected",
		},
		{
			"maintenance without duration",
			append([]string{"update-status", spAddr, "STATUS_IN_MAINTENANCE"}, s.commonTxFlags()...),
			true, "maintenance duration need to be set",
		},
		{
			"happy path to in service",
			append([]string{"update-status", spAddr, "STATUS_IN_SERVICE"}, s.commonTxFlags()...),
			false, "",
		},
		{
			"happy path to in maintenance",
			append([]string{
				"update-status", spAddr, "STATUS_IN_MAINTENANCE",
				"--" + cli.FlagDuration, "21600",
			}, s.commonTxFlags()...),
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			if tc.expectErr {
				s.requireTxError(s.clientCtx, cmd, tc.args, tc.expectErrMsg)
			} else {
				s.requireTxSuccess(s.clientCtx, cmd, tc.args)
			}
		})
	}
}

func (s *CLITestSuite) TestCmdUpdateStorageProviderStoragePrice() {
	spAddr := sample.RandAccAddressHex()

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectErrMsg string
	}{
		{
			"bad node",
			[]string{"update-price", spAddr, "0.1", "0.01", "1024", fmt.Sprintf("--%s=%s", flags.FlagNode, badNodeURL)},
			true, errInvalidNodeURL,
		},
		{
			"invalid sp address",
			append([]string{"update-price", "not-a-valid-address", "0.1", "0.01", "1024"}, s.commonTxFlags()...),
			true, "invalid address hex length",
		},
		{
			"invalid read price",
			append([]string{"update-price", spAddr, "not-a-decimal", "0.01", "1024"}, s.commonTxFlags()...),
			true, "failed to set decimal string",
		},
		{
			"invalid store price",
			append([]string{"update-price", spAddr, "0.1", "not-a-decimal", "1024"}, s.commonTxFlags()...),
			true, "failed to set decimal string",
		},
		{
			"invalid free read quota",
			append([]string{"update-price", spAddr, "0.1", "0.01", "not-a-number"}, s.commonTxFlags()...),
			true, "invalid syntax",
		},
		{
			// "--" stops pflag from trying (and failing) to parse the
			// negative read price as an unknown shorthand flag.
			"negative read price",
			[]string{"update-price", spAddr, "--", "-0.1", "0.01", "1024"},
			true, "invalid read price",
		},
		{
			"negative store price",
			[]string{"update-price", spAddr, "0.1", "--", "-0.01", "1024"},
			true, "invalid store price",
		},
		{
			"happy path",
			append([]string{"update-price", spAddr, "0.1", "0.01", "1024"}, s.commonTxFlags()...),
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			if tc.expectErr {
				s.requireTxError(s.clientCtx, cmd, tc.args, tc.expectErrMsg)
			} else {
				s.requireTxSuccess(s.clientCtx, cmd, tc.args)
			}
		})
	}
}

func (s *CLITestSuite) TestCmdCreateStorageProvider() {
	fundingAddr := s.clientCtx.GetFromAddress().String()
	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	s.Run("bad node", func() {
		cmd := cli.GetTxCmd()
		path := s.writeProposalFile(spCreateProposalMsgJSON(fundingAddr, "sp0", blsKey, blsProof))
		args := []string{
			"create-storage-provider", path,
			// CmdCreateStorageProvider (unlike the other CmdXxx commands in
			// this file) marks --from required, so cobra's own flag
			// validation must be satisfied before RunE ever runs and reaches
			// client.GetClientTxContext.
			fmt.Sprintf("--%s=%s", flags.FlagFrom, s.clientCtx.GetFromAddress().String()),
			fmt.Sprintf("--%s=%s", flags.FlagNode, badNodeURL),
		}
		s.requireTxError(s.clientCtx, cmd, args, errInvalidNodeURL)
	})

	s.Run("proposal file not found", func() {
		cmd := cli.GetTxCmd()
		missing := filepath.Join(s.T().TempDir(), "missing.json")
		args := append([]string{"create-storage-provider", missing}, s.commonTxFlags()...)
		s.requireTxError(s.spProposalClientCtx(), cmd, args, "no such file or directory")
	})

	s.Run("too many messages", func() {
		cmd := cli.GetTxCmd()
		other := sample.RandAccAddressHex()
		msgs := spDepositProposalMsgJSON(other, other) + "," + spDepositProposalMsgJSON(other, other)
		path := s.writeProposalFile(msgs)
		args := append([]string{"create-storage-provider", path}, s.commonTxFlags()...)
		s.requireTxError(s.spProposalClientCtx(), cmd, args, "invalid message length")
	})

	s.Run("wrong message type", func() {
		cmd := cli.GetTxCmd()
		other := sample.RandAccAddressHex()
		path := s.writeProposalFile(spDepositProposalMsgJSON(other, other))
		args := append([]string{"create-storage-provider", path}, s.commonTxFlags()...)
		s.requireTxError(s.spProposalClientCtx(), cmd, args, "invalid create storage provider message")
	})

	s.Run("message fails validate basic", func() {
		cmd := cli.GetTxCmd()
		path := s.writeProposalFile(spCreateProposalMsgJSON(fundingAddr, "", blsKey, blsProof))
		args := append([]string{"create-storage-provider", path}, s.commonTxFlags()...)
		s.requireTxError(s.spProposalClientCtx(), cmd, args, "invalid create storage provider message")
	})

	s.Run("funding address is not from address", func() {
		cmd := cli.GetTxCmd()
		path := s.writeProposalFile(spCreateProposalMsgJSON(sample.RandAccAddressHex(), "sp0", blsKey, blsProof))
		args := append([]string{"create-storage-provider", path}, s.commonTxFlags()...)
		s.requireTxError(s.spProposalClientCtx(), cmd, args, "the from address should be the funding address")
	})

	s.Run("happy path", func() {
		cmd := cli.GetTxCmd()
		path := s.writeProposalFile(spCreateProposalMsgJSON(fundingAddr, "sp0", blsKey, blsProof))
		args := append([]string{"create-storage-provider", path}, s.commonTxFlags()...)
		s.requireTxSuccess(s.spProposalClientCtx(), cmd, args)
	})
}

func (s *CLITestSuite) TestCreateStorageProviderMsgFlagSet() {
	fs, defaultsDesc := cli.CreateStorageProviderMsgFlagSet("203.0.113.5")

	s.Require().Empty(defaultsDesc)
	s.Require().NotNil(fs.Lookup(cli.FlagIP))
	s.Require().Equal("203.0.113.5", fs.Lookup(cli.FlagIP).DefValue)
	s.Require().Equal("100", fs.Lookup(cli.FlagReadPrice).DefValue)
	s.Require().Equal("10000", fs.Lookup(cli.FlagStorePrice).DefValue)
	s.Require().Equal("100000000", fs.Lookup(cli.FlagFreeReadQuota).DefValue)
	s.Require().NotNil(fs.Lookup(cli.FlagCreator))
	s.Require().NotNil(fs.Lookup(cli.FlagBlsPubKey))
	s.Require().NotNil(fs.Lookup(cli.FlagMaintenanceAddress))
}

// newValidCreateSPFlagSet returns a flag set for
// PrepareConfigForTxCreateStorageProvider with every address/bls flag
// pre-set to a valid value; individual tests below override one flag to an
// invalid value to isolate that branch, or leave everything as-is for the
// happy path.
func (s *CLITestSuite) newValidCreateSPFlagSet(addr, blsKey string) *pflag.FlagSet {
	fs, _ := cli.CreateStorageProviderMsgFlagSet("203.0.113.5")
	s.Require().NoError(fs.Set(cli.FlagCreator, addr))
	s.Require().NoError(fs.Set(cli.FlagOperatorAddress, addr))
	s.Require().NoError(fs.Set(cli.FlagFundingAddress, addr))
	s.Require().NoError(fs.Set(cli.FlagSealAddress, addr))
	s.Require().NoError(fs.Set(cli.FlagBlsPubKey, blsKey))
	s.Require().NoError(fs.Set(cli.FlagApprovalAddress, addr))
	s.Require().NoError(fs.Set(cli.FlagGcAddress, addr))
	s.Require().NoError(fs.Set(cli.FlagMaintenanceAddress, addr))
	return fs
}

func (s *CLITestSuite) TestPrepareConfigForTxCreateStorageProvider() {
	addr := sample.RandAccAddressHex()
	blsKey := sample.RandBlsPubKeyHex()

	// Every one of these address flags is parsed with sdk.AccAddressFromHexUnsafe
	// right after its (never-failing) flagSet.GetString call; leaving one unset
	// (empty string, the flag's registered default) reaches that parse's own
	// error return, one field at a time.
	addressFlags := []string{
		cli.FlagCreator,
		cli.FlagOperatorAddress,
		cli.FlagFundingAddress,
		cli.FlagSealAddress,
		cli.FlagApprovalAddress,
		cli.FlagGcAddress,
		cli.FlagMaintenanceAddress,
	}
	for _, flagName := range addressFlags {
		flagName := flagName

		s.Run("empty "+flagName, func() {
			fs := s.newValidCreateSPFlagSet(addr, blsKey)
			s.Require().NoError(fs.Set(flagName, ""))
			_, err := cli.PrepareConfigForTxCreateStorageProvider(fs)
			s.Require().ErrorContains(err, "address")
		})
	}

	s.Run("invalid bls pub key length", func() {
		fs := s.newValidCreateSPFlagSet(addr, "abcd")
		_, err := cli.PrepareConfigForTxCreateStorageProvider(fs)
		s.Require().ErrorContains(err, "invalid bls pubkey")
	})

	s.Run("invalid read price", func() {
		fs := s.newValidCreateSPFlagSet(addr, blsKey)
		s.Require().NoError(fs.Set(cli.FlagReadPrice, "not-a-decimal"))
		_, err := cli.PrepareConfigForTxCreateStorageProvider(fs)
		s.Require().ErrorContains(err, "failed to set decimal string")
	})

	s.Run("invalid store price", func() {
		fs := s.newValidCreateSPFlagSet(addr, blsKey)
		s.Require().NoError(fs.Set(cli.FlagStorePrice, "not-a-decimal"))
		_, err := cli.PrepareConfigForTxCreateStorageProvider(fs)
		s.Require().ErrorContains(err, "failed to set decimal string")
	})

	s.Run("happy path", func() {
		fs := s.newValidCreateSPFlagSet(addr, blsKey)
		s.Require().NoError(fs.Set(cli.FlagMoniker, "sp0"))
		s.Require().NoError(fs.Set(cli.FlagEndpoint, "https://sp0.moca.io"))

		cfg, err := cli.PrepareConfigForTxCreateStorageProvider(fs)
		s.Require().NoError(err)
		s.Require().Equal(addr, cfg.Creator.String())
		s.Require().Equal(addr, cfg.SpAddress.String())
		s.Require().Equal(addr, cfg.FundingAddress.String())
		s.Require().Equal(addr, cfg.SealAddress.String())
		s.Require().Equal(addr, cfg.ApprovalAddress.String())
		s.Require().Equal(addr, cfg.GcAddress.String())
		s.Require().Equal(addr, cfg.MaintenanceAddress.String())
		s.Require().Equal(blsKey, cfg.BlsPubKey)
		s.Require().Equal("sp0", cfg.Moniker)
		s.Require().Equal("https://sp0.moca.io", cfg.Endpoint)
		s.Require().Equal(uint64(100000000), cfg.FreeReadQuota)
		s.Require().True(cfg.ReadPrice.Equal(math.LegacyMustNewDecFromStr("100")))
		s.Require().True(cfg.StorePrice.Equal(math.LegacyMustNewDecFromStr("10000")))
	})
}

func (s *CLITestSuite) TestBuildCreateStorageProviderMsg() {
	addr := sample.RandAccAddressHex()
	accAddr, err := sdk.AccAddressFromHexUnsafe(addr)
	s.Require().NoError(err)
	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()

	baseConfig := cli.TxCreateStorageProviderConfig{
		Creator:            accAddr,
		SpAddress:          accAddr,
		FundingAddress:     accAddr,
		SealAddress:        accAddr,
		ApprovalAddress:    accAddr,
		GcAddress:          accAddr,
		MaintenanceAddress: accAddr,
		Moniker:            "sp0",
		Endpoint:           "https://sp0.moca.io",
		BlsPubKey:          blsKey,
		BlsProof:           blsProof,
		ReadPrice:          math.LegacyMustNewDecFromStr("0.1"),
		StorePrice:         math.LegacyMustNewDecFromStr("0.01"),
		FreeReadQuota:      1024,
	}

	s.Run("invalid deposit coin", func() {
		config := baseConfig
		config.Deposit = "not-a-coin"
		_, _, err := cli.BuildCreateStorageProviderMsg(config, tx.Factory{})
		s.Require().ErrorContains(err, "invalid decimal coin expression")
	})

	s.Run("happy path", func() {
		config := baseConfig
		config.Deposit = "100amoca"
		_, msg, err := cli.BuildCreateStorageProviderMsg(config, tx.Factory{})
		s.Require().NoError(err)
		spMsg, ok := msg.(*types.MsgCreateStorageProvider)
		s.Require().True(ok)
		s.Require().Equal(addr, spMsg.SpAddress)
		s.Require().Equal("sp0", spMsg.Description.Moniker)
		s.Require().Equal("amoca", spMsg.Deposit.Denom)
		s.Require().Equal(blsKey, spMsg.BlsKey)
	})
}
