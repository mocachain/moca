package cli_test

import (
	"bytes"
	"io"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/stretchr/testify/suite"

	"github.com/evmos/evmos/v12/encoding"
	"github.com/evmos/evmos/v12/sdk/client/test"
	"github.com/evmos/evmos/v12/x/storage/client/cli"
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
		// We do not expect success because network/EVM context is not configured in unit tests.
		s.Require().Error(err)
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
		// As above, success is not required; ensure no panic from slice construction.
		s.Require().Error(err)
	})
}
