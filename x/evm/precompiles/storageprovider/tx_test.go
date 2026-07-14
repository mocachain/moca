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
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
	"github.com/mocachain/moca/v2/x/evm/precompiles/storageprovider"
	evmtypes "github.com/mocachain/moca/v2/x/evm/precompiles/types"
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

// TestUpdateSPPrice_RejectsContractForwarding freezes the current EOA-only guard:
// when the direct caller differs from the tx origin (i.e. a contract is forwarding
// the call), updateSPPrice is rejected before any state is touched. The migration
// to direct-caller semantics must consciously change this behavior, so we pin it here.
func (s *PrecompileTestSuite) TestUpdateSPPrice_RejectsContractForwarding() {
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	input := s.mustPackUpdateSPPriceInput(
		big.NewInt(2000000000000000000), uint64(1024), big.NewInt(1000000000000000000))

	contract := vm.NewContract(caller, storageprovider.GetAddress(), uint256.NewInt(0), 60_000, nil)
	contract.Input = input

	evm := &vm.EVM{}
	evm.SetTxContext(vm.TxContext{Origin: s.address}) // origin != caller

	c := storageprovider.NewPrecompiledContract(s.app.SpKeeper)
	_, err := c.UpdateSPPrice(s.ctx, evm, contract, false)
	s.Require().EqualError(err, "only allow EOA can call this method")
}

// TestUpdateSPPrice_FailureDoesNotMutateState drives updateSPPrice through the EVM
// keeper with NO storage provider registered, so the SP msg server fails with
// "StorageProvider does not exist". It pins that a failed precompile call reverts
// cleanly (Contract.Run snapshot/revert) and writes no SP state.
func (s *PrecompileTestSuite) TestUpdateSPPrice_FailureDoesNotMutateState() {
	s.mustEnableStaticPrecompiles()

	input := s.mustPackUpdateSPPriceInput(
		big.NewInt(2000000000000000000), uint64(1024), big.NewInt(1000000000000000000))

	precompileAddr := storageprovider.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().Error(err)
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	// Native mode surfaces the failure as an EVM revert; the underlying
	// "StorageProvider does not exist" reason is ABI-encoded in the revert data.
	s.Require().Contains(err.Error(), "execution reverted")
	reason, uErr := abi.UnpackRevert(res.Ret)
	s.Require().NoError(uErr)
	s.Require().Contains(reason, "StorageProvider does not exist")

	// The failed EVM call leaves s.ctx's gas meter exhausted, so read post-call
	// state through a fresh context (mirrors the bank baseline). The failed call
	// must not have created any SP (and hence no SP price) state.
	checkCtx := s.app.BaseApp.NewContext(false).
		WithBlockHeader(s.ctx.BlockHeader()).
		WithChainID(s.ctx.ChainID()).
		WithGasMeter(storetypes.NewInfiniteGasMeter()).
		WithBlockGasMeter(storetypes.NewInfiniteGasMeter())
	_, found := s.app.SpKeeper.GetStorageProviderByOperatorAddr(checkCtx, sdk.AccAddress(s.address.Bytes()))
	s.Require().False(found, "failed updateSPPrice must not create SP state")
}

func (s *PrecompileTestSuite) mustEnableStaticPrecompiles() {
	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

func (s *PrecompileTestSuite) mustPackUpdateSPPriceInput(readPrice *big.Int, freeReadQuota uint64, storePrice *big.Int) []byte {
	method := storageprovider.GetAbiMethod(storageprovider.UpdateSPPriceMethodName)
	packedArgs, err := method.Inputs.Pack(readPrice, freeReadQuota, storePrice)
	s.Require().NoError(err)
	return append(append([]byte{}, method.ID...), packedArgs...)
}
