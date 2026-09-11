package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/stretchr/testify/suite"

	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/sdk/client/test"
	"github.com/mocachain/moca/v2/x/virtualgroup/client/cli"
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

func (s *CLITestSuite) TestTxCmd() {
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagFrom, s.clientCtx.GetFromAddress().String()),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin("amoca", math.NewInt(10))).String()),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagOutput, "json"),
	}

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectErrMsg string
	}{
		{
			"settle happy path",
			append([]string{"settle", "1", "1,2"}, commonFlags...),
			false, "",
		},
		{
			"non-numeric family id",
			append([]string{"settle", "abc", "1"}, commonFlags...),
			true, "invalid GVG family id",
		},
		{
			// "--" stops pflag from treating the leading "-" of the
			// negative family id as an (unknown) shorthand flag.
			"negative family id",
			append(append([]string{"settle"}, commonFlags...), "--", "-1", "1"),
			true, "invalid GVG family id",
		},
		{
			"non-numeric entry in gvg id list",
			append([]string{"settle", "1", "1,x"}, commonFlags...),
			true, "invalid GVG id",
		},
		{
			// family id 0 (NoSpecifiedFamilyID) requires between 1 and 10
			// gvg ids; 11 trips MsgSettle.ValidateBasic's count check.
			"too many gvg ids when family unspecified",
			append([]string{"settle", "0", "1,2,3,4,5,6,7,8,9,10,11"}, commonFlags...),
			true, "the count of global virtual group ids is invalid",
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
