package cli_test

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/gogoproto/proto"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/storage/client/cli"
	"github.com/mocachain/moca/v2/x/storage/types"
)

func (s *CLITestSuite) TestQueryCmd() {
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}

	testCases := []struct {
		name         string
		args         []string
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
			false, "", &types.QueryParamsResponse{},
		},
		{
			"query head-bucket",
			append(
				[]string{
					"head-bucket",
					"bucketName",
				},
				commonFlags...,
			),
			false, "", &types.QueryHeadBucketResponse{},
		},
		{
			"query head-group",
			append(
				[]string{
					"head-group",
					sample.RandAccAddressHex(),
					"groupName",
				},
				commonFlags...,
			),
			false, "", &types.QueryHeadGroupResponse{},
		},
		{
			"query head-group-member",
			append(
				[]string{
					"head-group-member",
					sample.RandAccAddressHex(),
					"groupName",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryHeadGroupMemberResponse{},
		},
		{
			"query head-object",
			append(
				[]string{
					"head-object",
					"bucketName",
					"objectName",
				},
				commonFlags...,
			),
			false, "", &types.QueryHeadObjectResponse{},
		},
		{
			"query list-buckets",
			append(
				[]string{
					"list-buckets",
				},
				commonFlags...,
			),
			false, "", &types.QueryListBucketsResponse{},
		},
		{
			"query list-groups",
			append(
				[]string{
					"list-groups",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryListGroupsResponse{},
		},
		{
			"query list-objects",
			append(
				[]string{
					"list-objects",
					"bucketName",
				},
				commonFlags...,
			),
			false, "", &types.QueryListObjectsResponse{},
		},
		{
			"query verify-permission",
			append(
				[]string{
					"verify-permission",
					sample.RandAccAddressHex(),
					"bucketName",
					"objectName",
					"ACTION_TYPE_ALL",
				},
				commonFlags...,
			),
			false, "", &types.QueryVerifyPermissionResponse{},
		},
		{
			"query account-policy",
			append(
				[]string{
					"account-policy",
					"grn:o::testbucket/testobject",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryPolicyForAccountResponse{},
		},
		{
			"query group-policy",
			append(
				[]string{
					"group-policy",
					"grn:o::testbucket/testobject",
					"1",
				},
				commonFlags...,
			),
			false, "", &types.QueryPolicyForGroupResponse{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetQueryCmd()
			out, err := clitestutil.ExecTestCLICmd(s.clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErrMsg)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(s.clientCtx.Codec.UnmarshalJSON(out.Bytes(), tc.respType), out.String())
			}
		})
	}
}

// TestQueryCmd_ABCIError covers the "query failed" branch of every query command: the local
// flags/args all parse fine, but the node itself reports a failure. Reusing a client whose mocked
// CometBFT RPC always answers with a non-OK ABCI response lets every command reach and exercise
// its own `if err != nil { return err }` after the query call, without touching a real network.
func (s *CLITestSuite) TestQueryCmd_ABCIError() {
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}

	errClientCtx := s.baseCtx.WithClient(clitestutil.NewMockCometRPC(abci.ResponseQuery{
		Code: 1,
		Log:  "boom",
	}))

	testCases := []struct {
		name string
		args []string
	}{
		{"params", []string{"params"}},
		{"head-bucket", []string{"head-bucket", "bucketName"}},
		{"head-group", []string{"head-group", sample.RandAccAddressHex(), "groupName"}},
		{"head-group-member", []string{"head-group-member", sample.RandAccAddressHex(), "groupName", sample.RandAccAddressHex()}},
		{"head-object", []string{"head-object", "bucketName", "objectName"}},
		{"list-buckets", []string{"list-buckets"}},
		{"list-groups", []string{"list-groups", sample.RandAccAddressHex()}},
		{"list-objects", []string{"list-objects", "bucketName"}},
		{"verify-permission", []string{"verify-permission", sample.RandAccAddressHex(), "bucketName", "objectName", "ACTION_TYPE_ALL"}},
		{"account-policy", []string{"account-policy", "grn:o::testbucket/testobject", sample.RandAccAddressHex()}},
		{"group-policy", []string{"group-policy", "grn:o::testbucket/testobject", "1"}},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetQueryCmd()
			args := append(append([]string{}, tc.args...), commonFlags...)
			_, err := clitestutil.ExecTestCLICmd(errClientCtx, cmd, args)
			s.Require().Error(err)
		})
	}
}

// TestQueryCmd_InvalidInput covers the local validation error branches that sit before any
// network call: an unparseable action type, GRN, principal address or group id.
func (s *CLITestSuite) TestQueryCmd_InvalidInput() {
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}

	testCases := []struct {
		name string
		args []string
	}{
		{
			"verify-permission with an invalid action type",
			[]string{"verify-permission", sample.RandAccAddressHex(), "bucketName", "objectName", "ACTION_BOGUS"},
		},
		{
			"account-policy with an invalid grn",
			[]string{"account-policy", "not-a-grn", sample.RandAccAddressHex()},
		},
		{
			"account-policy with an invalid principal address",
			[]string{"account-policy", "grn:o::testbucket/testobject", "not-a-hex-address"},
		},
		{
			"group-policy with an invalid grn",
			[]string{"group-policy", "not-a-grn", "1"},
		},
		{
			"group-policy with a non-numeric group id",
			[]string{"group-policy", "grn:o::testbucket/testobject", "not-a-number"},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetQueryCmd()
			args := append(append([]string{}, tc.args...), commonFlags...)
			_, err := clitestutil.ExecTestCLICmd(s.clientCtx, cmd, args)
			s.Require().Error(err)
		})
	}
}
