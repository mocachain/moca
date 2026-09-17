package cli_test

import (
	"context"
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
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/ethclient"

	spp "github.com/mocachain/moca/v2/precompiles/storageprovider"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/client/cli"
	"github.com/mocachain/moca/v2/x/sp/types"
)

const (
	cmdMaintenanceRecordsByOperatorAddress = "maintenance-records-by-operator-address"
	cmdPrice                               = "price"
	cmdGlobalPrice                         = "global-price"
	notAHexAddress                         = "not-a-hex-address"
	invalidAddressHexLengthErr             = "invalid address hex length"
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
					cmdMaintenanceRecordsByOperatorAddress,
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
					cmdPrice,
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
					cmdGlobalPrice,
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
					notAHexAddress,
				},
				commonFlags...,
			),
			true, invalidAddressHexLengthErr, nil,
		},
		{
			"query maintenance-records-by-operator-address - invalid address",
			append(
				[]string{
					cmdMaintenanceRecordsByOperatorAddress,
					notAHexAddress,
				},
				commonFlags...,
			),
			true, invalidAddressHexLengthErr, nil,
		},
		{
			"query price - invalid address",
			append(
				[]string{
					cmdPrice,
					notAHexAddress,
				},
				commonFlags...,
			),
			true, invalidAddressHexLengthErr, nil,
		},
		{
			"query global-price - non-numeric timestamp",
			append(
				[]string{
					cmdGlobalPrice,
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
		{cmdMaintenanceRecordsByOperatorAddress, []string{cmdMaintenanceRecordsByOperatorAddress, sample.RandAccAddressHex()}},
		{cmdPrice, []string{cmdPrice, sample.RandAccAddressHex()}},
		{cmdGlobalPrice, []string{cmdGlobalPrice, "0"}},
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

// newEVMRPCStub serves a canned eth_call result so the precompile-backed query
// client can be exercised without a node.
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

// The precompile returns the prices as raw LegacyDec integers (price * 10^18),
// so the EVM query client has to scale them back down when rebuilding the
// response: a store price of 10^22 must render as 10000, not as 10^22.
func (s *CLITestSuite) TestEvmQuerySpStoragePriceScalesPrices() {
	readPrice := sdkmath.LegacyNewDec(100)
	storePrice := sdkmath.LegacyNewDec(10000)

	spABI, err := spp.IStorageProviderMetaData.GetAbi()
	s.Require().NoError(err)

	result, err := spABI.Methods["storageProviderPrice"].Outputs.Pack(spp.SpStoragePrice{
		SpId:          1,
		UpdateTimeSec: big.NewInt(1776174903),
		ReadPrice:     readPrice.BigInt(),
		FreeReadQuota: 100000000,
		StorePrice:    storePrice.BigInt(),
	})
	s.Require().NoError(err)

	srv := newEVMRPCStub(s.T(), result)
	defer srv.Close()

	evmClient, err := ethclient.Dial(srv.URL)
	s.Require().NoError(err)
	defer evmClient.Close()

	resp, err := cli.NewQueryClientEVM(evmClient).QuerySpStoragePrice(
		context.Background(),
		&types.QuerySpStoragePriceRequest{SpAddr: sample.RandAccAddressHex()},
	)
	s.Require().NoError(err)
	s.Require().Equal(readPrice.String(), resp.SpStoragePrice.ReadPrice.String())
	s.Require().Equal(storePrice.String(), resp.SpStoragePrice.StorePrice.String())
}
