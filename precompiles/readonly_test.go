package precompiles_test

import (
	"maps"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtestutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/precompiles/authz"
	"github.com/mocachain/moca/v2/precompiles/bank"
	"github.com/mocachain/moca/v2/precompiles/distribution"
	"github.com/mocachain/moca/v2/precompiles/gov"
	"github.com/mocachain/moca/v2/precompiles/payment"
	"github.com/mocachain/moca/v2/precompiles/permission"
	"github.com/mocachain/moca/v2/precompiles/slashing"
	"github.com/mocachain/moca/v2/precompiles/staking"
	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/precompiles/storageprovider"
	"github.com/mocachain/moca/v2/precompiles/virtualgroup"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
)

var (
	staticRelay = common.HexToAddress("0x8888888888888888888888888888888888888888")
	callRelay   = common.HexToAddress("0x9999999999999999999999999999999999999999")
	payee       = common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
)

// precompileABIs holds the name and ABI of each precompile the app registers.
var precompileABIs = map[common.Address]struct{ name, abi string }{
	authz.GetAddress():           {"authz", authz.IAuthzABI},
	bank.GetAddress():            {"bank", bank.IBankABI},
	distribution.GetAddress():    {"distribution", distribution.IDistributionABI},
	gov.GetAddress():             {"gov", gov.IGovABI},
	payment.GetAddress():         {"payment", payment.IPaymentABI},
	permission.GetAddress():      {"permission", permission.IPermissionABI},
	slashing.GetAddress():        {"slashing", slashing.ISlashingABI},
	staking.GetAddress():         {"staking", staking.IStakingABI},
	storage.GetAddress():         {"storage", storage.IStorageABI},
	storageprovider.GetAddress(): {"storageprovider", storageprovider.IStorageProviderABI},
	virtualgroup.GetAddress():    {"virtualgroup", virtualgroup.IVirtualGroupABI},
}

type ReadOnlyTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestReadOnlyTestSuite(t *testing.T) {
	suite.Run(t, new(ReadOnlyTestSuite))
}

func (s *ReadOnlyTestSuite) SetupTest() {
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
	s.Require().NoError(s.app.StakingKeeper.SetValidator(s.ctx, validator))
	s.Require().NoError(s.app.StakingKeeper.SetValidatorByConsAddr(s.ctx, validator))

	safeTime := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	header := evmtestutil.NewHeader(1, safeTime, chainID, sdk.ConsAddress(valConsAddr.Bytes()), tmhash.Sum([]byte("app")), tmhash.Sum([]byte("validators")))
	s.ctx = s.ctx.WithBlockHeader(header).WithChainID(chainID)

	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(s.address.Bytes()), 1_000_000_000_000))
	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(callRelay.Bytes()), 1_000_000))

	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

// TestTransactionMethodsFollowInterpreterReadOnly expects IsTransaction to match the ABI, and every
// transaction method CALLed from inside a STATICCALL frame to revert with the write-protection error.
func (s *ReadOnlyTestSuite) TestTransactionMethodsFollowInterpreterReadOnly() {
	params := s.app.EvmKeeper.GetParams(s.ctx)
	checked := 0
	for _, hexAddr := range app.MocaActiveStaticPrecompiles() {
		addr := common.HexToAddress(hexAddr)
		pc, ok := precompileABIs[addr]
		s.Require().True(ok, "no ABI listed for %s", hexAddr)
		parsed, err := abi.JSON(strings.NewReader(pc.abi))
		s.Require().NoError(err)
		instance, found, err := s.app.EvmKeeper.GetStaticPrecompileInstance(&params, addr)
		s.Require().NoError(err)
		s.Require().True(found, pc.name)
		classifier, ok := instance.(interface{ IsTransaction(*abi.Method) bool })
		s.Require().True(ok, pc.name)

		for _, name := range slices.Sorted(maps.Keys(parsed.Methods)) {
			method := parsed.Methods[name]
			s.Require().Equal(!method.IsConstant(), classifier.IsTransaction(&method), "%s.%s", pc.name, name)
			if method.IsConstant() {
				continue
			}
			checked++
			s.Run(pc.name+"/"+name, func() {
				// Zero words decode as zero values for any argument list.
				res, err := s.staticCall(addr, slices.Concat(method.ID, make([]byte, 32*64)))
				s.requireWriteProtection(res, err)
			})
		}
	}
	s.Require().Positive(checked)
}

