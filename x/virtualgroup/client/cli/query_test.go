package cli_test

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/gogoproto/proto"

	"github.com/mocachain/moca/v2/x/virtualgroup/client/cli"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// invalidIDMsg is the error the global-virtual-group query commands return for an unusable id.
func invalidIDMsg(id string) string { return "invalid GVG id " + id }

const (
	nonNumericID  = "abc"
	nonPositiveID = "0"
)

// errRPCFailureLog is the mocked ABCI query log used to exercise the "query
// RPC failed" error branch shared by every query command below.
const errRPCFailureLog = "boom"

func (s *CLITestSuite) TestQueryCmd() {
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}

	// A client context whose mocked CometRPC answers every ABCI query with a
	// non-OK code, to exercise the "query RPC failed" error branch shared by
	// every query command below.
	errClientCtx := s.clientCtx.WithClient(clitestutil.NewMockCometRPC(abci.ResponseQuery{
		Code: 1,
		Log:  errRPCFailureLog,
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
			true, true, errRPCFailureLog, nil,
		},
		{
			"query global-virtual-group",
			append(
				[]string{
					"global-virtual-group",
					"1",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryGlobalVirtualGroupResponse{},
		},
		{
			"query global-virtual-group RPC failure",
			append(
				[]string{
					"global-virtual-group",
					"1",
				},
				commonFlags...,
			),
			true, true, errRPCFailureLog, nil,
		},
		{
			"query global-virtual-group-by-family-id",
			append(
				[]string{
					"global-virtual-group-by-family-id",
					"1",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryGlobalVirtualGroupByFamilyIDResponse{},
		},
		{
			"query global-virtual-group-by-family-id RPC failure",
			append(
				[]string{
					"global-virtual-group-by-family-id",
					"1",
				},
				commonFlags...,
			),
			true, true, errRPCFailureLog, nil,
		},
		{
			"query global-virtual-group-families",
			append(
				[]string{
					"global-virtual-group-families",
					"100",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryGlobalVirtualGroupFamiliesResponse{},
		},
		{
			// query_global_virtual_group_families.go is the one command in
			// this file group whose invalid-id branch correctly indexes
			// args[0]; its 3 siblings panic on args[1] with ExactArgs(1)
			// (see Findings in the PR body) so their invalid-id branches are
			// not covered here.
			"query global-virtual-group-families invalid limit",
			append(
				[]string{
					"global-virtual-group-families",
					"0",
				},
				commonFlags...,
			),
			false, true, "invalid limit", nil,
		},
		{
			"query global-virtual-group-families RPC failure",
			append(
				[]string{
					"global-virtual-group-families",
					"100",
				},
				commonFlags...,
			),
			true, true, errRPCFailureLog, nil,
		},
		{
			"query global-virtual-group-family",
			append(
				[]string{
					"global-virtual-group-family",
					"1",
				},
				commonFlags...,
			),
			false, false, "", &types.QueryGlobalVirtualGroupFamilyResponse{},
		},
		{
			"query global-virtual-group-family RPC failure",
			append(
				[]string{
					"global-virtual-group-family",
					"1",
				},
				commonFlags...,
			),
			true, true, errRPCFailureLog, nil,
		},
		{
			"query global-virtual-group rejects id abc",
			append(
				[]string{
					"global-virtual-group",
					nonNumericID,
				},
				commonFlags...,
			),
			false, true, invalidIDMsg(nonNumericID), nil,
		},
		{
			"query global-virtual-group rejects id 0",
			append(
				[]string{
					"global-virtual-group",
					nonPositiveID,
				},
				commonFlags...,
			),
			false, true, invalidIDMsg(nonPositiveID), nil,
		},
		{
			"query global-virtual-group-by-family-id rejects id abc",
			append(
				[]string{
					"global-virtual-group-by-family-id",
					nonNumericID,
				},
				commonFlags...,
			),
			false, true, invalidIDMsg(nonNumericID), nil,
		},
		{
			"query global-virtual-group-by-family-id rejects id 0",
			append(
				[]string{
					"global-virtual-group-by-family-id",
					nonPositiveID,
				},
				commonFlags...,
			),
			false, true, invalidIDMsg(nonPositiveID), nil,
		},
		{
			"query global-virtual-group-family rejects id abc",
			append(
				[]string{
					"global-virtual-group-family",
					nonNumericID,
				},
				commonFlags...,
			),
			false, true, invalidIDMsg(nonNumericID), nil,
		},
		{
			"query global-virtual-group-family rejects id 0",
			append(
				[]string{
					"global-virtual-group-family",
					nonPositiveID,
				},
				commonFlags...,
			),
			false, true, invalidIDMsg(nonPositiveID), nil,
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
