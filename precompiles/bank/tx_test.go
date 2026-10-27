package bank_test

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtestutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/precompiles/bank"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
)

type PrecompileTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestPrecompileTestSuite(t *testing.T) {
	suite.Run(t, new(PrecompileTestSuite))
}

func (s *PrecompileTestSuite) SetupTest() {
	checkTx := false
	chainID := utils.TestnetChainID + "-1"

	s.app = app.EthSetup(checkTx, nil)
	s.ctx = s.app.NewContext(checkTx)
	s.address = common.HexToAddress("0x1111111111111111111111111111111111111111")

	valConsAddr, privkey := utiltx.NewAddrKey()
	pkAny, err := codectypes.NewAnyWithValue(privkey.PubKey())
	s.Require().NoError(err)
	validator := stakingtypes.Validator{
		OperatorAddress: sdk.AccAddress(s.address.Bytes()).String(),
		ConsensusPubkey: pkAny,
	}
	err = s.app.StakingKeeper.SetValidator(s.ctx, validator)
	s.Require().NoError(err)
	err = s.app.StakingKeeper.SetValidatorByConsAddr(s.ctx, validator)
	s.Require().NoError(err)

	safeTime := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	header := evmtestutil.NewHeader(1, safeTime, chainID, sdk.ConsAddress(valConsAddr.Bytes()), tmhash.Sum([]byte("app")), tmhash.Sum([]byte("validators")))
	s.ctx = s.ctx.WithBlockHeader(header).WithChainID(chainID)

	err = testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(s.address.Bytes()), 1_000_000_000_000)
	s.Require().NoError(err)
}

// TestBankSend_EVMDispatchSuccess drives bank.send end-to-end through the EVM
// keeper and asserts the static precompile was dispatched (real coin move).
func (s *PrecompileTestSuite) TestBankSend_EVMDispatchSuccess() {
	s.mustEnableStaticPrecompiles()

	receiver := common.HexToAddress("0x2222222222222222222222222222222222222222")
	input := s.mustPackBankSendInput(receiver, big.NewInt(12345))

	precompileAddr := bank.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	s.Require().Equal(math.NewInt(999999987655), s.balance(sdk.AccAddress(s.address.Bytes())))
	s.Require().Equal(math.NewInt(12345), s.balance(sdk.AccAddress(receiver.Bytes())))
}

// TestBankSend_TotalSupplyInvariant asserts that a bank.send through the
// precompile leaves total bank supply unchanged (a transfer must not change
// supply). The sender is given code and its stateObject is loaded before the
// call so the assertion exercises the keeper/StateDB balance path; without that
// setup the check would pass regardless of that path.
func (s *PrecompileTestSuite) TestBankSend_TotalSupplyInvariant() {
	s.mustEnableStaticPrecompiles()

	supplyBefore := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount

	receiver := common.HexToAddress("0x2222222222222222222222222222222222222222")
	input := s.mustPackBankSendInput(receiver, big.NewInt(500_000_000))

	precompileAddr := bank.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	// Give the sender code and load its stateObject before the call so the test
	// exercises the balance path; otherwise the assertion would be trivial.
	stateDB.SetCode(s.address, []byte{0x60, 0x00})
	_ = stateDB.GetBalance(s.address)
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	supplyAfter := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount
	s.Require().Equal(supplyBefore.String(), supplyAfter.String(), "total bank supply must be unchanged")
}

// TestBankSend_FailureRevertsCleanly pins the native revert semantics: an
// insufficient-funds send reverts (reason ABI-encoded in the return data) and
// leaves balances and supply unchanged.
func (s *PrecompileTestSuite) TestBankSend_FailureRevertsCleanly() {
	s.mustEnableStaticPrecompiles()

	receiver := common.HexToAddress("0x5555555555555555555555555555555555555555")
	input := s.mustPackBankSendInput(receiver, big.NewInt(2_000_000_000_000))

	supplyBefore := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount

	precompileAddr := bank.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().Error(err)
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	s.Require().Contains(err.Error(), "execution reverted")
	reason, uErr := abi.UnpackRevert(res.Ret)
	s.Require().NoError(uErr)
	s.Require().Contains(reason, "insufficient funds")

	checkCtx := s.app.BaseApp.NewContext(false).
		WithBlockHeader(s.ctx.BlockHeader()).
		WithChainID(s.ctx.ChainID()).
		WithGasMeter(storetypes.NewInfiniteGasMeter()).
		WithBlockGasMeter(storetypes.NewInfiniteGasMeter())
	s.Require().Equal(math.NewInt(1_000_000_000_000), s.app.BankKeeper.GetBalance(checkCtx, sdk.AccAddress(s.address.Bytes()), utils.BaseDenom).Amount)
	s.Require().True(s.app.BankKeeper.GetBalance(checkCtx, sdk.AccAddress(receiver.Bytes()), utils.BaseDenom).Amount.IsZero())
	s.Require().Equal(supplyBefore.String(), s.app.BankKeeper.GetSupply(checkCtx, utils.BaseDenom).Amount.String())
}

