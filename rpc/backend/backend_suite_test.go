package backend

import (
	"bufio"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"

	dbm "github.com/cosmos/cosmos-db"

	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/suite"

	evmmodule "github.com/cosmos/evm/x/vm"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	cmdcfg "github.com/mocachain/moca/v2/cmd/config"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/indexer"
	"github.com/mocachain/moca/v2/rpc/backend/mocks"
	rpctypes "github.com/mocachain/moca/v2/rpc/types"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
)

type BackendTestSuite struct {
	suite.Suite

	backend *Backend
	from    common.Address
	acc     sdk.AccAddress
	signer  keyring.Signer
}

func TestBackendTestSuite(t *testing.T) {
	suite.Run(t, new(BackendTestSuite))
}

const ChainID = utils.TestnetChainID + "-1"

// setEVMChainConfigOnce seeds the global EVM chain config that ChainConfig()
// reads via evmtypes.GetEthChainConfig(). Unlike the keeper/app test suites,
// this suite never constructs a full app (it wires a Backend with mocked query
// clients), so evmkeeper.NewKeeper -> evmtypes.SetChainConfig never runs and the
// global config stays nil, panicking on first ChainConfig() call. Seed it once
// here with the cosmos/evm default, matching what NewKeeper does internally.
var setEVMChainConfigOnce sync.Once

func seedEVMChainConfig() {
	setEVMChainConfigOnce.Do(func() {
		// Seed the EVM coin info (denom/decimals) that BuildTx reads via
		// evmtypes.GetEVMCoinExtendedDenom; mirrors app.go's one-time
		// evmmodule.SetGlobalConfigVariables call.
		evmmodule.SetGlobalConfigVariables(evmtypes.EvmCoinInfo{
			Denom:         cmdcfg.BaseDenom,
			ExtendedDenom: cmdcfg.BaseDenom,
			DisplayDenom:  cmdcfg.DisplayDenom,
			Decimals:      uint32(evmtypes.EighteenDecimals),
		})
		// Seed the global chain config that ChainConfig() reads via
		// evmtypes.GetEthChainConfig; mirrors what evmkeeper.NewKeeper does.
		if err := evmtypes.SetChainConfig(evmtypes.DefaultChainConfig(evmtypes.DefaultEVMChainID)); err != nil {
			panic(err)
		}
	})
}

// SetupTest is executed before every BackendTestSuite test
func (suite *BackendTestSuite) SetupTest() {
	seedEVMChainConfig()

	ctx := server.NewDefaultContext()
	ctx.Viper.Set("telemetry.global-labels", []interface{}{})

	baseDir := suite.T().TempDir()
	nodeDirName := "node"
	clientDir := filepath.Join(baseDir, nodeDirName, "mocacli")
	keyRing, err := suite.generateTestKeyring(clientDir)
	if err != nil {
		panic(err)
	}

	// Create Account with set sequence
	suite.acc = sdk.AccAddress(utiltx.GenerateAddress().Bytes())
	accounts := map[string]client.TestAccount{}
	accounts[suite.acc.String()] = client.TestAccount{
		Address: suite.acc,
		Num:     uint64(1),
		Seq:     uint64(1),
	}

	from, priv := utiltx.NewAddrKey()
	suite.from = from
	suite.signer = utiltx.NewSigner(priv)
	suite.Require().NoError(err)

	encodingConfig := encoding.MakeConfig()
	// Register x/evm msg types so TxConfig.TxDecoder can resolve
	// /ethermint.evm.v1.MsgEthereumTx when decoding block txs in tests.
	// Outside tests this is done by app.BasicModuleManager.RegisterInterfaces.
	evmtypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	clientCtx := client.Context{}.WithChainID(ChainID).
		WithHeight(1).
		WithTxConfig(encodingConfig.TxConfig).
		WithCodec(encodingConfig.Codec).
		WithKeyringDir(clientDir).
		WithKeyring(keyRing).
		WithAccountRetriever(client.TestAccountRetriever{Accounts: accounts})

	allowUnprotectedTxs := false
	idxer := indexer.NewKVIndexer(dbm.NewMemDB(), ctx.Logger, clientCtx)

	suite.backend = NewBackend(ctx, ctx.Logger, clientCtx, allowUnprotectedTxs, idxer)
	suite.backend.queryClient.QueryClient = mocks.NewEVMQueryClient(suite.T())
	suite.backend.clientCtx.Client = mocks.NewClient(suite.T())
	suite.backend.queryClient.FeeMarket = mocks.NewFeeMarketQueryClient(suite.T())
	suite.backend.ctx = rpctypes.ContextWithHeight(1)
}

