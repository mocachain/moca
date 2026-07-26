package payment_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtestutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/precompiles/payment"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
	paymentkeeper "github.com/mocachain/moca/v2/x/payment/keeper"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

type SupplyTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestSupplyTestSuite(t *testing.T) {
	suite.Run(t, new(SupplyTestSuite))
}

func (s *SupplyTestSuite) SetupTest() {
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

// TestDeposit_TotalSupplyInvariant asserts that a deposit through the payment
// precompile (which moves coins depositor -> payment module) leaves total bank
// supply unchanged. The depositor is given code and its stateObject is loaded
// before the call so the assertion exercises the keeper/StateDB balance path;
// without that setup the check would pass regardless of that path.
func (s *SupplyTestSuite) TestDeposit_TotalSupplyInvariant() {
	// EthSetup's genesis leaves payment params zero-valued, so install defaults
	// (DefaultFeeDenom is the base denom "amoca") before depositing.
	s.Require().NoError(s.app.PaymentKeeper.SetParams(s.ctx, paymenttypes.DefaultParams()))

	s.mustEnableStaticPrecompiles()

	supplyBefore := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount

	// self-deposit: the caller account exists, so a stream record is created
	to := sdk.AccAddress(s.address.Bytes()).String()
	input := s.mustPackDepositInput(to, big.NewInt(100_000))
	precompileAddr := payment.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	// Give the depositor code and load its stateObject before the call so the
	// test exercises the balance path; otherwise the assertion would be trivial.
	stateDB.SetCode(s.address, []byte{0x60, 0x00})
	_ = stateDB.GetBalance(s.address)
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	supplyAfter := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount
	s.Require().Equal(supplyBefore.String(), supplyAfter.String(), "total bank supply must be unchanged")
}

// TestDeposit_ContractCallerSupplyInvariant asserts that a deposit driven by
// a contract caller (an address distinct from the transaction origin) through
// the full Run/RunNativeAction/BalanceHandler path, committed to the StateDB,
// leaves total bank supply unchanged. Mirrors
// bank.TestBankSend_ContractCallerSupplyInvariant for the payment
// precompile's depositor -> payment module coin move: #362 removed the
// EOA-only guard so a contract's Caller() can now reach this path directly,
// and #332's BalanceHandler must still neutralize the delta-mint for that
// caller.
func (s *SupplyTestSuite) TestDeposit_ContractCallerSupplyInvariant() {
	// EthSetup's genesis leaves payment params zero-valued, so install defaults
	// (DefaultFeeDenom is the base denom "amoca") before depositing.
	s.Require().NoError(s.app.PaymentKeeper.SetParams(s.ctx, paymenttypes.DefaultParams()))

	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(caller.Bytes()), 1_000_000))

	supplyBefore := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount

	// self-deposit: the caller account exists, so a stream record is created
	to := sdk.AccAddress(caller.Bytes()).String()
	contract := vm.NewContract(caller, payment.GetAddress(), uint256.NewInt(0), 25_000_000, nil)
	contract.Input = s.mustPackDepositInput(to, big.NewInt(100_000))

	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	// Give the caller code and load its stateObject before the call, exactly
	// like TestDeposit_TotalSupplyInvariant does for the EOA case, so
	// StateDB.Commit's reconciliation walk actually visits it; otherwise the
	// invariant would hold trivially regardless of the reconciliation path.
	stateDB.SetCode(caller, []byte{0x60, 0x00})
	_ = stateDB.GetBalance(caller)

	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	c := payment.NewPrecompile(paymentkeeper.NewMsgServerImpl(s.app.PaymentKeeper), s.app.PaymentKeeper, s.app.BankKeeper)
	_, err := c.Run(evm, contract, false)
	s.Require().NoError(err)
	s.Require().NoError(stateDB.Commit())

	supplyAfter := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount
	s.Require().Equal(supplyBefore.String(), supplyAfter.String(), "total bank supply must be unchanged for a contract caller distinct from origin")
	s.Require().Equal(math.NewInt(900_000), s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(caller.Bytes()), utils.BaseDenom).Amount, "acting contract.Caller() must be debited")
	s.Require().Equal(math.NewInt(1_000_000_000_000), s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(s.address.Bytes()), utils.BaseDenom).Amount, "transaction origin must be untouched")
}

func (s *SupplyTestSuite) mustEnableStaticPrecompiles() {
	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

func (s *SupplyTestSuite) mustPackDepositInput(to string, amount *big.Int) []byte {
	method := payment.MustMethod(payment.DepositMethodName)
	packedArgs, err := method.Inputs.Pack(to, amount)
	s.Require().NoError(err)
	return append(append([]byte{}, method.ID...), packedArgs...)
}
