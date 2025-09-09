package keeper_test

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/evmos/evmos/v12/utils"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	simutils "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	servercfg "github.com/evmos/evmos/v12/server/config"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/encoding"
	"github.com/evmos/evmos/v12/testutil"
	utiltx "github.com/evmos/evmos/v12/testutil/tx"
	evmostypes "github.com/evmos/evmos/v12/types"
	evmtypes "github.com/evmos/evmos/v12/x/evm/types"
	"github.com/evmos/evmos/v12/x/feemarket/types"

	"github.com/stretchr/testify/require"

	storetypes "cosmossdk.io/store/types"
	"github.com/0xPolygon/polygon-edge/bls"
	abci "github.com/cometbft/cometbft/abci/types"
)

func (suite *KeeperTestSuite) SetupApp(checkTx bool, chainID string) {
	t := suite.T()
	// account key
	priv, err := ethsecp256k1.GenPrivKey()
	require.NoError(t, err)
	suite.address = common.BytesToAddress(priv.PubKey().Address().Bytes())
	suite.signer = utiltx.NewSigner(priv)

	// consensus key
	priv, err = ethsecp256k1.GenPrivKey()
	require.NoError(t, err)
	suite.consAddress = sdk.ConsAddress(priv.PubKey().Address())

	// create base context first, then set header, gas meter, and chain ID
	suite.ctx = suite.app.BaseApp.NewContext(checkTx)
	header := testutil.NewHeader(1, time.Now().UTC(), chainID, suite.consAddress, nil, nil)
	suite.ctx = suite.ctx.WithBlockHeader(header)
	suite.ctx = suite.ctx.WithBlockGasMeter(storetypes.NewInfiniteGasMeter())
	suite.ctx = suite.ctx.WithChainID(chainID)

	// initialize first block (begin block) so tx delivery can finalize it later
	_, err = suite.app.BeginBlocker(suite.ctx)
	require.NoError(t, err)

	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, suite.app.FeeMarketKeeper)
	suite.queryClient = types.NewQueryClient(queryHelper)

	acc := &evmostypes.EthAccount{
		BaseAccount: authtypes.NewBaseAccount(sdk.AccAddress(suite.address.Bytes()), nil, 0, 0),
		CodeHash:    common.BytesToHash(crypto.Keccak256(nil)).String(),
	}
	acc = suite.app.AccountKeeper.NewAccount(suite.ctx, acc).(*evmostypes.EthAccount)

	suite.app.AccountKeeper.SetAccount(suite.ctx, acc)

	valAddr := sdk.AccAddress(suite.address.Bytes())
	blsSecretKey, _ := bls.GenerateBlsKey()
	blsPk := blsSecretKey.PublicKey().Marshal()
	validator, err := stakingtypes.NewValidator(valAddr.String(), priv.PubKey(), stakingtypes.Description{}, valAddr.String(), valAddr.String(), valAddr.String(), blsPk)
	require.NoError(t, err)
	validator = stakingkeeper.TestingUpdateValidator(suite.app.StakingKeeper, suite.ctx, validator, true)
	err = suite.app.StakingKeeper.Hooks().AfterValidatorCreated(suite.ctx, valAddr)
	require.NoError(t, err)

	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = utils.BaseDenom
	err = suite.app.StakingKeeper.SetParams(suite.ctx, stakingParams)
	require.NoError(t, err)

	encodingConfig := encoding.MakeConfig()
	suite.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)
	suite.ethSigner = ethtypes.LatestSignerForChainID(suite.app.EvmKeeper.ChainID())
	suite.appCodec = encodingConfig.Codec
	suite.denom = evmtypes.DefaultEVMDenom
}

// Commit commits and starts a new block with an updated context.
func (suite *KeeperTestSuite) Commit() {
	suite.CommitAfter(time.Second * 0)
}

// Commit commits a block at a given time.
func (suite *KeeperTestSuite) CommitAfter(t time.Duration) {
	// finalize current block
	header := suite.ctx.BlockHeader()
	_, err := suite.app.BaseApp.FinalizeBlock(&abci.RequestFinalizeBlock{Height: header.Height, ProposerAddress: header.ProposerAddress})
	suite.Require().NoError(err)

	// commit app state
	_, err = suite.app.Commit()
	suite.Require().NoError(err)

	// advance header
	header.Height++
	header.Time = header.Time.Add(t)
	header.AppHash = suite.app.LastCommitID().Hash
	suite.ctx = suite.ctx.WithBlockHeader(header)

	// begin next block
	_, err = suite.app.BeginBlocker(suite.ctx)
	suite.Require().NoError(err)

	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, suite.app.FeeMarketKeeper)
	suite.queryClient = types.NewQueryClient(queryHelper)
}

