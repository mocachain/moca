package storageprovider_test

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/math"
	"cosmossdk.io/simapp"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/evmos/evmos/v12/app"
	"github.com/evmos/evmos/v12/server/config"
	"github.com/evmos/evmos/v12/testutil"
	utiltx "github.com/evmos/evmos/v12/testutil/tx"
	"github.com/evmos/evmos/v12/utils"
	"github.com/evmos/evmos/v12/x/evm/precompiles/storageprovider"
	sptypes "github.com/evmos/evmos/v12/x/sp/types"
)

type PrecompileTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Evmos
	address common.Address
	// no EVM stateDB needed
}

func TestPrecompileTestSuite(t *testing.T) {
	suite.Run(t, new(PrecompileTestSuite))
}

func (s *PrecompileTestSuite) SetupTest() {
	checkTx := false
	chainID := utils.TestnetChainID + "-1"
	s.app = app.EthSetup(checkTx, func(app *app.Evmos, genesis simapp.GenesisState) simapp.GenesisState { return genesis })

	// initialize context, then prepare a valid proposer/validator for EVM coinbase resolution
	s.ctx = s.app.BaseApp.NewContext(checkTx)
	// prepare a valid proposer/validator for EVM coinbase resolution
	valConsAddr, privkey := utiltx.NewAddrKey()
	pkAny, err := codectypes.NewAnyWithValue(privkey.PubKey())
	s.Require().NoError(err)
	validator := stakingtypes.Validator{
		OperatorAddress: sdk.AccAddress(s.address.Bytes()).String(),
		ConsensusPubkey: pkAny,
	}
	s.app.StakingKeeper.SetValidator(s.ctx, validator)
	err = s.app.StakingKeeper.SetValidatorByConsAddr(s.ctx, validator)
	s.Require().NoError(err)

	safeTime := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	header := testutil.NewHeader(1, safeTime, chainID, sdk.ConsAddress(valConsAddr.Bytes()), tmhash.Sum([]byte("app")), tmhash.Sum([]byte("validators")))
	s.ctx = s.ctx.WithBlockHeader(header).WithChainID(chainID)

	// use a fixed test address
	s.address = common.HexToAddress("0x1111111111111111111111111111111111111111")
	accAddr := sdk.AccAddress(s.address.Bytes())

	// fund the account
	err = testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, accAddr, 1_000_000_000_000)
	s.Require().NoError(err)

}

func (s *PrecompileTestSuite) TestUpdateSPPrice() {
	// Create a storage provider (use bech32 addresses)
	bech32 := sdk.AccAddress(s.address.Bytes()).String()
	sp := sptypes.StorageProvider{
		OperatorAddress: bech32,
		FundingAddress:  bech32,
		SealAddress:     bech32,
		ApprovalAddress: bech32,
		GcAddress:       bech32,
		Status:          sptypes.STATUS_IN_SERVICE,
		TotalDeposit:    math.NewInt(1000),
	}
	s.app.SpKeeper.SetStorageProvider(s.ctx, &sp)
	s.app.SpKeeper.SetStorageProviderByOperatorAddr(s.ctx, &sp)

	// Prepare ABI-encoded calldata for updateSPPrice(readPrice, freeReadQuota, storePrice)
	newReadPrice := big.NewInt(2000000000000000000)  // 2e18
	newStorePrice := big.NewInt(1000000000000000000) // 1e18
	freeReadQuota := uint64(1024)

	method := storageprovider.GetAbiMethod(storageprovider.UpdateSPPriceMethodName)
	packedArgs, err := method.Inputs.Pack(newReadPrice, freeReadQuota, newStorePrice)
	s.Require().NoError(err)
	input := append(method.ID, packedArgs...)
	precompileAddr := storageprovider.GetAddress()

	// Build an EVM message and apply via EvmKeeper to exercise ABI decoding path
	nonce := s.app.EvmKeeper.GetNonce(s.ctx, s.address)
	gasLimit := config.DefaultGasCap
	msg := ethtypes.NewMessage(
		s.address,             // from (0x address)
		&precompileAddr,       // to precompile
		nonce,                 // nonce
		big.NewInt(0),         // value
		gasLimit,              // gas limit
		big.NewInt(0),         // gasFeeCap
		big.NewInt(0),         // gasTipCap
		big.NewInt(0),         // gasPrice
		input,                 // data
		ethtypes.AccessList{}, // access list
		false,                 // not fake, commit state
	)
	_, err = s.app.EvmKeeper.ApplyMessage(s.ctx, msg, nil, true)
	s.Require().NoError(err)

	// Verify the price has been updated in SP storage price
	updatedSP, found := s.app.SpKeeper.GetStorageProviderByOperatorAddr(s.ctx, sdk.AccAddress(s.address.Bytes()))
	s.Require().True(found, "storage provider should be found")

	spPrice, ok := s.app.SpKeeper.GetSpStoragePrice(s.ctx, updatedSP.Id)
	s.Require().True(ok, "storage provider price should exist")

	expectedReadPrice := math.LegacyNewDecFromBigIntWithPrec(newReadPrice, math.LegacyPrecision)
	expectedStorePrice := math.LegacyNewDecFromBigIntWithPrec(newStorePrice, math.LegacyPrecision)

	s.Require().Equal(expectedReadPrice.String(), spPrice.ReadPrice.String(), "read price should be updated")
	s.Require().Equal(expectedStorePrice.String(), spPrice.StorePrice.String(), "store price should be updated")
	s.Require().Equal(freeReadQuota, spPrice.FreeReadQuota, "free read quota should be updated")
}
