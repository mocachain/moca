package cli_test

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/gogoproto/proto"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/client/cli"
	"github.com/mocachain/moca/v2/x/sp/types"
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
			"query storage-provider",
			append(
				[]string{
					"storage-provider",
					"1",
				},
				commonFlags...,
			),
			false, "", &types.QueryStorageProviderResponse{},
		},
		{
			"query storage-provider-by-operator-address",
			append(
				[]string{
					"storage-provider-by-operator-address",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryStorageProviderByOperatorAddressResponse{},
		},
		{
			"query storage-providers",
			append(
				[]string{
					"storage-providers",
				},
				commonFlags...,
			),
			false, "", &types.QueryStorageProvidersResponse{},
		},
		{
			"query maintenance-records-by-operator-address",
			append(
				[]string{
					"maintenance-records-by-operator-address",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryStorageProviderMaintenanceRecordsResponse{},
		},
		{
			"query price",
			append(
				[]string{
					"price",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QuerySpStoragePriceResponse{},
		},
		{
			"query global-price",
			append(
				[]string{
					"global-price",
					"0",
				},
				commonFlags...,
			),
			false, "", &types.QueryGlobalSpStorePriceByTimeResponse{},
		},
		// --- local arg-parse error paths below: these fail before any query is
		// sent, so they exercise each command's own validation branch rather
		// than the shared ABCI-error path covered in TestQueryCmd_ABCIError.
		{
			"query storage-provider - non-numeric id",
			append(
				[]string{
					"storage-provider",
					"not-a-number",
				},
				commonFlags...,
			),
			true, "invalid syntax", nil,
		},
		{
			"query storage-provider-by-operator-address - invalid address",
			append(
				[]string{
					"storage-provider-by-operator-address",
					"not-a-hex-address",
				},
				commonFlags...,
			),
			true, "invalid address hex length", nil,
		},
		{
			"query maintenance-records-by-operator-address - invalid address",
			append(
				[]string{
					"maintenance-records-by-operator-address",
					"not-a-hex-address",
				},
				commonFlags...,
			),
			true, "invalid address hex length", nil,
		},
		{
			"query price - invalid address",
			append(
				[]string{
					"price",
					"not-a-hex-address",
				},
				commonFlags...,
			),
			true, "invalid address hex length", nil,
		},
		{
			"query global-price - non-numeric timestamp",
			append(
				[]string{
					"global-price",
					"not-a-number",
				},
				commonFlags...,
			),
			true, "invalid syntax", nil,
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

// TestQueryCmd_ABCIError covers the "query failed" branch of every query
// command: local flags/args all parse fine, but the node itself reports a
// failure. A client whose mocked CometBFT RPC always answers with a non-OK
// ABCI response lets every command reach and exercise its own
// "if err != nil { return err }" after the query call, without a real node.
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
		{"storage-provider", []string{"storage-provider", "1"}},
		{"storage-provider-by-operator-address", []string{"storage-provider-by-operator-address", sample.RandAccAddressHex()}},
		{"storage-providers", []string{"storage-providers"}},
		{"maintenance-records-by-operator-address", []string{"maintenance-records-by-operator-address", sample.RandAccAddressHex()}},
		{"price", []string{"price", sample.RandAccAddressHex()}},
		{"global-price", []string{"global-price", "0"}},
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
