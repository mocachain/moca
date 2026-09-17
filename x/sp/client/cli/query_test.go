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
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/ethclient"

	spp "github.com/mocachain/moca/v2/precompiles/storageprovider"
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