// buildEthereumTx returns an example legacy Ethereum transaction
func (suite *BackendTestSuite) buildEthereumTx() (*evmtypes.MsgEthereumTx, []byte) {
	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:  suite.backend.chainID,
		Nonce:    uint64(0),
		To:       &common.Address{},
		Amount:   big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
	}
	msgEthereumTx := evmtypes.NewTx(&ethTxParams)

	// A valid msg should have empty `From`
	msgEthereumTx.From = suite.from.Bytes()

	txBuilder := suite.backend.clientCtx.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgEthereumTx)
	suite.Require().NoError(err)

	bz, err := suite.backend.clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	suite.Require().NoError(err)

	// Return the msg as it round-trips through encode/decode rather than the
	// in-memory original. In cosmos/evm v0.6.0 the decoded MsgEthereumTx carries
	// a freshly-built *ethtypes.Transaction whose empty calldata is normalized to
	// an empty (non-nil) slice and whose cached creation timestamp differs from
	// the original. The backend always serves the decoded form, so expectations
	// derived from this msg must use the decoded form too for deep equality.
	decoded, err := suite.backend.clientCtx.TxConfig.TxDecoder()(bz)
	suite.Require().NoError(err)
	decodedMsg, ok := decoded.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
	suite.Require().True(ok)
	return decodedMsg, bz
}

// buildFormattedBlock returns a formatted block for testing
func (suite *BackendTestSuite) buildFormattedBlock(
	blockRes *tmrpctypes.ResultBlockResults,
	resBlock *tmrpctypes.ResultBlock,
	fullTx bool,
	tx *evmtypes.MsgEthereumTx,
	validator sdk.AccAddress,
	baseFee *big.Int,
) map[string]interface{} {
	header := resBlock.Block.Header
	gasLimit := int64(^uint32(0)) // for `MaxGas = -1` (DefaultConsensusParams)
	gasUsed := new(big.Int).SetUint64(uint64(blockRes.TxsResults[0].GasUsed))

	root := common.Hash{}.Bytes()
	receipt := ethtypes.NewReceipt(root, false, gasUsed.Uint64())
	bloom := ethtypes.CreateBloom(receipt)

	ethRPCTxs := []interface{}{}
	if tx != nil {
		if fullTx {
			rpcTx, err := rpctypes.NewRPCTransaction(
				tx.AsTransaction(),
				common.BytesToHash(header.Hash()),
				uint64(header.Height),
				uint64(0),
				baseFee,
				suite.backend.chainID,
			)
			suite.Require().NoError(err)
			ethRPCTxs = []interface{}{rpcTx}
		} else {
			ethRPCTxs = []interface{}{tx.Hash()}
		}
	}

	return rpctypes.FormatBlock(
		header,
		resBlock.Block.Size(),
		gasLimit,
		gasUsed,
		ethRPCTxs,
		bloom,
		common.BytesToAddress(validator.Bytes()),
		baseFee,
	)
}

func (suite *BackendTestSuite) generateTestKeyring(clientDir string) (keyring.Keyring, error) {
	buf := bufio.NewReader(os.Stdin)
	encCfg := encoding.MakeConfig()
	return keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, clientDir, buf, encCfg.Codec, []keyring.Option{keyring.ETHAlgoOption()}...)
}

func (suite *BackendTestSuite) signAndEncodeEthTx(msgEthereumTx *evmtypes.MsgEthereumTx) []byte {
	from, priv := utiltx.NewAddrKey()
	signer := utiltx.NewSigner(priv)

	queryClient := suite.backend.queryClient.QueryClient.(*mocks.EVMQueryClient)
	RegisterParamsWithoutHeader(queryClient, 1)

	ethSigner := ethtypes.LatestSigner(suite.backend.ChainConfig())
	msgEthereumTx.From = from.Bytes()
	err := msgEthereumTx.Sign(ethSigner, signer)
	suite.Require().NoError(err)

	tx, err := msgEthereumTx.BuildTx(suite.backend.clientCtx.TxConfig.NewTxBuilder(), utils.BaseDenom)
	suite.Require().NoError(err)

	txEncoder := suite.backend.clientCtx.TxConfig.TxEncoder()
	txBz, err := txEncoder(tx)
	suite.Require().NoError(err)

	return txBz
}
