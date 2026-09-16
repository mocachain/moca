package cli_test

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/gogoproto/proto"

	"github.com/mocachain/moca/v2/x/challenge/client/cli"
	"github.com/mocachain/moca/v2/x/challenge/types"
)

const (
	errMockQueryFailure = "mock query failure"
	errInvalidNodeURL   = "invalid control character in URL"
	testBadNodeURL      = "://bad host \x00"

	cmdAttestedChallenge = "attested-challenge"
	cmdSubmit            = "submit"
	cmdAttest            = "attest"
	argNotANumber        = "not-a-number"
	argNotAnAddress      = "not-an-address"
	argTestBucketName    = "test-bucket"
	argTestObjectName    = "test-object"
	argTrue              = "true"
	argFalse             = "false"

	// argChallengeID/argObjectID are deliberately distinct (and non-numeric
	// look-alikes of each other) so a mutation that swaps the challenge-id
	// and object-id positional arguments in CmdAttest changes the decoded
	// message instead of silently reproducing the same "1"/"1" row.
	argChallengeID = "7"
	argObjectID    = "11"
)

func (s *CLITestSuite) TestQueryCmd() {
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}

	// errClientCtx behaves like s.clientCtx for everything except that the
	// mocked CometBFT RPC returns a non-OK ABCI response, so any query that
	// reaches queryClient.<Method>(...) deterministically fails without a
	// live backend.
	errClientCtx := s.clientCtx.WithClient(clitestutil.NewMockCometRPC(abci.ResponseQuery{
		Code: 1,
		Log:  errMockQueryFailure,
	}))

	testCases := []struct {
		name         string
		args         []string
		useErrClient bool
		expectErr    bool
		expectErrMsg string
		respType     proto.Message
	}{
		{
			"query params",
			append(
				[]string{
					"params",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryParamsResponse{},
		},
		{
			"query params RPC failure",
			append(
				[]string{
					"params",
				},
				commonFlags...,
			),
			true, true, errMockQueryFailure, nil,
		},
		{
			"query latest-attested-challenges",
			append(
				[]string{
					"latest-attested-challenges",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryLatestAttestedChallengesResponse{},
		},
		{
			"query latest-attested-challenges bad node",
			append(
				[]string{
					"latest-attested-challenges",
					fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
				},
				commonFlags...,
			),
			false, true, errInvalidNodeURL, nil,
		},
		{
			"query latest-attested-challenges RPC failure",
			append(
				[]string{
					"latest-attested-challenges",
				},
				commonFlags...,
			),
			true, true, errMockQueryFailure, nil,
		},
		{
			"query inturn-attestation-submitter",
			append(
				[]string{
					"inturn-attestation-submitter",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryInturnAttestationSubmitterResponse{},
		},
		{
			"query inturn-attestation-submitter bad node",
			append(
				[]string{
					"inturn-attestation-submitter",
					fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
				},
				commonFlags...,
			),
			false, true, errInvalidNodeURL, nil,
		},
		{
			"query inturn-attestation-submitter RPC failure",
			append(
				[]string{
					"inturn-attestation-submitter",
				},
				commonFlags...,
			),
			true, true, errMockQueryFailure, nil,
		},
		{
			"query attested-challenge",
			append(
				[]string{
					cmdAttestedChallenge,
					"1",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryAttestedChallengeResponse{},
		},
		{
			"query attested-challenge invalid challenge-id",
			append(
				[]string{
					cmdAttestedChallenge,
					argNotANumber,
				},
				commonFlags...,
			),
			false, true, "please input a valid challenge-id", nil,
		},
		{
			"query attested-challenge bad node",
			append(
				[]string{
					cmdAttestedChallenge,
					"1",
					fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
				},
				commonFlags...,
			),
			false, true, errInvalidNodeURL, nil,
		},
		{
			"query attested-challenge RPC failure",
			append(
				[]string{
					cmdAttestedChallenge,
					"1",
				},
				commonFlags...,
			),
			true, true, errMockQueryFailure, nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			ctx := s.clientCtx
			if tc.useErrClient {
				ctx = errClientCtx
			}

			cmd := cli.GetQueryCmd()
			out, err := clitestutil.ExecTestCLICmd(ctx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErrMsg)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(ctx.Codec.UnmarshalJSON(out.Bytes(), tc.respType), out.String())
			}
		})
	}
}
