package bank_test

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
	"github.com/mocachain/moca/v2/x/evm/precompiles/bank"
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
	header := testutil.NewHeader(1, safeTime, chainID, sdk.ConsAddress(valConsAddr.Bytes()), tmhash.Sum([]byte("app")), tmhash.Sum([]byte("validators")))
	s.ctx = s.ctx.WithBlockHeader(header).WithChainID(chainID)

	accAddr := sdk.AccAddress(s.address.Bytes())
	err = testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, accAddr, 1_000_000_000_000)
	s.Require().NoError(err)
}

func (s *PrecompileTestSuite) TestBankSend_EVMDispatchSuccess() {
	s.mustEnableStaticPrecompiles()

	receiver := common.HexToAddress("0x2222222222222222222222222222222222222222")
	input := mustPackBankSendInput(s.T(), receiver, big.NewInt(12345))

	precompileAddr := bank.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	senderBalance := s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(s.address.Bytes()), utils.BaseDenom)
	receiverBalance := s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(receiver.Bytes()), utils.BaseDenom)

	s.Require().Equal(math.NewInt(999999987655), senderBalance.Amount)
	s.Require().Equal(math.NewInt(12345), receiverBalance.Amount)
}

func (s *PrecompileTestSuite) TestBankSend_RejectsContractForwarding() {
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	receiver := common.HexToAddress("0x4444444444444444444444444444444444444444")
	input := mustPackBankSendInput(s.T(), receiver, big.NewInt(1))

	contract := vm.NewContract(caller, bank.GetAddress(), uint256.NewInt(0), 60_000, nil)
	contract.Input = input

	evm := &vm.EVM{}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	c := bank.NewPrecompiledContract(s.app.BankKeeper, s.app.PaymentKeeper)
	_, err := c.Send(s.ctx, evm, contract, false)
	s.Require().EqualError(err, "only allow EOA can call this method")
}

func (s *PrecompileTestSuite) TestBankSend_FailureDoesNotChangeBalances() {
	s.mustEnableStaticPrecompiles()

	receiver := common.HexToAddress("0x5555555555555555555555555555555555555555")
	input := mustPackBankSendInput(s.T(), receiver, big.NewInt(2_000_000_000_000))

	precompileAddr := bank.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())

	senderBefore := s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(s.address.Bytes()), utils.BaseDenom)
	receiverBefore := s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(receiver.Bytes()), utils.BaseDenom)

	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "insufficient funds")
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	s.Require().Contains(res.VmError, "insufficient funds")

	checkCtx := s.app.BaseApp.NewContext(false).
		WithBlockHeader(s.ctx.BlockHeader()).
		WithChainID(s.ctx.ChainID()).
		WithGasMeter(storetypes.NewInfiniteGasMeter()).
		WithBlockGasMeter(storetypes.NewInfiniteGasMeter())
	senderAfter := s.app.BankKeeper.GetBalance(checkCtx, sdk.AccAddress(s.address.Bytes()), utils.BaseDenom)
	receiverAfter := s.app.BankKeeper.GetBalance(checkCtx, sdk.AccAddress(receiver.Bytes()), utils.BaseDenom)

	s.Require().Equal(senderBefore.Amount, senderAfter.Amount)
	s.Require().Equal(receiverBefore.Amount, receiverAfter.Amount)
}

func (s *PrecompileTestSuite) mustEnableStaticPrecompiles() {
	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

func mustPackBankSendInput(t *testing.T, to common.Address, amount *big.Int) []byte {
	t.Helper()

	method := bank.MustMethod(bank.SendMethodName)
	packedArgs, err := method.Inputs.Pack(to, []bank.Coin{{Denom: utils.BaseDenom, Amount: amount}})
	require.NoError(t, err)

	return append(append([]byte{}, method.ID...), packedArgs...)
}