// setupTestWithContext sets up a test chain with an example Cosmos send msg,
// given a local (validator config) and a global (feemarket param) minGasPrice
//
//nolint:unparam
func setupTestWithContext(chainID, valMinGasPrice string, minGasPrice sdkmath.LegacyDec, baseFee sdkmath.Int) (*ethsecp256k1.PrivKey, banktypes.MsgSend) {
	privKey, msg := setupTest(valMinGasPrice+evmtypes.DefaultEVMDenom, chainID, minGasPrice, baseFee)
	return privKey, msg
}

func setupTest(localMinGasPrices, chainID string, minGasPrice sdkmath.LegacyDec, baseFee sdkmath.Int) (*ethsecp256k1.PrivKey, banktypes.MsgSend) {
	setupChain(localMinGasPrices, chainID, minGasPrice, baseFee)

	address, privKey := utiltx.NewAccAddressAndKey()
	amount, ok := sdkmath.NewIntFromString("10000000000000000000")
	s.Require().True(ok)
	initBalance := sdk.Coins{sdk.Coin{
		Denom:  s.denom,
		Amount: amount,
	}}
	err := testutil.FundAccount(s.ctx, s.app.BankKeeper, address, initBalance)
	s.Require().NoError(err)

	msg := banktypes.MsgSend{
		FromAddress: address.String(),
		ToAddress:   address.String(),
		Amount: sdk.Coins{sdk.Coin{
			Denom:  s.denom,
			Amount: sdkmath.NewInt(10000),
		}},
	}
	s.Commit()
	return privKey, msg
}

func setupChain(localMinGasPricesStr string, chainID string, minGasPrice sdkmath.LegacyDec, baseFee sdkmath.Int) {
	// Initialize the app, so we can use SetMinGasPrices to set the
	// validator-specific min-gas-prices setting
	db := dbm.NewMemDB()
	newapp := app.NewEvmos(
		log.NewNopLogger(),
		db,
		nil,
		true,
		map[int64]bool{},
		app.DefaultNodeHome,
		5,
		servercfg.NewDefaultAppConfig(evmostypes.AttoEvmos),
		simutils.NewAppOptionsWithFlagHome(app.DefaultNodeHome),
		baseapp.SetChainID(chainID),
		baseapp.SetMinGasPrices(localMinGasPricesStr),
	)

	genesisState := app.NewTestGenesisState(newapp.AppCodec())
	fmGenesis := types.DefaultGenesisState()
	fmGenesis.Params.MinGasPrice = minGasPrice
	fmGenesis.Params.BaseFee = sdkmath.NewIntFromBigInt(baseFee.BigInt())
	genesisState[types.ModuleName] = newapp.AppCodec().MustMarshalJSON(fmGenesis)

	stateBytes, err := json.MarshalIndent(genesisState, "", "  ")
	s.Require().NoError(err)

	// Initialize the chain
	newapp.InitChain(
		&abci.RequestInitChain{
			ChainId:         chainID,
			Validators:      []abci.ValidatorUpdate{},
			AppStateBytes:   stateBytes,
			ConsensusParams: app.DefaultConsensusParams,
		},
	)

	s.app = newapp
	s.SetupApp(false, chainID)
}

func getNonce(addressBytes []byte) uint64 {
	return s.app.EvmKeeper.GetNonce(
		s.ctx,
		common.BytesToAddress(addressBytes),
	)
}

func buildEthTx(
	priv *ethsecp256k1.PrivKey,
	to *common.Address,
	gasPrice *big.Int,
	gasFeeCap *big.Int,
	gasTipCap *big.Int,
	accesses *ethtypes.AccessList,
) *evmtypes.MsgEthereumTx {
	chainID := s.app.EvmKeeper.ChainID()
	from := common.BytesToAddress(priv.PubKey().Address().Bytes())
	nonce := getNonce(from.Bytes())
	data := make([]byte, 0)
	gasLimit := uint64(100000)
	ethTxParams := &evmtypes.EvmTxArgs{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        to,
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Input:     data,
		Accesses:  accesses,
	}
	msgEthereumTx := evmtypes.NewTx(ethTxParams)
	msgEthereumTx.From = from.String()
	return msgEthereumTx
}
