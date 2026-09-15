package cli_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/mocachain/moca/v2/precompiles/payment"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/client/cli"
	"github.com/mocachain/moca/v2/x/payment/types"
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
			"query dynamic-balance",
			append(
				[]string{
					"dynamic-balance",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryDynamicBalanceResponse{},
		},
		{
			"query get-payment-accounts-by-owner",
			append(
				[]string{
					"get-payment-accounts-by-owner",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryPaymentAccountsByOwnerResponse{},
		},
		{
			"query list-auto-settle-record",
			append(
				[]string{
					"list-auto-settle-record",
				},
				commonFlags...,
			),
			false, "", &types.QueryAutoSettleRecordsResponse{},
		},
		{
			"query list-payment-account",
			append(
				[]string{
					"list-payment-account",
				},
				commonFlags...,
			),
			false, "", &types.QueryPaymentAccountsResponse{},
		},
		{
			"query list-payment-account-count",
			append(
				[]string{
					"list-payment-account-count",
				},
				commonFlags...,
			),
			false, "", &types.QueryPaymentAccountCountsResponse{},
		},
		{
			"query list-stream-record",
			append(
				[]string{
					"list-stream-record",
				},
				commonFlags...,
			),
			false, "", &types.QueryStreamRecordsResponse{},
		},
		{
			"query show-payment-account",
			append(
				[]string{
					"show-payment-account",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryPaymentAccountResponse{},
		},
		{
			"query show-payment-account-count",
			append(
				[]string{
					"show-payment-account-count",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryPaymentAccountCountResponse{},
		},
		{
			"query show-stream-record",
			append(
				[]string{
					"show-stream-record",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			false, "", &types.QueryGetStreamRecordResponse{},
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

// newEVMRPCStub serves a canned eth_call result so the precompile-backed query
// commands can be exercised without a node.
func newEVMRPCStub(t *testing.T, result []byte) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode rpc request: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":"0x%s"}`, req.ID, hex.EncodeToString(result))
	}))
}

// The precompile returns validator_tax_rate as the raw LegacyDec integer
// (rate * 10^18), so the command has to scale it back down when rebuilding the
// response: 10^16 must render as 0.01, not as 10^16.
func (s *CLITestSuite) TestEvmQueryParamsScalesValidatorTaxRate() {
	taxRate := sdkmath.LegacyNewDecWithPrec(1, 2)

	paymentABI, err := payment.IPaymentMetaData.GetAbi()
	s.Require().NoError(err)

	result, err := paymentABI.Methods["params"].Outputs.Pack(payment.Params{
		VersionedParams: payment.VersionedParams{
			ReserveTime:      60,
			ValidatorTaxRate: taxRate.BigInt(),
		},
		PaymentAccountCountLimit:  200,
		ForcedSettleTime:          30,
		MaxAutoSettleFlowCount:    100,
		MaxAutoResumeFlowCount:    100,
		FeeDenom:                  "amoca",
		WithdrawTimeLockThreshold: big.NewInt(100),
		WithdrawTimeLockDuration:  86400,
	})
	s.Require().NoError(err)

	srv := newEVMRPCStub(s.T(), result)
	defer srv.Close()

	evmClient, err := ethclient.Dial(srv.URL)
	s.Require().NoError(err)
	defer evmClient.Close()

	out, err := clitestutil.ExecTestCLICmd(
		s.clientCtx.WithEvmClient(evmClient),
		cli.GetEvmQueryCmd(),
		[]string{"params"},
	)
	s.Require().NoError(err)

	var resp types.QueryParamsResponse
	s.Require().NoError(s.clientCtx.Codec.UnmarshalJSON(out.Bytes(), &resp), out.String())
	s.Require().Equal(taxRate.String(), resp.Params.VersionedParams.ValidatorTaxRate.String())
}
