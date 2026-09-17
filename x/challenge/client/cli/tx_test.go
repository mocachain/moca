package cli_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
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
	// encoding.MakeConfig() registers only the SDK/moca "std" interfaces, so
	// its InterfaceRegistry otherwise can't resolve a broadcast tx's
	// "/moca.challenge.MsgAttest" (or MsgSubmit) Any back to a concrete Go
	// type -- the same gap x/sp/client/cli/tx_test.go's spProposalClientCtx
	// works around for proposal decoding. Registering here, once, lets
	// TestCmdAttestMsgFields/TestCmdSubmitMsgFields decode a broadcast
	// message and assert on its fields directly.
	types.RegisterInterfaces(s.encCfg.InterfaceRegistry)
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

// commonTxFlags returns the flags shared by every CmdXxx happy-path case
// below: --from/--fees/--broadcast-mode=sync/--skip-confirmation drive
// tx.GenerateOrBroadcastTxCLI all the way through signing and
// MockCometRPC.BroadcastTxSync (which always answers Code: 0) without a live
// node, and --output=json lets the result be decoded back into an
// sdk.TxResponse. Mirrors x/sp/client/cli/tx_test.go's commonTxFlags.
func (s *CLITestSuite) commonTxFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", flags.FlagFrom, s.clientCtx.GetFromAddress().String()),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin("amoca", math.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}
}

// recordingRPC wraps clitestutil.MockCometRPC and records the raw bytes of
// the last transaction it broadcasts. The mock's canned response ignores the
// broadcast request entirely, so asserting only that a command exits without
// error can never prove an argument landed in the right message field
// instead of a neighboring one (e.g. two positional args silently swapped);
// decoding what was actually broadcast is the only way to prove that.
type recordingRPC struct {
	clitestutil.MockCometRPC
	lastTx cmttypes.Tx
}

func (r *recordingRPC) BroadcastTxSync(ctx context.Context, tx cmttypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	r.lastTx = tx
	return r.MockCometRPC.BroadcastTxSync(ctx, tx)
}

// recordingTxClientCtx returns a client.Context identical to s.baseCtx
// except that its RPC client is a *recordingRPC, letting a test decode the
// exact message a command under test broadcast.
func (s *CLITestSuite) recordingTxClientCtx() (client.Context, *recordingRPC) {
	rpc := &recordingRPC{MockCometRPC: clitestutil.NewMockCometRPC(abci.ResponseQuery{})}
	return s.baseCtx.WithClient(rpc), rpc
}

// decodedMsg runs cmd/args against a fresh recordingTxClientCtx and returns
// the single sdk.Msg the command broadcast, for field-by-field inspection.
func (s *CLITestSuite) decodedMsg(args []string) sdk.Msg {
	ctx, rpc := s.recordingTxClientCtx()
	out, err := clitestutil.ExecTestCLICmd(ctx, cli.GetTxCmd(), args)
	s.Require().NoError(err, out.String())

	decodedTx, err := s.encCfg.TxConfig.TxDecoder()(rpc.lastTx)
	s.Require().NoError(err)

	msgs := decodedTx.GetMsgs()
	s.Require().Len(msgs, 1)
	return msgs[0]
}

// TestCmdAttestMsgFields proves every attest positional argument lands in
// its own MsgAttest field rather than a neighboring one. Every value below
// is distinct from every other -- including sp-operator-address and
// challenger-address, both real non-empty addresses -- so a mutation that
// swaps two same-typed argument positions (challenge-id/object-id,
// sp-operator-address/challenger-address, or reusing args[0] for both ids)
// changes the decoded message instead of merely relabeling which value goes
// where, which the "1"/"1" ids used elsewhere in this table can't catch.
func (s *CLITestSuite) TestCmdAttestMsgFields() {
	spOperatorAddr := sample.RandAccAddressHex()
	challengerAddr := sample.RandAccAddressHex()
	wantSig := strings.Repeat("cd", types.BlsSignatureLength)
	wantSigBytes, err := hex.DecodeString(wantSig)
	s.Require().NoError(err)

	args := append([]string{
		cmdAttest,
		argChallengeID, argObjectID, spOperatorAddr, "1", challengerAddr, "21,22", wantSig,
	}, s.commonTxFlags()...)

	msg, ok := s.decodedMsg(args).(*types.MsgAttest)
	s.Require().True(ok)

	wantChallengeID, err := strconv.ParseUint(argChallengeID, 10, 64)
	s.Require().NoError(err)

	s.Require().Equal(wantChallengeID, msg.ChallengeId)
	s.Require().True(math.NewUintFromString(argObjectID).Equal(msg.ObjectId))
	s.Require().Equal(spOperatorAddr, msg.SpOperatorAddress)
	s.Require().Equal(types.CHALLENGE_SUCCEED, msg.VoteResult)
	s.Require().Equal(challengerAddr, msg.ChallengerAddress)
	s.Require().Equal([]uint64{21, 22}, msg.VoteValidatorSet)
	s.Require().Equal(wantSigBytes, msg.VoteAggSignature)
}

