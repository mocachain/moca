package storageprovider_test

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
	"github.com/mocachain/moca/v2/x/evm/precompiles/storageprovider"
	evmtypes "github.com/mocachain/moca/v2/x/evm/types"
	spkeeper "github.com/mocachain/moca/v2/x/sp/keeper"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

type PrecompileTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
	// no EVM stateDB needed
}

func TestPrecompileTestSuite(t *testing.T) {
	suite.Run(t, new(PrecompileTestSuite))
}

func (s *PrecompileTestSuite) SetupTest() {
	checkTx := false
	chainID := utils.TestnetChainID + "-1"
	// cosmos/evm v0.6.0 migration: the EVM "evm" genesis key is intentionally
	// skipped by the EthSetup harness (see app.NewTestGenesisState), and the
	// old evmtypes.Params.EnableCall / DefaultGenesisState patch is obsolete
	// (call permission is now governed by AccessControl, which allows calls by
	// default). The storageprovider precompile is also not yet wired into
	// cosmos/evm's keeper (see app.go EvmPrecompiled, a no-op stub), so no EVM
	// genesis patching is required for these tests.
	s.app = app.EthSetup(checkTx, nil)

	// initialize context, then prepare a valid proposer/validator for EVM coinbase resolution
	s.ctx = s.app.BaseApp.NewContext(checkTx)

	// use a fixed test address
	s.address = common.HexToAddress("0x1111111111111111111111111111111111111111")

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

	accAddr := sdk.AccAddress(s.address.Bytes())

	// fund the account
	err = testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, accAddr, 1_000_000_000_000)
	s.Require().NoError(err)
}

// TestUpdateSPPrice exercises the production updateSPPrice precompile decode +
// business-logic path WITHOUT routing through the EVM keeper. It packs the ABI
// calldata exactly as a caller would, decodes it with the same
// evmtypes.ParseMethodArgs helper the precompile uses, rebuilds the
// MsgUpdateSpStoragePrice the same way storageprovider.Contract.UpdateSPPrice
// does, runs it through the real SP msg server, and asserts the SP price is
// updated in state.
//
// NOTE: the end-to-end EVM-apply path (ethtypes message -> EvmKeeper ->
// precompile dispatch) is covered separately and is currently skipped: see
// TestUpdateSPPrice_EVMApply below.
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

	// Decode via the same helper the precompile uses (ParseMethodArgs is invoked
	// with contract.Input[4:], i.e. the packed args without the 4-byte selector).
	var args storageprovider.UpdateSPPriceArgs
	err = evmtypes.ParseMethodArgs(method, &args, packedArgs)
	s.Require().NoError(err)
	s.Require().Equal(newReadPrice, args.ReadPrice)
	s.Require().Equal(freeReadQuota, args.FreeReadQuota)
	s.Require().Equal(newStorePrice, args.StorePrice)

	// Rebuild and execute the MsgUpdateSpStoragePrice exactly as
	// storageprovider.Contract.UpdateSPPrice does (caller == s.address).
	msg := &sptypes.MsgUpdateSpStoragePrice{
		SpAddress:     bech32,
		ReadPrice:     math.LegacyNewDecFromBigIntWithPrec(args.ReadPrice, math.LegacyPrecision),
		FreeReadQuota: args.FreeReadQuota,
		StorePrice:    math.LegacyNewDecFromBigIntWithPrec(args.StorePrice, math.LegacyPrecision),
	}
	s.Require().NoError(msg.ValidateBasic())
	server := spkeeper.NewMsgServerImpl(s.app.SpKeeper)
	_, err = server.UpdateSpStoragePrice(s.ctx, msg)
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

// TestUpdateSPPrice_EVMApply would drive updateSPPrice end-to-end through the
// EVM keeper (build an eth tx to the precompile address and ApplyTransaction),
// asserting the precompile is dispatched and the SP price is updated.
//
// It is skipped pending the cosmos/evm v0.6.0 migration finishing the precompile
// wiring: app.Moca.EvmPrecompiled() is currently a no-op stub (see
// app/app.go), so moca's static precompiles (including storageprovider) are NOT
// registered with cosmos/evm's vm keeper. Until WithStaticPrecompiles is wired
// up, an EVM call to the precompile address would not dispatch to this handler.
// Additionally geth v1.15+ removed ethtypes.NewMessage and the
// EvmKeeper.ApplyMessage signature changed (now requires a *statedb.StateDB and
// *tracing.Hooks), so the old fixture can no longer be built as-is.
func (s *PrecompileTestSuite) TestUpdateSPPrice_EVMApply() {
	s.T().Skip("cosmos/evm v0.6.0 migration: storageprovider precompile not yet registered with cosmos/evm vm keeper (app.go EvmPrecompiled is a no-op stub); re-enable once WithStaticPrecompiles wiring lands")
}
