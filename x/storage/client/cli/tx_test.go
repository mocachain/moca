package cli_test

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/suite"

	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/sdk/client/test"
	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
	"github.com/mocachain/moca/v2/x/storage/client/cli"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
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

func (s *CLITestSuite) TestUpdateGroupMember_SliceBuild_Aligned_NoPanic() {
	cmd := cli.GetTxCmd()

	// args: update-group-member [group-name] [member-to-add] [member-expiration-to-add] [member-to-delete] --privatekey xxx
	groupName := "test-group"
	membersToAdd := "0x1111111111111111111111111111111111111111,0x2222222222222222222222222222222222222222"
	expirations := "0,0" // 0 means default MaxTimeStamp will be used in precompile
	membersToDelete := ""

	args := []string{
		"update-group-member",
		groupName,
		membersToAdd,
		expirations,
		membersToDelete,
		"--privatekey", "", // trigger downstream error without affecting slice construction
	}

	s.Require().NotPanics(func() {
		_, err := clitestutil.ExecTestCLICmd(s.clientCtx, cmd, args)
		// Inputs are aligned and valid, so local validation passes and this must fail at the
		// private-key gate specifically, not on some other unrelated error.
		s.Require().ErrorContains(err, gateErrSubstring)
	})
}

func (s *CLITestSuite) TestRenewGroupMember_SliceBuild_Aligned_NoPanic() {
	cmd := cli.GetTxCmd()

	// args: renew-group-member [group-name] [member] [member-expiration] --privatekey xxx
	groupName := "test-group"
	members := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	expirations := "0,0"

	args := []string{
		"renew-group-member",
		groupName,
		members,
		expirations,
		"--privatekey", "",
	}

	s.Require().NotPanics(func() {
		_, err := clitestutil.ExecTestCLICmd(s.clientCtx, cmd, args)
		// As above: aligned, valid inputs pass local validation, so this must reach the gate.
		s.Require().ErrorContains(err, gateErrSubstring)
	})
}

// execExpectError runs cmd with args through the same MockCometRPC client context used by every
// other CLI test in this suite and asserts it returns an error without panicking. Every CmdXxx tx
// command in this package does its own local flag/arg parsing first and only errors out for real
// once it reaches keys.NewPrivateKeyManager (an empty --privatekey always fails there) or one of
// its own local validation checks; none of these tests ever dial the network.
func (s *CLITestSuite) execExpectError(cmd *cobra.Command, args []string) error {
	var err error
	s.Require().NotPanics(func() {
		_, err = clitestutil.ExecTestCLICmd(s.clientCtx, cmd, args)
	})
	s.Require().Error(err)
	return err
}

// gateErrSubstring is the distinctive substring of the error keys.NewPrivateKeyManager returns
// for every empty --privatekey flag in this file: hex-decoding an empty string yields zero
// bytes, which fails its 32-byte length check. A "reaches private key gate" case asserts this
// substring to prove it got past its own local validation; every other case asserts its own
// validation error instead, which proves the opposite: that it never reached this far.
const gateErrSubstring = "len of Keybytes is not equal to 32"

// assertValidationErr asserts that err is the specific failure a case names: a sentinel via
// errIs, or a distinctive substring via errContains (exactly one must be set). Asserting a
// specific error instead of just "some error" is what makes a case fail if the validation it
// covers is removed: execution would then fall through to the private-key gate below it and
// return gateErrSubstring instead, matching neither the sentinel nor the substring.
func (s *CLITestSuite) assertValidationErr(err error, errIs error, errContains string) {
	s.T().Helper()
	switch {
	case errIs != nil:
		s.Require().ErrorIs(err, errIs)
	case errContains != "":
		s.Require().ErrorContains(err, errContains)
	default:
		s.T().Fatal("test case must set errIs or errContains")
	}
}