// TestBankSend_ContractCallerRejectsValue asserts that #291's native-value
// guard still fires when the caller is a contract distinct from the
// transaction origin, not just for an EOA.
func (s *PrecompileTestSuite) TestBankSend_ContractCallerRejectsValue() {
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	receiver := common.HexToAddress("0x4444444444444444444444444444444444444444")

	contract := vm.NewContract(caller, bank.GetAddress(), uint256.NewInt(1), 25_000_000, nil)
	contract.Input = s.mustPackBankSendInput(receiver, big.NewInt(1))

	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	c := bank.NewPrecompile(bankkeeper.NewMsgServerImpl(s.app.BankKeeper, s.app.PaymentKeeper), s.app.BankKeeper)
	_, err := c.Run(evm, contract, false)
	s.Require().Error(err)
	s.Require().Equal("precompile does not accept value", err.Error())
}

// denoms the coin-set construction tests fund and send, besides the base denom.
const (
	coinA = "coina"
	coinB = "coinb"
)

// TestBankSend_UnsortedDuplicateDenoms pins the coin-set construction contract:
// an unsorted amount array with a repeated denom is sorted and summed into one
// coin per denom before the send is dispatched.
func (s *PrecompileTestSuite) TestBankSend_UnsortedDuplicateDenoms() {
	s.mustEnableStaticPrecompiles()

	sender := sdk.AccAddress(s.address.Bytes())
	s.Require().NoError(testutil.FundAccount(s.ctx, s.app.BankKeeper, sender, sdk.NewCoins(
		sdk.NewInt64Coin(coinA, 100),
		sdk.NewInt64Coin(coinB, 100),
	)))

	receiver := common.HexToAddress("0x6666666666666666666666666666666666666666")
	method := bank.MustMethod(bank.SendMethodName)
	packedArgs, err := method.Inputs.Pack(receiver, []bank.Coin{
		{Denom: coinB, Amount: big.NewInt(2)},
		{Denom: coinA, Amount: big.NewInt(5)},
		{Denom: coinB, Amount: big.NewInt(3)},
	})
	s.Require().NoError(err)
	input := append(append([]byte{}, method.ID...), packedArgs...)

	precompileAddr := bank.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	recv := sdk.AccAddress(receiver.Bytes())
	s.Require().Equal(math.NewInt(5), s.app.BankKeeper.GetBalance(s.ctx, recv, coinA).Amount)
	s.Require().Equal(math.NewInt(5), s.app.BankKeeper.GetBalance(s.ctx, recv, coinB).Amount)
}

// TestBankMultiSend_UnsortedDuplicateDenoms is the multiSend counterpart: each
// output's unsorted, repeated denoms are summed, and the sender is debited the
// total across all outputs, including a recipient that appears twice.
func (s *PrecompileTestSuite) TestBankMultiSend_UnsortedDuplicateDenoms() {
	s.mustEnableStaticPrecompiles()

	sender := sdk.AccAddress(s.address.Bytes())
	s.Require().NoError(testutil.FundAccount(s.ctx, s.app.BankKeeper, sender, sdk.NewCoins(
		sdk.NewInt64Coin(coinA, 100),
		sdk.NewInt64Coin(coinB, 100),
	)))

	receiverA := common.HexToAddress("0x7777777777777777777777777777777777777777")
	receiverB := common.HexToAddress("0x8888888888888888888888888888888888888888")
	method := bank.MustMethod(bank.MultiSendMethodName)
	packedArgs, err := method.Inputs.Pack([]bank.Output{
		{ToAddress: receiverA, Amount: []bank.Coin{
			{Denom: coinB, Amount: big.NewInt(2)},
			{Denom: coinA, Amount: big.NewInt(5)},
			{Denom: coinB, Amount: big.NewInt(3)},
		}},
		{ToAddress: receiverB, Amount: []bank.Coin{{Denom: coinA, Amount: big.NewInt(7)}}},
		{ToAddress: receiverA, Amount: []bank.Coin{{Denom: coinB, Amount: big.NewInt(1)}}},
	})
	s.Require().NoError(err)
	input := append(append([]byte{}, method.ID...), packedArgs...)

	precompileAddr := bank.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	recvA := sdk.AccAddress(receiverA.Bytes())
	recvB := sdk.AccAddress(receiverB.Bytes())
	s.Require().Equal(math.NewInt(5), s.app.BankKeeper.GetBalance(s.ctx, recvA, coinA).Amount)
	s.Require().Equal(math.NewInt(6), s.app.BankKeeper.GetBalance(s.ctx, recvA, coinB).Amount)
	s.Require().Equal(math.NewInt(7), s.app.BankKeeper.GetBalance(s.ctx, recvB, coinA).Amount)
	s.Require().True(s.app.BankKeeper.GetBalance(s.ctx, recvB, coinB).Amount.IsZero())
	s.Require().Equal(math.NewInt(88), s.app.BankKeeper.GetBalance(s.ctx, sender, coinA).Amount)
	s.Require().Equal(math.NewInt(94), s.app.BankKeeper.GetBalance(s.ctx, sender, coinB).Amount)
}

func (s *PrecompileTestSuite) balance(addr sdk.AccAddress) math.Int {
	return s.app.BankKeeper.GetBalance(s.ctx, addr, utils.BaseDenom).Amount
}

func (s *PrecompileTestSuite) mustEnableStaticPrecompiles() {
	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

func (s *PrecompileTestSuite) mustPackBankSendInput(to common.Address, amount *big.Int) []byte {
	method := bank.MustMethod(bank.SendMethodName)
	packedArgs, err := method.Inputs.Pack(to, []bank.Coin{{Denom: utils.BaseDenom, Amount: amount}})
	s.Require().NoError(err)
	return append(append([]byte{}, method.ID...), packedArgs...)
}
