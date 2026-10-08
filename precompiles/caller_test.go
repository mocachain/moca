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
	ethtypes "github.com/ethereum/go-ethereum/core/types"

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
	"github.com/mocachain/moca/v2/precompiles/types"
	"github.com/mocachain/moca/v2/precompiles/virtualgroup"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
)

// forwarderCode is runtime bytecode that calls the address in the first
// calldata word with the rest of the calldata and returns, or reverts with,
// the callee's return data:
//
//	calldatacopy(0, 32, calldatasize() - 32)
//	ok := call(gas(), calldataload(0), 0, 0, calldatasize() - 32, 0, 0)
//	returndatacopy(0, 0, returndatasize())
//	if ok { return(0, returndatasize()) }
//	revert(0, returndatasize())
var forwarderCode = common.FromHex("6020360360206000376000600060203603600060006000355af13d600060003e6027573d6000fd5b3d6000f3")

var (
	forwarder = common.HexToAddress("0x7777777777777777777777777777777777777777")
	receiver  = common.HexToAddress("0x2222222222222222222222222222222222222222")
)

var precompiles = []struct {
	name    string
	address common.Address
	abi     string
}{
	{"authz", authz.GetAddress(), authz.IAuthzABI},
	{"bank", bank.GetAddress(), bank.IBankABI},
	{"distribution", distribution.GetAddress(), distribution.IDistributionABI},
	{"gov", gov.GetAddress(), gov.IGovABI},
	{"payment", payment.GetAddress(), payment.IPaymentABI},
	{"permission", permission.GetAddress(), permission.IPermissionABI},
	{"slashing", slashing.GetAddress(), slashing.ISlashingABI},
	{"staking", staking.GetAddress(), staking.IStakingABI},
	{"storage", storage.GetAddress(), storage.IStorageABI},
	{"storageprovider", storageprovider.GetAddress(), storageprovider.IStorageProviderABI},
	{"virtualgroup", virtualgroup.GetAddress(), virtualgroup.IVirtualGroupABI},
}

type CallerTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestCallerTestSuite(t *testing.T) {
	suite.Run(t, new(CallerTestSuite))
}

func (s *CallerTestSuite) SetupTest() {
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

	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

// TestTransactionMethodsRejectContractCaller calls every state-changing method
// of every precompile from a contract and expects ErrInvalidCaller. Each
// precompile's IsTransaction must agree with its ABI's state mutability.
func (s *CallerTestSuite) TestTransactionMethodsRejectContractCaller() {
	addrs := make([]string, 0, len(precompiles))
	for _, pc := range precompiles {
		addrs = append(addrs, pc.address.Hex())
	}
	s.Require().ElementsMatch(app.MocaActiveStaticPrecompiles(), addrs, "every registered precompile must be listed")

	params := s.app.EvmKeeper.GetParams(s.ctx)
	checked := 0
	for _, pc := range precompiles {
		instance, found, err := s.app.EvmKeeper.GetStaticPrecompileInstance(&params, pc.address)
		s.Require().NoError(err)
		s.Require().True(found, pc.name)
		classifier, ok := instance.(interface{ IsTransaction(*abi.Method) bool })
		s.Require().True(ok, pc.name)

		parsed, err := abi.JSON(strings.NewReader(pc.abi))
		s.Require().NoError(err)
		for _, name := range slices.Sorted(maps.Keys(parsed.Methods)) {
			method := parsed.Methods[name]
			s.Require().Equal(!method.IsConstant(), classifier.IsTransaction(&method), "%s.%s", pc.name, name)
			if method.IsConstant() {
				continue
			}
			checked++
			s.Run(pc.name+"/"+method.Name, func() {
				// Zero words decode as zero values for any argument list.
				input := slices.Concat(method.ID, make([]byte, 32*64))
				res, err := s.call(s.address, forwarder, pc.address, input, nil)
				s.requireInvalidCaller(res, err)
			})
		}
	}
	s.Require().Positive(checked)
}

// TestQueryMethodsAllowContractCaller pins that view methods stay callable
// from contracts.
func (s *CallerTestSuite) TestQueryMethodsAllowContractCaller() {
	method := bank.MustMethod(bank.BalanceMethodName)
	args, err := method.Inputs.Pack(s.address, utils.BaseDenom)
	s.Require().NoError(err)

	res, err := s.call(s.address, forwarder, bank.GetAddress(), slices.Concat(method.ID, args), nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), res.VmError)

	out, err := method.Outputs.Unpack(res.Ret)
	s.Require().NoError(err)
	coin := abi.ConvertType(out[0], new(bank.Coin)).(*bank.Coin)
	s.Require().Equal(big.NewInt(1_000_000_000_000), coin.Amount)
}