// TestViewMethodUnderStaticCall expects a view method reached the same way to succeed.
func (s *ReadOnlyTestSuite) TestViewMethodUnderStaticCall() {
	method := bank.MustMethod(bank.BalanceMethodName)
	args, err := method.Inputs.Pack(callRelay, utils.BaseDenom)
	s.Require().NoError(err)

	res, err := s.staticCall(bank.GetAddress(), slices.Concat(method.ID, args))
	s.Require().NoError(err)
	s.Require().False(res.Failed(), res.VmError)

	out, err := method.Outputs.Unpack(res.Ret)
	s.Require().NoError(err)
	coin := abi.ConvertType(out[0], new(bank.Coin)).(*bank.Coin)
	s.Require().Equal(big.NewInt(1_000_000), coin.Amount)
}

// TestBankSendUnderStaticCallKeepsBalances expects bank.send reached the same way to
// leave the sender and recipient balances unchanged.
func (s *ReadOnlyTestSuite) TestBankSendUnderStaticCallKeepsBalances() {
	method := bank.MustMethod(bank.SendMethodName)
	args, err := method.Inputs.Pack(payee, []bank.Coin{{Denom: utils.BaseDenom, Amount: big.NewInt(400_000)}})
	s.Require().NoError(err)

	res, err := s.staticCall(bank.GetAddress(), slices.Concat(method.ID, args))
	s.Require().Equal(math.NewInt(1_000_000), s.balance(callRelay))
	s.Require().True(s.balance(payee).IsZero())
	s.requireWriteProtection(res, err)
}

// staticCall sends a transaction from the EOA to staticRelay, which STATICCALLs
// callRelay, which CALLs target with input.
func (s *ReadOnlyTestSuite) staticCall(target common.Address, input []byte) (*evmtypes.MsgEthereumTxResponse, error) {
	// A failed call charges the whole gas limit, so each call gets its own meter.
	ctx := s.ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	stateDB := statedb.New(ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	stateDB.SetCode(staticRelay, relayCode(vm.STATICCALL))
	stateDB.SetCode(callRelay, relayCode(vm.CALL))

	to := staticRelay
	data := slices.Concat(common.LeftPadBytes(callRelay.Bytes(), 32), common.LeftPadBytes(target.Bytes(), 32), input)
	return s.app.EvmKeeper.CallEVMWithData(ctx, stateDB, s.address, &to, data, true, false, nil)
}

func (s *ReadOnlyTestSuite) requireWriteProtection(res *evmtypes.MsgEthereumTxResponse, err error) {
	s.Require().Error(err)
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	reason, err := abi.UnpackRevert(res.Ret)
	s.Require().NoError(err)
	s.Require().Equal(vm.ErrWriteProtection.Error(), reason)
}

func (s *ReadOnlyTestSuite) balance(addr common.Address) math.Int {
	return s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(addr.Bytes()), utils.BaseDenom).Amount
}

// relayCode returns runtime code that forwards calldata[32:] with op (CALL or STATICCALL)
// to the address in calldata[0:32] and returns or reverts with the callee's return data.
func relayCode(op vm.OpCode) []byte {
	p := program.New().Push(32).Op(vm.CALLDATASIZE, vm.SUB).Push(32).Push(0).Op(vm.CALLDATACOPY)
	p.Push(0).Push(0).Push(32).Op(vm.CALLDATASIZE, vm.SUB).Push(0)
	if op == vm.CALL {
		p.Push(0)
	}
	p.Push(0).Op(vm.CALLDATALOAD, vm.GAS, op, vm.RETURNDATASIZE).Push(0).Push(0).Op(vm.RETURNDATACOPY)
	p.Push(p.Size()+7).Op(vm.JUMPI, vm.RETURNDATASIZE).Push(0).Op(vm.REVERT) // on success, jump over these 7 bytes
	return p.Op(vm.JUMPDEST, vm.RETURNDATASIZE).Push(0).Op(vm.RETURN).Bytes()
}
