package keeper_test

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint
	. "github.com/onsi/gomega"    //nolint

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/evmos/v12/server/config"
	"github.com/evmos/evmos/v12/testutil"
	utiltx "github.com/evmos/evmos/v12/testutil/tx"
	"github.com/evmos/evmos/v12/utils"
	"github.com/evmos/evmos/v12/x/evm/statedb"
	evm "github.com/evmos/evmos/v12/x/evm/types"

	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/contracts"
	"github.com/evmos/evmos/v12/x/erc20/types"

	"github.com/0xPolygon/polygon-edge/bls"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
)

type KeeperTestSuite struct {
	suite.Suite

	ctx              sdk.Context
	app              *app.Evmos
	queryClientEvm   evm.QueryClient
	queryClient      types.QueryClient
	address          common.Address
	consAddress      sdk.ConsAddress
	clientCtx        client.Context //nolint:unused
	ethSigner        ethtypes.Signer
	priv             cryptotypes.PrivKey
	validator        stakingtypes.Validator
	signer           keyring.Signer
	mintFeeCollector bool
}

var s *KeeperTestSuite

func TestKeeperTestSuite(t *testing.T) {
	s = new(KeeperTestSuite)
	suite.Run(t, s)

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keeper Suite")
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.DoSetupTest(suite.T())
}