// TestDelegatedEOASigningItself pins that an EIP-7702 delegated EOA that signs
// the transaction itself can call a transaction method from its delegated code,
// and that the precompile acts as that EOA without changing total supply.
func (s *CallerTestSuite) TestDelegatedEOASigningItself() {
	eoa := common.HexToAddress("0x3333333333333333333333333333333333333333")
	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(eoa.Bytes()), 1_000_000))
	supplyBefore := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount

	res, err := s.call(eoa, eoa, bank.GetAddress(), s.packSend(400_000), ethtypes.AddressToDelegation(forwarder))
	s.Require().NoError(err)
	s.Require().False(res.Failed(), res.VmError)

	s.Require().Equal(math.NewInt(600_000), s.balance(eoa))
	s.Require().Equal(math.NewInt(400_000), s.balance(receiver))
	s.Require().Equal(supplyBefore.String(), s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount.String())
}

// TestDelegatedEOARelayed pins that a call relayed into an EIP-7702 delegated
// EOA by another signer is rejected.
func (s *CallerTestSuite) TestDelegatedEOARelayed() {
	eoa := common.HexToAddress("0x3333333333333333333333333333333333333333")
	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(eoa.Bytes()), 1_000_000))

	res, err := s.call(s.address, eoa, bank.GetAddress(), s.packSend(400_000), ethtypes.AddressToDelegation(forwarder))
	s.requireInvalidCaller(res, err)

	s.Require().Equal(math.NewInt(1_000_000), s.balance(eoa))
	s.Require().True(s.balance(receiver).IsZero())
}

// call sends a transaction from `from` to `to` that forwards input to target.
// `to` runs forwarderCode, or the given code when set.
func (s *CallerTestSuite) call(from, to, target common.Address, input, code []byte) (*evmtypes.MsgEthereumTxResponse, error) {
	// A failed call charges the whole gas limit, so each call gets its own meter.
	ctx := s.ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	stateDB := statedb.New(ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	stateDB.SetCode(forwarder, forwarderCode)
	if code != nil {
		stateDB.SetCode(to, code)
	}
	data := slices.Concat(common.LeftPadBytes(target.Bytes(), 32), input)
	return s.app.EvmKeeper.CallEVMWithData(ctx, stateDB, from, &to, data, true, false, nil)
}

func (s *CallerTestSuite) requireInvalidCaller(res *evmtypes.MsgEthereumTxResponse, err error) {
	s.Require().Error(err)
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	reason, err := abi.UnpackRevert(res.Ret)
	s.Require().NoError(err)
	s.Require().Equal(types.ErrInvalidCaller.Error(), reason)
}

func (s *CallerTestSuite) packSend(amount int64) []byte {
	method := bank.MustMethod(bank.SendMethodName)
	args, err := method.Inputs.Pack(receiver, []bank.Coin{{Denom: utils.BaseDenom, Amount: big.NewInt(amount)}})
	s.Require().NoError(err)
	return slices.Concat(method.ID, args)
}

func (s *CallerTestSuite) balance(addr common.Address) math.Int {
	return s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(addr.Bytes()), utils.BaseDenom).Amount
}