// TestCmdSubmitMsgFields proves every submit positional argument lands in
// its own MsgSubmit field. bucket-name and object-name are both plain
// strings, so the CLI's own validation can't tell a swap between them
// apart; and at the NewMsgSubmit call site the tx signer
// (clientCtx.GetFromAddress(), the "challenger" field) and the
// sp-operator-address argument are both sdk.AccAddress-typed, so a swap
// there type-checks too. Every value here is therefore distinct.
func (s *CLITestSuite) TestCmdSubmitMsgFields() {
	spOperatorAddr := sample.RandAccAddressHex()

	args := append([]string{
		cmdSubmit,
		spOperatorAddr, argTestBucketName, argTestObjectName, argFalse, "5",
	}, s.commonTxFlags()...)

	msg, ok := s.decodedMsg(args).(*types.MsgSubmit)
	s.Require().True(ok)

	s.Require().Equal(s.clientCtx.GetFromAddress().String(), msg.Challenger)
	s.Require().Equal(spOperatorAddr, msg.SpOperatorAddress)
	s.Require().Equal(argTestBucketName, msg.BucketName)
	s.Require().Equal(argTestObjectName, msg.ObjectName)
	s.Require().False(msg.RandomIndex)
	s.Require().Equal(uint32(5), msg.SegmentIndex)
}

// TestTxCmd drives CmdSubmit and CmdAttest far enough to prove their argument
// handling and, on a well-formed row, all the way through a successful
// broadcast: the mocked CometBFT RPC (clitestutil.NewMockCometRPC) answers
// BroadcastTxSync with a canned OK response regardless of content, and the
// keyring holds a real key, so tx.GenerateOrBroadcastTxCLI needs no live
// node to complete (same recipe as x/virtualgroup/client/cli/tx_test.go).
func (s *CLITestSuite) TestTxCmd() {
	commonFlags := s.commonTxFlags()

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
				spOperatorAddr, argTestBucketName, argTestObjectName, argFalse, argNotANumber,
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
				spOperatorAddr, argTestBucketName, argTestObjectName, argFalse, "3",
			}, commonFlags...),
			false, "",
		},
		{
			"attest bad challenge-id",
			append([]string{
				cmdAttest,
				argNotANumber, argObjectID, spOperatorAddr, "0", "", "1", validSig,
			}, commonFlags...),
			true, "please input a valid challenge-id",
		},
		{
			"attest bad sp-operator-address",
			append([]string{
				cmdAttest,
				argChallengeID, argObjectID, argNotAnAddress, "0", "", "1", validSig,
			}, commonFlags...),
			true, "please input a valid sp-operator-address",
		},
		{
			"attest bad vote-result",
			append([]string{
				cmdAttest,
				argChallengeID, argObjectID, spOperatorAddr, "5", "", "1", validSig,
			}, commonFlags...),
			true, "please input a valid vote-result",
		},
		{
			"attest bad challenger-address",
			append([]string{
				cmdAttest,
				argChallengeID, argObjectID, spOperatorAddr, "0", argNotAnAddress, "1", validSig,
			}, commonFlags...),
			true, "please input a valid challenger-address",
		},
		{
			"attest bad vote-validator-set entry",
			append([]string{
				cmdAttest,
				argChallengeID, argObjectID, spOperatorAddr, "0", "", "1,x", validSig,
			}, commonFlags...),
			true, "please input a valid vote-validator-set",
		},
		{
			"attest bad vote-agg-signature",
			append([]string{
				cmdAttest,
				argChallengeID, argObjectID, spOperatorAddr, "0", "", "1", "not-hex",
			}, commonFlags...),
			true, "please input a valid vote-agg-signature",
		},
		{
			"attest bad node",
			append([]string{
				cmdAttest,
				argChallengeID, argObjectID, spOperatorAddr, "0", "", "1", validSig,
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
				argChallengeID, argObjectID, spOperatorAddr, "0", "", "1", "ab",
			}, commonFlags...),
			true, "length of aggregated signature is invalid",
		},
		{
			"attest happy path",
			append([]string{
				cmdAttest,
				argChallengeID, argObjectID, spOperatorAddr, "0", "", "1,2", validSig,
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