func (s *CLITestSuite) TestCmdCreateBucket() {
	testCases := []struct {
		name        string
		args        []string
		errIs       error
		errContains string
	}{
		{
			name:        "reaches private key gate",
			args:        []string{"create-bucket", "test-bucket", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
		{
			name:  "invalid visibility",
			args:  []string{"create-bucket", "test-bucket", "--visibility", "BOGUS_VISIBILITY", "--privatekey", ""},
			errIs: gnfderrors.ErrInvalidVisibilityType,
		},
		{
			name:  "invalid payment account",
			args:  []string{"create-bucket", "test-bucket", "--payment-account", "not-a-registered-key", "--privatekey", ""},
			errIs: sdkerrors.ErrKeyNotFound,
		},
		{
			name:  "invalid primary sp",
			args:  []string{"create-bucket", "test-bucket", "--primary-sp", "not-a-registered-key", "--privatekey", ""},
			errIs: sdkerrors.ErrKeyNotFound,
		},
		{
			name:        "invalid approve signature",
			args:        []string{"create-bucket", "test-bucket", "--approve-signature", "zz", "--privatekey", ""},
			errContains: "invalid byte",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.assertValidationErr(err, tc.errIs, tc.errContains)
		})
	}
}

func (s *CLITestSuite) TestCmdDeleteBucket() {
	cmd := cli.GetTxCmd()
	args := []string{"delete-bucket", "test-bucket", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdUpdateBucketInfo() {
	testCases := []struct {
		name        string
		args        []string
		errIs       error
		errContains string
	}{
		{
			name:        "reaches private key gate",
			args:        []string{"update-bucket-info", "test-bucket", "100", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
		{
			name:        "invalid charged read quota",
			args:        []string{"update-bucket-info", "test-bucket", "not-a-number", "--privatekey", ""},
			errContains: "invalid syntax",
		},
		{
			name:  "invalid visibility",
			args:  []string{"update-bucket-info", "test-bucket", "100", "--visibility", "BOGUS_VISIBILITY", "--privatekey", ""},
			errIs: gnfderrors.ErrInvalidVisibilityType,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.assertValidationErr(err, tc.errIs, tc.errContains)
		})
	}
}

func (s *CLITestSuite) TestCmdDiscontinueBucket() {
	cmd := cli.GetTxCmd()
	args := []string{"discontinue-bucket", "test-bucket", "test reason", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

// TestCmdSetBucketFlowRateLimit exercises CmdSetBucketFlowRateLimit's local validation of the
// payment-account / bucket-owner addresses and the flow-rate-limit integer, then the
// private-key gate all the CLI tx commands share. None of these cases ever reach the network.
func (s *CLITestSuite) TestCmdSetBucketFlowRateLimit() {
	validPaymentAcc := sample.RandAccAddressHex()
	validBucketOwner := sample.RandAccAddressHex()

	testCases := []struct {
		name          string
		bucketName    string
		paymentAcc    string
		bucketOwner   string
		flowRateLimit string
		errContains   string
	}{
		{
			name:          "reaches private key gate",
			bucketName:    "test-bucket",
			paymentAcc:    validPaymentAcc,
			bucketOwner:   validBucketOwner,
			flowRateLimit: "1000",
			errContains:   gateErrSubstring,
		},
		{
			name:          "invalid payment account",
			bucketName:    "test-bucket",
			paymentAcc:    "not-a-valid-address",
			bucketOwner:   validBucketOwner,
			flowRateLimit: "1000",
			errContains:   "invalid address hex length",
		},
		{
			name:          "invalid bucket owner",
			bucketName:    "test-bucket",
			paymentAcc:    validPaymentAcc,
			bucketOwner:   "not-a-valid-address",
			flowRateLimit: "1000",
			errContains:   "invalid address hex length",
		},
		{
			name:          "invalid flow rate limit",
			bucketName:    "test-bucket",
			paymentAcc:    validPaymentAcc,
			bucketOwner:   validBucketOwner,
			flowRateLimit: "not-a-number",
			errContains:   "invalid flow-rate-limit",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()

			args := []string{
				"set-bucket-flow-rate-limit",
				tc.bucketName,
				tc.paymentAcc,
				tc.bucketOwner,
				tc.flowRateLimit,
				"--privatekey", "",
			}

			err := s.execExpectError(cmd, args)
			s.Require().ErrorContains(err, tc.errContains)
		})
	}
}

func (s *CLITestSuite) TestCmdMigrateBucket() {
	testCases := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "reaches private key gate",
			args:        []string{"migrate-bucket", "test-bucket", "1", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
		{
			name:        "invalid dest primary sp id",
			args:        []string{"migrate-bucket", "test-bucket", "not-a-number", "--privatekey", ""},
			errContains: "invalid syntax",
		},
		{
			name:        "invalid approve signature",
			args:        []string{"migrate-bucket", "test-bucket", "1", "--approve-signature", "zz", "--privatekey", ""},
			errContains: "invalid byte",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.Require().ErrorContains(err, tc.errContains)
		})
	}
}

// TestCmdCancelMigrateBucket exercises CmdCancelMigrateBucket up to the private-key gate:
// local arg parsing and client-context setup all succeed, and the command only fails once it
// tries to build a key manager from an empty private key (before any network dial happens).
func (s *CLITestSuite) TestCmdCancelMigrateBucket() {
	cmd := cli.GetTxCmd()

	args := []string{
		"cancel-migrate-bucket",
		"test-bucket",
		"--privatekey", "",
	}

	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdCreateGroup() {
	cmd := cli.GetTxCmd()
	args := []string{"create-group", "test-group", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdDeleteGroup() {
	cmd := cli.GetTxCmd()
	args := []string{"delete-group", "test-group", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdLeaveGroup() {
	testCases := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "reaches private key gate",
			args:        []string{"leave-group", sample.RandAccAddressHex(), "test-group", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
		{
			name:        "invalid group owner",
			args:        []string{"leave-group", "not-a-valid-address", "test-group", "--privatekey", ""},
			errContains: "invalid address hex length",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.Require().ErrorContains(err, tc.errContains)
		})
	}
}

// TestUpdateGroupMember_ValidationBranches covers the CmdUpdateGroupMember slice-alignment and
// per-entry parsing branches that TestUpdateGroupMember_SliceBuild_Aligned_NoPanic (above) does
// not exercise: the length-mismatch guard, an invalid hex entry in the add/delete lists, an
// invalid expiration timestamp, and the "skip empty entry" / "default expiration to zero" paths.
func (s *CLITestSuite) TestUpdateGroupMember_ValidationBranches() {
	memberToAdd := sample.RandAccAddressHex()
	memberToDelete := sample.RandAccAddressHex()

	testCases := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "mismatched add and expiration length",
			args:        []string{"update-group-member", "test-group", memberToAdd, "0,0", "", "--privatekey", ""},
			errContains: "should have the same length",
		},
		{
			name:        "invalid hex in member to add",
			args:        []string{"update-group-member", "test-group", "not-a-valid-address", "0", "", "--privatekey", ""},
			errContains: "invalid address hex length",
		},
		{
			name:        "invalid expiration timestamp",
			args:        []string{"update-group-member", "test-group", memberToAdd, "not-a-number", "", "--privatekey", ""},
			errContains: "invalid syntax",
		},
		{
			name:        "invalid hex in member to delete",
			args:        []string{"update-group-member", "test-group", memberToAdd, "0", "not-a-valid-address", "--privatekey", ""},
			errContains: "invalid address hex length",
		},
		{
			// Second add-entry is empty (skipped) and its paired expiration is empty (defaults
			// to zero); the delete list carries one real entry. All local validation still
			// passes, so this reaches the private-key gate like the aligned happy-path test.
			name: "empty entries default and skip, reaches private key gate",
			args: []string{
				"update-group-member", "test-group",
				memberToAdd + ",", ",0", memberToDelete,
				"--privatekey", "",
			},
			errContains: gateErrSubstring,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.Require().ErrorContains(err, tc.errContains)
		})
	}
}

// TestRenewGroupMember_ValidationBranches covers the CmdRenewGroupMember branches that
// TestRenewGroupMember_SliceBuild_Aligned_NoPanic (above) does not exercise: the length-mismatch
// guard, an invalid hex member, an invalid expiration timestamp, and the "skip empty entry" /
// "default expiration to zero" paths.
func (s *CLITestSuite) TestRenewGroupMember_ValidationBranches() {
	member := sample.RandAccAddressHex()

	testCases := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "mismatched member and expiration length",
			args:        []string{"renew-group-member", "test-group", member, "0,0", "--privatekey", ""},
			errContains: "should have the same length",
		},
		{
			name:        "invalid hex member",
			args:        []string{"renew-group-member", "test-group", "not-a-valid-address", "0", "--privatekey", ""},
			errContains: "invalid address hex length",
		},
		{
			name:        "invalid expiration timestamp",
			args:        []string{"renew-group-member", "test-group", member, "not-a-number", "--privatekey", ""},
			errContains: "invalid syntax",
		},
		{
			// Second entry is empty (skipped) and its paired expiration is empty (defaults to
			// zero); local validation still passes so this reaches the private-key gate.
			name:        "empty entries default and skip, reaches private key gate",
			args:        []string{"renew-group-member", "test-group", member + ",", ",0", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.Require().ErrorContains(err, tc.errContains)
		})
	}
}

func (s *CLITestSuite) TestCmdUpdateGroupExtra() {
	cmd := cli.GetTxCmd()
	args := []string{"update-group-extra", "test-group", "extra info", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdPutPolicy() {
	cmd := cli.GetTxCmd()
	args := []string{"put-policy", sample.RandAccAddressHex(), "grn:b::test-bucket", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdDeletePolicy() {
	cmd := cli.GetTxCmd()
	args := []string{"delete-policy", sample.RandAccAddressHex(), "grn:b::test-bucket", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdSetTag() {
	cmd := cli.GetTxCmd()
	args := []string{"set-tag", "grn:b::test-bucket", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdToggleSPAsDelegatedAgent() {
	cmd := cli.GetTxCmd()
	args := []string{"toggle-sp-as-delegated-agent", "test-bucket", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdCancelCreateObject() {
	cmd := cli.GetTxCmd()
	args := []string{"cancel-create-object", "test-bucket", "test-object", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdCreateObject() {
	validChecksums := hex.EncodeToString(sample.Checksum()) + "," + hex.EncodeToString(sample.Checksum())

	testCases := []struct {
		name        string
		args        []string
		errIs       error
		errContains string
	}{
		{
			name: "EC redundancy reaches private key gate",
			args: []string{
				"create-object", "test-bucket", "test-object", "1024", "text/plain",
				"--expect-checksums", validChecksums,
				"--redundancy-type", "EC",
				"--privatekey", "",
			},
			errContains: gateErrSubstring,
		},
		{
			name: "Replica redundancy reaches private key gate",
			args: []string{
				"create-object", "test-bucket", "test-object", "1024", "text/plain",
				"--expect-checksums", validChecksums,
				"--redundancy-type", "Replica",
				"--privatekey", "",
			},
			errContains: gateErrSubstring,
		},
		{
			name: "invalid payload size",
			args: []string{
				"create-object", "test-bucket", "test-object", "not-a-number", "text/plain",
				"--expect-checksums", validChecksums,
				"--redundancy-type", "EC",
				"--privatekey", "",
			},
			errContains: "invalid syntax",
		},
		{
			name: "invalid visibility",
			args: []string{
				"create-object", "test-bucket", "test-object", "1024", "text/plain",
				"--visibility", "BOGUS_VISIBILITY",
				"--expect-checksums", validChecksums,
				"--redundancy-type", "EC",
				"--privatekey", "",
			},
			errIs: gnfderrors.ErrInvalidVisibilityType,
		},
		{
			name: "invalid checksum hex",
			args: []string{
				"create-object", "test-bucket", "test-object", "1024", "text/plain",
				"--expect-checksums", "not-hex-at-all",
				"--redundancy-type", "EC",
				"--privatekey", "",
			},
			errContains: "invalid byte",
		},
		{
			name: "invalid redundancy type",
			args: []string{
				"create-object", "test-bucket", "test-object", "1024", "text/plain",
				"--expect-checksums", validChecksums,
				"--redundancy-type", "BOGUS",
				"--privatekey", "",
			},
			errIs: storagetypes.ErrInvalidRedundancyType,
		},
		{
			name: "invalid approve signature",
			args: []string{
				"create-object", "test-bucket", "test-object", "1024", "text/plain",
				"--expect-checksums", validChecksums,
				"--redundancy-type", "EC",
				"--approve-signature", "zz",
				"--privatekey", "",
			},
			errContains: "invalid byte",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.assertValidationErr(err, tc.errIs, tc.errContains)
		})
	}
}

func (s *CLITestSuite) TestCmdCopyObject() {
	testCases := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "reaches private key gate",
			args:        []string{"copy-object", "src-bucket", "dst-bucket", "src-object", "dst-object", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
		{
			name: "invalid approve signature",
			args: []string{
				"copy-object", "src-bucket", "dst-bucket", "src-object", "dst-object",
				"--approve-signature", "zz",
				"--privatekey", "",
			},
			errContains: "invalid byte",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.Require().ErrorContains(err, tc.errContains)
		})
	}
}

func (s *CLITestSuite) TestCmdDeleteObject() {
	cmd := cli.GetTxCmd()
	args := []string{"delete-object", "test-bucket", "test-object", "--privatekey", ""}
	err := s.execExpectError(cmd, args)
	s.Require().ErrorContains(err, gateErrSubstring)
}

func (s *CLITestSuite) TestCmdUpdateObjectInfo() {
	testCases := []struct {
		name        string
		args        []string
		errIs       error
		errContains string
	}{
		{
			name:        "reaches private key gate",
			args:        []string{"update-object-info", "test-bucket", "test-object", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
		{
			name: "invalid visibility",
			args: []string{
				"update-object-info", "test-bucket", "test-object",
				"--visibility", "BOGUS_VISIBILITY",
				"--privatekey", "",
			},
			errIs: gnfderrors.ErrInvalidVisibilityType,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.assertValidationErr(err, tc.errIs, tc.errContains)
		})
	}
}

func (s *CLITestSuite) TestCmdDiscontinueObject() {
	testCases := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "reaches private key gate",
			args:        []string{"discontinue-object", "test-bucket", "1,2,3", "test reason", "--privatekey", ""},
			errContains: gateErrSubstring,
		},
		{
			name:        "invalid object id",
			args:        []string{"discontinue-object", "test-bucket", "not-a-number", "test reason", "--privatekey", ""},
			errContains: "invalid object id",
		},
		{
			// "--" stops pflag from trying (and failing) to parse "-1" as a flag; everything
			// after it is taken as positional args, so --privatekey is left at its zero-value
			// default ("") which is exactly what every other case sets explicitly.
			name:        "negative object id",
			args:        []string{"discontinue-object", "test-bucket", "--", "-1", "test reason"},
			errContains: "object id should not be negative",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()
			err := s.execExpectError(cmd, tc.args)
			s.Require().ErrorContains(err, tc.errContains)
		})
	}
}
