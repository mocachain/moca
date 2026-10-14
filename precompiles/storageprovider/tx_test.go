package storageprovider_test

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtestutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/precompiles/storageprovider"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
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
	// default). Tests that drive the EVM keeper (TestUpdateSPPrice_EVMApply) set
	// the evm denom + active static-precompile allowlist on the per-test ctx
	// themselves, so no EVM genesis patching is required here.
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
	header := evmtestutil.NewHeader(1, safeTime, chainID, sdk.ConsAddress(valConsAddr.Bytes()), tmhash.Sum([]byte("app")), tmhash.Sum([]byte("validators")))
	s.ctx = s.ctx.WithBlockHeader(header).WithChainID(chainID)

	accAddr := sdk.AccAddress(s.address.Bytes())

	// fund the account
	err = testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, accAddr, 1_000_000_000_000)
	s.Require().NoError(err)

	// EthSetup leaves sp params zero-valued, so install defaults for the max_sp_price check.
	s.Require().NoError(s.app.SpKeeper.SetParams(s.ctx, sptypes.DefaultParams()))
}

// TestUpdateSPPrice exercises the production updateSPPrice precompile decode +
// business-logic path WITHOUT routing through the EVM keeper. It packs the ABI
// calldata exactly as a caller would, decodes it with the same
// method.Inputs.Copy mechanism the precompile uses (via cmn.SetupABI), rebuilds
// the MsgUpdateSpStoragePrice the same way storageprovider.Precompile.UpdateSPPrice
// does, runs it through the real SP msg server, and asserts the SP price is
// updated in state.
//
// NOTE: the end-to-end EVM-apply path (calldata -> EvmKeeper -> precompile
// dispatch) is covered separately by TestUpdateSPPrice_EVMApply below.
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

	// Decode via the same mechanism the precompile uses (cmn.SetupABI unpacks the
	// packed args — contract.Input[4:], i.e. without the 4-byte selector — and copies
	// them into the arg struct via method.Inputs.Copy).
	unpacked, err := method.Inputs.Unpack(packedArgs)
	s.Require().NoError(err)
	var args storageprovider.UpdateSPPriceArgs
	err = method.Inputs.Copy(&args, unpacked)
	s.Require().NoError(err)
	s.Require().Equal(newReadPrice, args.ReadPrice)
	s.Require().Equal(freeReadQuota, args.FreeReadQuota)
	s.Require().Equal(newStorePrice, args.StorePrice)

	// Rebuild and execute the MsgUpdateSpStoragePrice exactly as
	// storageprovider.Precompile.UpdateSPPrice does (caller == s.address).
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

// TestUpdateSPPrice_EVMApply drives updateSPPrice end-to-end THROUGH the EVM
// keeper: it builds the ABI calldata, sends it to the storageprovider precompile
// address via CallEVMWithData, and asserts the precompile was dispatched (the SP
// price actually changed in state). Unlike TestUpdateSPPrice — which exercises
// only the decode + msg-server path — this proves cosmos/evm's static-precompile
// dispatch reaches moca's handler after the v0.6.0 migration (app.EvmPrecompiled
// -> WithStaticPrecompiles, plus the address listed in Params.ActiveStaticPrecompiles).
func (s *PrecompileTestSuite) TestUpdateSPPrice_EVMApply() {
	// Create the storage provider whose price the precompile will update.
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

	// The EthSetup harness skips the EVM genesis, so set the params this dispatch
	// path needs: the evm denom and the active static-precompile allowlist (a
	// static precompile is only dispatched when its address is listed here).
	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))

	// Pack updateSPPrice(readPrice, freeReadQuota, storePrice) WITH the 4-byte
	// selector — the EVM forwards the full input and the precompile slices [4:].
	newReadPrice := big.NewInt(2000000000000000000)  // 2e18
	newStorePrice := big.NewInt(1000000000000000000) // 1e18
	freeReadQuota := uint64(1024)
	method := storageprovider.GetAbiMethod(storageprovider.UpdateSPPriceMethodName)
	packedArgs, err := method.Inputs.Pack(newReadPrice, freeReadQuota, newStorePrice)
	s.Require().NoError(err)
	input := append(append([]byte{}, method.ID...), packedArgs...)

	// Execute through the EVM keeper so static-precompile dispatch runs for real.
	precompileAddr := storageprovider.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	// The precompile must have mutated SP state — this is what bypassing the EVM
	// (TestUpdateSPPrice) cannot prove.
	updatedSP, found := s.app.SpKeeper.GetStorageProviderByOperatorAddr(s.ctx, sdk.AccAddress(s.address.Bytes()))
	s.Require().True(found, "storage provider should be found")
	spPrice, ok := s.app.SpKeeper.GetSpStoragePrice(s.ctx, updatedSP.Id)
	s.Require().True(ok, "storage provider price should exist")
	s.Require().Equal(math.LegacyNewDecFromBigIntWithPrec(newReadPrice, math.LegacyPrecision).String(), spPrice.ReadPrice.String(), "read price updated via EVM dispatch")
	s.Require().Equal(math.LegacyNewDecFromBigIntWithPrec(newStorePrice, math.LegacyPrecision).String(), spPrice.StorePrice.String(), "store price updated via EVM dispatch")
	s.Require().Equal(freeReadQuota, spPrice.FreeReadQuota, "free read quota updated via EVM dispatch")
}

