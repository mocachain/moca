package virtualgroup_test

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/suite"

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
	"github.com/mocachain/moca/v2/precompiles/virtualgroup"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	virtualgrouptypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

type CompleteSPExitTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestCompleteSPExitTestSuite(t *testing.T) {
	suite.Run(t, new(CompleteSPExitTestSuite))
}

func (s *CompleteSPExitTestSuite) SetupTest() {
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
}

// TestCompleteSPExit_OperatorIsCaller asserts that completeSPExit sources
// both MsgCompleteStorageProviderExit.Operator and the EVM log's operator
// topic from contract.Caller(), never from the caller-controlled
// args.Operator. GetSigners() returns Operator ahead of StorageProvider when
// Operator parses, and the msgServer copies msg.Operator verbatim into the
// emitted EventCompleteStorageProviderExit.OperatorAddress -- so before the
// fix, a caller could stamp an arbitrary address as the signer/operator of
// their own exit, in both the Cosmos event and the EVM log.
func (s *CompleteSPExitTestSuite) TestCompleteSPExit_OperatorIsCaller() {
	callerAcc := sdk.AccAddress(s.address.Bytes()).String()
	deposit := math.NewInt(1_000)

	// EthSetup's genesis leaves sp params zero-valued (DepositDenomForSP would
	// be ""), so install defaults before the deposit refund.
	s.Require().NoError(s.app.SpKeeper.SetParams(s.ctx, sptypes.DefaultParams()))

	// Seed a storage provider already mid-graceful-exit (bypassing the
	// governance-gated create/exit flow, which is out of scope here) so
	// CompleteStorageProviderExit's preconditions are met.
	sp := sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: callerAcc,
		FundingAddress:  callerAcc,
		SealAddress:     callerAcc,
		ApprovalAddress: callerAcc,
		GcAddress:       callerAcc,
		Status:          sptypes.STATUS_GRACEFUL_EXITING,
		TotalDeposit:    deposit,
	}
	s.app.SpKeeper.SetStorageProvider(s.ctx, &sp)
	s.app.SpKeeper.SetStorageProviderByOperatorAddr(s.ctx, &sp)
	s.Require().NoError(testutil.FundModuleAccount(s.ctx, s.app.BankKeeper, sptypes.ModuleName,
		sdk.NewCoins(sdk.NewCoin(utils.BaseDenom, deposit))))

	// args.Operator is attacker-controlled EVM calldata; set it to a
	// different address than caller to prove it cannot leak into either the
	// Cosmos message or the EVM log.
	spoofedOperator := common.HexToAddress("0x9999999999999999999999999999999999999999")

	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	contract := vm.NewContract(s.address, virtualgroup.GetAddress(), uint256.NewInt(0), 200_000, nil)
	contract.Input = s.mustPackCompleteSPExitInput(spoofedOperator)
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	c := virtualgroup.NewPrecompiledContract(s.app.VirtualgroupKeeper, s.app.BankKeeper)
	_, err := c.CompleteSPExit(s.ctx, evm, contract, false)
	s.Require().NoError(err)

	event := s.findCompleteStorageProviderExitEvent()
	s.Require().NotNil(event, "EventCompleteStorageProviderExit must be emitted")
	s.Require().Equal(callerAcc, event.OperatorAddress, "emitted Cosmos operator must be the caller, never the spoofed args.Operator")

	logs := stateDB.Logs()
	s.Require().Len(logs, 1)
	s.Require().Len(logs[0].Topics, 3, "event sig + storageProvider + operator topics")
	s.Require().Equal(common.BytesToHash(s.address.Bytes()), logs[0].Topics[2], "EVM log operator topic must be the caller, never the spoofed args.Operator")
}

func (s *CompleteSPExitTestSuite) findCompleteStorageProviderExitEvent() *virtualgrouptypes.EventCompleteStorageProviderExit {
	for _, abciEvent := range s.ctx.EventManager().Events().ToABCIEvents() {
		msg, err := sdk.ParseTypedEvent(abciEvent)
		if err != nil {
			continue
		}
		if typed, ok := msg.(*virtualgrouptypes.EventCompleteStorageProviderExit); ok {
			return typed
		}
	}
	return nil
}

func (s *CompleteSPExitTestSuite) mustPackCompleteSPExitInput(operator common.Address) []byte {
	method := virtualgroup.MustMethod(virtualgroup.CompleteSPExitMethodName)
	packedArgs, err := method.Inputs.Pack(operator.String())
	s.Require().NoError(err)
	return append(append([]byte{}, method.ID...), packedArgs...)
}
