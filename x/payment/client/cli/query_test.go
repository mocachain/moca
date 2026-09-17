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
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/payment"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/client/cli"
	"github.com/mocachain/moca/v2/x/payment/types"
)

const (
	errMockQueryFailure   = "mock query failure"
	errPageOffsetConflict = "page and offset cannot be used together"
	errInvalidNodeURL     = "invalid control character in URL"
	testBadNodeURL        = "://bad host \x00"
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
		expectErr    bool
		expectErrMsg string
		respType     proto.Message
		useErrCtx    bool
	}{
		{
			"query params",
			append(
				[]string{
					"params",
				},
				commonFlags...,
			),
			false, "", &types.QueryParamsResponse{}, false,
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
			false, "", &types.QueryDynamicBalanceResponse{}, false,
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
			false, "", &types.QueryPaymentAccountsByOwnerResponse{}, false,
		},
		{
			"query list-auto-settle-record",
			append(
				[]string{
					"list-auto-settle-record",
				},
				commonFlags...,
			),
			false, "", &types.QueryAutoSettleRecordsResponse{}, false,
		},
		{
			"query list-payment-account",
			append(
				[]string{
					"list-payment-account",
				},
				commonFlags...,
			),
			false, "", &types.QueryPaymentAccountsResponse{}, false,
		},
		{
			"query list-payment-account-count",
			append(
				[]string{
					"list-payment-account-count",
				},
				commonFlags...,
			),
			false, "", &types.QueryPaymentAccountCountsResponse{}, false,
		},
		{
			"query list-stream-record",
			append(
				[]string{
					"list-stream-record",
				},
				commonFlags...,
			),
			false, "", &types.QueryStreamRecordsResponse{}, false,
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
			false, "", &types.QueryPaymentAccountResponse{}, false,
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
			false, "", &types.QueryPaymentAccountCountResponse{}, false,
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
			false, "", &types.QueryGetStreamRecordResponse{}, false,
		},
		// --- error paths below: none of the happy-path cases above ever hit an
		// "if err != nil { return err }" branch, so every one of those branches
		// was 0% covered. Each case here forces exactly one such branch.
		{
			"query params - query error",
			append(
				[]string{
					"params",
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query dynamic-balance - bad node",
			append(
				[]string{
					"dynamic-balance",
					sample.RandAccAddressHex(),
					fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
				},
				commonFlags...,
			),
			true, errInvalidNodeURL, nil, false,
		},
		{
			"query dynamic-balance - query error",
			append(
				[]string{
					"dynamic-balance",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query get-payment-accounts-by-owner - bad node",
			append(
				[]string{
					"get-payment-accounts-by-owner",
					sample.RandAccAddressHex(),
					fmt.Sprintf("--%s=%s", flags.FlagNode, testBadNodeURL),
				},
				commonFlags...,
			),
			true, errInvalidNodeURL, nil, false,
		},
		{
			"query get-payment-accounts-by-owner - query error",
			append(
				[]string{
					"get-payment-accounts-by-owner",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query list-auto-settle-record - page and offset conflict",
			append(
				[]string{
					"list-auto-settle-record",
					fmt.Sprintf("--%s=%d", flags.FlagPage, 2),
					fmt.Sprintf("--%s=%d", flags.FlagOffset, 5),
				},
				commonFlags...,
			),
			true, errPageOffsetConflict, nil, false,
		},
		{
			"query list-auto-settle-record - query error",
			append(
				[]string{
					"list-auto-settle-record",
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query list-payment-account - page and offset conflict",
			append(
				[]string{
					"list-payment-account",
					fmt.Sprintf("--%s=%d", flags.FlagPage, 2),
					fmt.Sprintf("--%s=%d", flags.FlagOffset, 5),
				},
				commonFlags...,
			),
			true, errPageOffsetConflict, nil, false,
		},
		{
			"query list-payment-account - query error",
			append(
				[]string{
					"list-payment-account",
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query list-payment-account-count - page and offset conflict",
			append(
				[]string{
					"list-payment-account-count",
					fmt.Sprintf("--%s=%d", flags.FlagPage, 2),
					fmt.Sprintf("--%s=%d", flags.FlagOffset, 5),
				},
				commonFlags...,
			),
			true, errPageOffsetConflict, nil, false,
		},
		{
			"query list-payment-account-count - query error",
			append(
				[]string{
					"list-payment-account-count",
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query list-stream-record - page and offset conflict",
			append(
				[]string{
					"list-stream-record",
					fmt.Sprintf("--%s=%d", flags.FlagPage, 2),
					fmt.Sprintf("--%s=%d", flags.FlagOffset, 5),
				},
				commonFlags...,
			),
			true, errPageOffsetConflict, nil, false,
		},
		{
			"query list-stream-record - query error",
			append(
				[]string{
					"list-stream-record",
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query show-payment-account - query error",
			append(
				[]string{
					"show-payment-account",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query show-payment-account-count - query error",
			append(
				[]string{
					"show-payment-account-count",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
		{
			"query show-stream-record - query error",
			append(
				[]string{
					"show-stream-record",
					sample.RandAccAddressHex(),
				},
				commonFlags...,
			),
			true, errMockQueryFailure, nil, true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetQueryCmd()

			cctx := s.clientCtx
			if tc.useErrCtx {
				cctx = errClientCtx
			}

			out, err := clitestutil.ExecTestCLICmd(cctx, cmd, tc.args)

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

// TestGetEvmQueryCmd only exercises command construction (GetEvmQueryCmd and,
// transitively, every CmdEvmXxx constructor): building the cobra tree does not
// invoke any RunE closure, so this never reaches the live-EVM-RPC calls inside
// them. Those RunE bodies are out of scope for this PR (see PR body).
func TestGetEvmQueryCmd(t *testing.T) {
	cmd := cli.GetEvmQueryCmd()
	require.Equal(t, types.ModuleName, cmd.Name())
	require.Len(t, cmd.Commands(), 10)
}

func TestToPaymentPageReq(t *testing.T) {
	require.Nil(t, cli.ToPaymentPageReq(nil))

	in := &query.PageRequest{
		Key:        []byte("some-key"),
		Offset:     3,
		Limit:      10,
		CountTotal: true,
		Reverse:    true,
	}
	out := cli.ToPaymentPageReq(in)
	require.Equal(t, &payment.PageRequest{
		Key:        in.Key,
		Offset:     in.Offset,
		Limit:      in.Limit,
		CountTotal: in.CountTotal,
		Reverse:    in.Reverse,
	}, out)
}

func TestToPageResp(t *testing.T) {
	require.Nil(t, cli.ToPageResp(nil))

	in := &payment.PageResponse{
		NextKey: []byte("next-key"),
		Total:   7,
	}
	out := cli.ToPageResp(in)
	require.Equal(t, &query.PageResponse{
		NextKey: in.NextKey,
		Total:   in.Total,
	}, out)
}

func TestToStreamRecord(t *testing.T) {
	require.Nil(t, cli.ToStreamRecord(nil))

	in := &payment.StreamRecord{
		Account:           sample.RandAccAddressHex(),
		CrudTimestamp:     100,
		NetflowRate:       big.NewInt(5),
		StaticBalance:     big.NewInt(10),
		BufferBalance:     big.NewInt(15),
		LockBalance:       big.NewInt(20),
		Status:            int32(types.STREAM_ACCOUNT_STATUS_FROZEN),
		SettleTimestamp:   200,
		OutFlowCount:      3,
		FrozenNetflowRate: big.NewInt(25),
	}
	out := cli.ToStreamRecord(in)
	require.Equal(t, &types.StreamRecord{
		Account:           in.Account,
		CrudTimestamp:     in.CrudTimestamp,
		NetflowRate:       sdkmath.NewIntFromBigInt(in.NetflowRate),
		StaticBalance:     sdkmath.NewIntFromBigInt(in.StaticBalance),
		BufferBalance:     sdkmath.NewIntFromBigInt(in.BufferBalance),
		LockBalance:       sdkmath.NewIntFromBigInt(in.LockBalance),
		Status:            types.STREAM_ACCOUNT_STATUS_FROZEN,
		SettleTimestamp:   in.SettleTimestamp,
		OutFlowCount:      in.OutFlowCount,
		FrozenNetflowRate: sdkmath.NewIntFromBigInt(in.FrozenNetflowRate),
	}, out)
}

func TestToPaymentAccount(t *testing.T) {
	require.Nil(t, cli.ToPaymentAccount(nil))

	in := &payment.PaymentAccount{
		Addr:       sample.RandAccAddressHex(),
		Owner:      sample.RandAccAddressHex(),
		Refundable: true,
	}
	out := cli.ToPaymentAccount(in)
	require.Equal(t, &types.PaymentAccount{
		Addr:       in.Addr,
		Owner:      in.Owner,
		Refundable: in.Refundable,
	}, out)
}

func TestToPaymentAccountCount(t *testing.T) {
	require.Nil(t, cli.ToPaymentAccountCount(nil))

	in := &payment.PaymentAccountCount{
		Owner: sample.RandAccAddressHex(),
		Count: 4,
	}
	out := cli.ToPaymentAccountCount(in)
	require.Equal(t, &types.PaymentAccountCount{
		Owner: in.Owner,
		Count: in.Count,
	}, out)
}

func TestToAutoSettleRecord(t *testing.T) {
	require.Nil(t, cli.ToAutoSettleRecord(nil))

	in := &payment.AutoSettleRecord{
		Timestamp: 42,
		Addr:      sample.RandAccAddressHex(),
	}
	out := cli.ToAutoSettleRecord(in)
	require.Equal(t, &types.AutoSettleRecord{
		Timestamp: in.Timestamp,
		Addr:      in.Addr,
	}, out)
}
