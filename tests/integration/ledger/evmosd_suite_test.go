package ledger_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/tests/integration/ledger/mocks"
	utiltx "github.com/evmos/evmos/v12/testutil/tx"
	"github.com/evmos/evmos/v12/utils"
	"github.com/stretchr/testify/suite"

	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	cosmosledger "github.com/cosmos/cosmos-sdk/crypto/ledger"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clientkeys "github.com/evmos/evmos/v12/client/keys"
	evmoskeyring "github.com/evmos/evmos/v12/crypto/keyring"
	feemarkettypes "github.com/evmos/evmos/v12/x/feemarket/types"

	. "github.com/onsi/ginkgo/v2" //nolint
	. "github.com/onsi/gomega"    //nolint
)

var s *LedgerTestSuite

type LedgerTestSuite struct {
	suite.Suite

	app *app.Evmos
	ctx sdk.Context

	ledger       *mocks.SECP256K1
	accRetriever *mocks.AccountRetriever

	accAddr sdk.AccAddress

	privKey types.PrivKey
	pubKey  types.PubKey
}

func TestLedger(t *testing.T) {
	s = new(LedgerTestSuite)
	suite.Run(t, s)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Evmosd Suite")
}

func (suite *LedgerTestSuite) SetupTest() {
	var (
		err     error
		ethAddr common.Address
	)

	suite.ledger = mocks.NewSECP256K1(s.T())

	ethAddr, s.privKey = utiltx.NewAddrKey()

	s.Require().NoError(err)
	suite.pubKey = s.privKey.PubKey()

	suite.accAddr = sdk.AccAddress(ethAddr.Bytes())
}

func (suite *LedgerTestSuite) SetupEvmosApp() {
	// init app
	chainID := utils.MainnetChainID + "-1"
	suite.app = app.Setup(false, feemarkettypes.DefaultGenesisState(), chainID)
	suite.ctx = suite.app.BaseApp.NewContext(false)
}

func (suite *LedgerTestSuite) NewKeyringAndCtxs(krHome string, input io.Reader, encCfg sdktestutil.TestEncodingConfig) (keyring.Keyring, client.Context, context.Context) {
	kr, err := keyring.New(
		sdk.KeyringServiceName(),
		keyring.BackendTest,
		krHome,
		input,
		encCfg.Codec,
		s.MockKeyringOption(),
	)
	s.Require().NoError(err)
	s.accRetriever = mocks.NewAccountRetriever(s.T())

	// Enable TEXTUAL sign mode for Ledger compatibility
	enabledSignModes := append(tx.DefaultSignModes, signingtypes.SignMode_SIGN_MODE_TEXTUAL)
	txConfigOpts := tx.ConfigOptions{
		EnabledSignModes:           enabledSignModes,
		TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(suite.app.BankKeeper),
	}
	txConfig, err := tx.NewTxConfigWithOptions(
		encCfg.Codec,
		txConfigOpts,
	)
	s.Require().NoError(err)

	initClientCtx := client.Context{}.
		WithCodec(encCfg.Codec).
		// NOTE: cmd.Execute() panics without account retriever
		WithAccountRetriever(s.accRetriever).
		WithTxConfig(txConfig).
		WithLedgerHasProtobuf(true).
		WithUseLedger(true).
		WithKeyring(kr).
		WithClient(mocks.MockCometRPC{Client: rpcclientmock.Client{}}).
		WithChainID(utils.TestnetChainID + "-13")

	srvCtx := server.NewDefaultContext()
	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &initClientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, srvCtx)

	return kr, initClientCtx, ctx
}

func (suite *LedgerTestSuite) evmosAddKeyCmd() *cobra.Command {
	cmd := keys.AddKeyCommand()

	algoFlag := cmd.Flag(flags.FlagKeyType)
	algoFlag.DefValue = string(hd.EthSecp256k1Type)

	err := algoFlag.Value.Set(string(hd.EthSecp256k1Type))
	suite.Require().NoError(err)

	cmd.Flags().AddFlagSet(keys.Commands().PersistentFlags())

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		clientCtx := client.GetClientContextFromCmd(cmd).WithKeyringOptions(keyring.ETHAlgoOption())
		clientCtx, err := client.ReadPersistentCommandFlags(clientCtx, cmd.Flags())
		if err != nil {
			return err
		}
		buf := bufio.NewReader(clientCtx.Input)
		return clientkeys.RunAddCmd(clientCtx, cmd, args, buf)
	}
	return cmd
}

func (suite *LedgerTestSuite) MockKeyringOption() keyring.Option {
	return func(options *keyring.Options) {
		options.SupportedAlgos = evmoskeyring.SupportedAlgorithms
		options.SupportedAlgosLedger = evmoskeyring.SupportedAlgorithmsLedger
		options.LedgerDerivation = func() (cosmosledger.SECP256K1, error) { return suite.ledger, nil }
		options.LedgerCreateKey = evmoskeyring.CreatePubkey
		options.LedgerAppName = evmoskeyring.AppName
		options.LedgerSigSkipDERConv = evmoskeyring.SkipDERConversion
	}
}

func (suite *LedgerTestSuite) FormatFlag(flag string) string {
	return fmt.Sprintf("--%s", flag)
}