// TestUpdateSPPrice_EVMApplyAboveMax sends updateSPPrice through the EVM keeper
// with a store price one step above the max_sp_price param and checks the call
// reverts without storing a price, then that a price equal to it goes through.
func (s *PrecompileTestSuite) TestUpdateSPPrice_EVMApplyAboveMax() {
	spAddr := sdk.AccAddress(s.address.Bytes()).String()
	sp := sptypes.StorageProvider{
		OperatorAddress: spAddr,
		FundingAddress:  spAddr,
		SealAddress:     spAddr,
		ApprovalAddress: spAddr,
		GcAddress:       spAddr,
		Status:          sptypes.STATUS_IN_SERVICE,
		TotalDeposit:    math.NewInt(1000),
	}
	s.app.SpKeeper.SetStorageProvider(s.ctx, &sp)
	s.app.SpKeeper.SetStorageProviderByOperatorAddr(s.ctx, &sp)

	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))

	// Lower max_sp_price well below its default, so only a check against the param rejects the call.
	params := s.app.SpKeeper.GetParams(s.ctx)
	params.MaxSpPrice = math.LegacyNewDec(10)
	s.Require().NoError(s.app.SpKeeper.SetParams(s.ctx, params))

	// The precompile takes prices as LegacyDec's raw 18-decimal integer.
	storePrice := new(big.Int).Add(params.MaxSpPrice.BigInt(), big.NewInt(1))
	method := storageprovider.GetAbiMethod(storageprovider.UpdateSPPriceMethodName)
	packedArgs, err := method.Inputs.Pack(big.NewInt(1), uint64(0), storePrice)
	s.Require().NoError(err)
	input := append(append([]byte{}, method.ID...), packedArgs...)

	precompileAddr := storageprovider.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().True(err != nil || res.Failed(), "updateSPPrice above max_sp_price should revert")
	s.Require().NotNil(res)
	reason, err := abi.UnpackRevert(res.Ret)
	s.Require().NoError(err)
	s.Require().Contains(reason, sptypes.ErrStorageProviderPriceExceedsMax.Error())

	// The reverted call leaves the infinite gas meter fully consumed, so reset it before reading state.
	s.ctx = s.ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	_, found := s.app.SpKeeper.GetSpStoragePrice(s.ctx, sp.Id)
	s.Require().False(found, "no price should be stored")

	packedArgs, err = method.Inputs.Pack(big.NewInt(1), uint64(0), params.MaxSpPrice.BigInt())
	s.Require().NoError(err)
	input = append(append([]byte{}, method.ID...), packedArgs...)
	stateDB = statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err = s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)
	spPrice, found := s.app.SpKeeper.GetSpStoragePrice(s.ctx, sp.Id)
	s.Require().True(found, "storage provider price should exist")
	s.Require().True(params.MaxSpPrice.Equal(spPrice.StorePrice), "store price equal to max_sp_price is stored")
}