func (suite *KeeperTestSuite) DoSetupTest(t require.TestingT) {
	// account key
	priv, err := ethsecp256k1.GenPrivKey()
	require.NoError(t, err)
	suite.priv = priv
	suite.address = common.BytesToAddress(priv.PubKey().Address().Bytes())
	suite.signer = utiltx.NewSigner(priv)

	// consensus key
	privCons, err := ethsecp256k1.GenPrivKey()
	require.NoError(t, err)
	consAddress := sdk.ConsAddress(privCons.PubKey().Address())
	suite.consAddress = consAddress

	// init app with EthSetup which has better test isolation
	chainID := utils.TestnetChainID + "-1"
	suite.app = app.EthSetup(false, nil)

	// Get the context from the committed state after InitChain
	suite.ctx = suite.app.BaseApp.NewContext(false)

	// Initialize context with proper gas meter and chain ID
	header := testutil.NewHeader(1, time.Now().UTC(), chainID, suite.consAddress, nil, nil)
	suite.ctx = suite.ctx.WithBlockHeader(header)
	suite.ctx = suite.ctx.WithBlockGasMeter(storetypes.NewInfiniteGasMeter())
	suite.ctx = suite.ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	suite.ctx = suite.ctx.WithChainID(chainID)

	// Set zero gas config to bypass fee collection during tests
	// This avoids the FeePool collection access that's causing the error
	suite.ctx = suite.ctx.
		WithKVGasConfig(storetypes.GasConfig{}).
		WithTransientKVGasConfig(storetypes.GasConfig{})

		// WORKAROUND: Initialize distribution module to avoid FeePool collection errors
	// Initialize the FeePool directly with a simple approach
	feePool := distrtypes.InitialFeePool()
	err = suite.app.DistrKeeper.FeePool.Set(suite.ctx, feePool)
	require.NoError(t, err)

	// Initialize distribution parameters
	distrParams := distrtypes.DefaultParams()
	err = suite.app.DistrKeeper.Params.Set(suite.ctx, distrParams)
	require.NoError(t, err)

	// Ensure the fee collector module account exists
	feeCollectorAcc := suite.app.AccountKeeper.GetModuleAccount(suite.ctx, authtypes.FeeCollectorName)
	if feeCollectorAcc == nil {
		feeCollectorAcc = authtypes.NewEmptyModuleAccount(authtypes.FeeCollectorName, authtypes.Minter, authtypes.Burner)
		suite.app.AccountKeeper.SetModuleAccount(suite.ctx, feeCollectorAcc)
	}

	// query clients
	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, suite.app.Erc20Keeper)
	suite.queryClient = types.NewQueryClient(queryHelper)

	queryHelperEvm := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	evm.RegisterQueryServer(queryHelperEvm, suite.app.EvmKeeper)
	suite.queryClientEvm = evm.NewQueryClient(queryHelperEvm)

	// bond denom
	stakingParams, err := suite.app.StakingKeeper.GetParams(suite.ctx)
	suite.Require().NoError(err)
	stakingParams.BondDenom = utils.BaseDenom
	err = suite.app.StakingKeeper.SetParams(suite.ctx, stakingParams)
	suite.Require().NoError(err)

	// Clean validator setup - avoid complex logic that can lead to nil pointers
	valAddr := sdk.AccAddress(suite.address.Bytes())
	blsSecretKey, _ := bls.GenerateBlsKey()
	blsPk := blsSecretKey.PublicKey().Marshal()

	// Always create a fresh, clean validator to avoid any state issues
	validator, err := stakingtypes.NewValidator(valAddr.String(), privCons.PubKey(), stakingtypes.Description{}, valAddr.String(), valAddr.String(), valAddr.String(), blsPk)
	require.NoError(t, err)

	// Set proper initial values with safe defaults
	bondAmt := sdk.DefaultPowerReduction
	validator.DelegatorShares = sdkmath.LegacyNewDecFromInt(bondAmt)
	validator.Tokens = bondAmt
	validator.Status = stakingtypes.Bonded

	// Use TestingUpdateValidator to properly initialize
	validator = stakingkeeper.TestingUpdateValidator(suite.app.StakingKeeper, suite.ctx, validator, true)

	valOperatorAddr, err := sdk.AccAddressFromBech32(validator.GetOperator())
	require.NoError(t, err)

	// Set validator by consensus address
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)

	// Create self-delegation with proper token management
	bondCoin := sdk.NewCoin(utils.BaseDenom, bondAmt)
	err = testutil.FundAccount(suite.ctx, suite.app.BankKeeper, valAddr, sdk.NewCoins(bondCoin))
	require.NoError(t, err)

	// Transfer to bonded pool
	bondedPoolAddr := suite.app.AccountKeeper.GetModuleAddress(stakingtypes.BondedPoolName)
	err = suite.app.BankKeeper.SendCoins(suite.ctx, valAddr, bondedPoolAddr, sdk.NewCoins(bondCoin))
	require.NoError(t, err)

	// Create delegation with explicit nil check
	delegationShares := sdkmath.LegacyNewDecFromInt(bondAmt)
	if delegationShares.IsNil() {
		delegationShares = sdkmath.LegacyOneDec()
	}
	delegation := stakingtypes.NewDelegation(valAddr.String(), validator.GetOperator(), delegationShares)

	// Verify delegation shares are not nil before proceeding
	if delegation.Shares.IsNil() {
		delegation.Shares = sdkmath.LegacyOneDec()
	}

	// Call hooks in correct order
	err = suite.app.StakingKeeper.Hooks().AfterValidatorCreated(suite.ctx, valOperatorAddr)
	require.NoError(t, err)

	err = suite.app.StakingKeeper.Hooks().BeforeDelegationCreated(suite.ctx, valAddr, valOperatorAddr)
	require.NoError(t, err)

	err = suite.app.StakingKeeper.SetDelegation(suite.ctx, delegation)
	require.NoError(t, err)

	err = suite.app.StakingKeeper.Hooks().AfterDelegationModified(suite.ctx, valAddr, valOperatorAddr)
	require.NoError(t, err)

	// Final validator update
	err = suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
	require.NoError(t, err)

	// fund signer acc to pay for tx fees
	amt := sdkmath.NewInt(int64(math.Pow10(18) * 2))
	err = testutil.FundAccount(
		suite.ctx,
		suite.app.BankKeeper,
		suite.priv.PubKey().Address().Bytes(),
		sdk.NewCoins(sdk.NewCoin(utils.BaseDenom, amt)),
	)
	suite.Require().NoError(err)

	// TODO change to setup with 1 validator
	validators, err := suite.app.StakingKeeper.GetValidators(suite.ctx, 2)
	suite.Require().NoError(err)
	// set a bonded validator that takes part in consensus
	if validators[0].Status == stakingtypes.Bonded {
		suite.validator = validators[0]
	} else {
		suite.validator = validators[1]
	}

	suite.ethSigner = ethtypes.LatestSignerForChainID(suite.app.EvmKeeper.ChainID())
}

var timeoutHeight = clienttypes.NewHeight(1000, 1000)

func (suite *KeeperTestSuite) StateDB() *statedb.StateDB {
	return statedb.New(suite.ctx, suite.app.EvmKeeper, statedb.NewEmptyTxConfig(common.BytesToHash(suite.ctx.HeaderHash())))
}

func (suite *KeeperTestSuite) MintFeeCollector(coins sdk.Coins) {
	err := suite.app.BankKeeper.MintCoins(suite.ctx, types.ModuleName, coins)
	suite.Require().NoError(err)
	err = suite.app.BankKeeper.SendCoinsFromModuleToModule(suite.ctx, types.ModuleName, authtypes.FeeCollectorName, coins)
	suite.Require().NoError(err)
}

// Commit commits and starts a new block with an updated context.
func (suite *KeeperTestSuite) Commit() {
	suite.CommitAndBeginBlockAfter(time.Hour * 1)
}

// Commit commits a block at a given time. Reminder: At the end of each
// Tendermint Consensus round the following methods are run
//  1. BeginBlock
//  2. DeliverTx
//  3. EndBlock
//  4. Commit
func (suite *KeeperTestSuite) CommitAndBeginBlockAfter(t time.Duration) {
	var err error
	suite.ctx, err = testutil.Commit(suite.ctx, suite.app, t, nil)
	suite.Require().NoError(err)
	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	evm.RegisterQueryServer(queryHelper, suite.app.EvmKeeper)
	suite.queryClientEvm = evm.NewQueryClient(queryHelper)
}

var _ transfertypes.ChannelKeeper = &MockChannelKeeper{}

type MockChannelKeeper struct {
	mock.Mock
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockChannelKeeper) GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool) {
	args := b.Called(mock.Anything, mock.Anything, mock.Anything)
	return args.Get(0).(channeltypes.Channel), true
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockChannelKeeper) HasChannel(ctx sdk.Context, portID, channelID string) bool {
	_ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return true
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockChannelKeeper) GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool) {
	_ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 1, true
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockChannelKeeper) GetAllChannelsWithPortPrefix(ctx sdk.Context, portPrefix string) []channeltypes.IdentifiedChannel {
	return []channeltypes.IdentifiedChannel{}
}

var _ porttypes.ICS4Wrapper = &MockICS4Wrapper{}

type MockICS4Wrapper struct {
	mock.Mock
}

func (b *MockICS4Wrapper) WriteAcknowledgement(_ sdk.Context, _ exported.PacketI, _ exported.Acknowledgement) error {
	return nil
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockICS4Wrapper) GetAppVersion(ctx sdk.Context, portID string, channelID string) (string, bool) {
	return "", false
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockICS4Wrapper) SendPacket(
	ctx sdk.Context,
	sourcePort string,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	data []byte,
) (sequence uint64, err error) {
	// _ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 0, nil
}

// DeployContract deploys the ERC20MinterBurnerDecimalsContract.
func (suite *KeeperTestSuite) DeployContract(name, symbol string, decimals uint8) (common.Address, error) {
	suite.Commit()
	addr, err := testutil.DeployContract(
		suite.ctx,
		suite.app,
		suite.priv,
		suite.queryClientEvm,
		contracts.ERC20MinterBurnerDecimalsContract,
		name, symbol, decimals,
	)
	suite.Commit()
	return addr, err
}

func (suite *KeeperTestSuite) MintERC20Token(contractAddr, from, to common.Address, amount *big.Int) *evm.MsgEthereumTx {
	transferData, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Pack("mint", to, amount)
	suite.Require().NoError(err)
	return suite.sendTx(contractAddr, from, transferData)
}

func (suite *KeeperTestSuite) sendTx(contractAddr, from common.Address, transferData []byte) *evm.MsgEthereumTx {
	ctx := sdk.WrapSDKContext(suite.ctx)
	chainID := suite.app.EvmKeeper.ChainID()

	args, err := json.Marshal(&evm.TransactionArgs{To: &contractAddr, From: &from, Data: (*hexutil.Bytes)(&transferData)})
	suite.Require().NoError(err)
	res, err := suite.queryClientEvm.EstimateGas(ctx, &evm.EthCallRequest{
		Args:   args,
		GasCap: config.DefaultGasCap,
	})
	suite.Require().NoError(err)

	nonce := suite.app.EvmKeeper.GetNonce(suite.ctx, suite.address)

	// Mint the max gas to the FeeCollector to ensure balance in case of refund
	suite.MintFeeCollector(sdk.NewCoins(sdk.NewCoin(evm.DefaultEVMDenom, sdkmath.NewInt(suite.app.FeeMarketKeeper.GetBaseFee(suite.ctx).Int64()*int64(res.Gas)))))

	ethTxParams := evm.EvmTxArgs{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &contractAddr,
		GasLimit:  res.Gas,
		GasFeeCap: suite.app.FeeMarketKeeper.GetBaseFee(suite.ctx),
		GasTipCap: big.NewInt(1),
		Input:     transferData,
		Accesses:  &ethtypes.AccessList{},
	}
	ercTransferTx := evm.NewTx(&ethTxParams)

	ercTransferTx.From = suite.address.Hex()
	err = ercTransferTx.Sign(ethtypes.LatestSignerForChainID(chainID), suite.signer)
	suite.Require().NoError(err)
	rsp, err := suite.app.EvmKeeper.EthereumTx(ctx, ercTransferTx)
	suite.Require().NoError(err)
	suite.Require().Empty(rsp.VmError)
	return ercTransferTx
}
