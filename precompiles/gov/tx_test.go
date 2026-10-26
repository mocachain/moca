package gov_test

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
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/precompiles/gov"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
)

type PrecompileTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
	abi     *abi.ABI
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

	var err error
	s.abi, err = gov.IGovMetaData.GetAbi()
	s.Require().NoError(err)

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

	// the default gov genesis denominates deposits in "stake"; moca deposits in the base denom
	govParams, err := s.app.GovKeeper.Params.Get(s.ctx)
	s.Require().NoError(err)
	govParams.MinDeposit = sdk.NewCoins(sdk.NewInt64Coin(utils.BaseDenom, 10_000_000))
	govParams.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewInt64Coin(utils.BaseDenom, 50_000_000))
	s.Require().NoError(s.app.GovKeeper.Params.Set(s.ctx, govParams))
}

// TestSubmitProposal_CreatesProposal drives submitProposal (v1) end-to-end through the
// EVM keeper and checks the stored proposal: proposer, deposit, title and summary.
func (s *PrecompileTestSuite) TestSubmitProposal_CreatesProposal() {
	deposit := big.NewInt(10_000_000)
	method := s.abi.Methods[gov.SubmitProposalMethod]
	packedArgs, err := method.Inputs.Pack("[]", []gov.Coin{{Denom: utils.BaseDenom, Amount: deposit}}, "metadata", "title", "summary", false)
	s.Require().NoError(err)
	input := append(append([]byte{}, method.ID...), packedArgs...)

	precompileAddr := gov.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	out, err := method.Outputs.Unpack(res.Ret)
	s.Require().NoError(err)
	s.Require().Len(out, 1)
	proposalID, ok := out[0].(uint64)
	s.Require().True(ok)

	proposal, err := s.app.GovKeeper.Proposals.Get(s.ctx, proposalID)
	s.Require().NoError(err)
	s.Require().Equal(sdk.AccAddress(s.address.Bytes()).String(), proposal.Proposer)
	s.Require().Equal(sdk.NewCoins(sdk.NewCoin(utils.BaseDenom, math.NewIntFromBigInt(deposit))), sdk.Coins(proposal.TotalDeposit))
	s.Require().Equal("title", proposal.Title)
	s.Require().Equal("summary", proposal.Summary)
	s.Require().Empty(proposal.Messages)
}

// TestLegacySubmitProposal_Removed pins the removal of the v1beta1 path: the ABI has
// neither the method nor its event, and a call to the old selector reverts as an
// unknown method instead of reaching a handler.
func (s *PrecompileTestSuite) TestLegacySubmitProposal_Removed() {
	_, hasMethod := s.abi.Methods["legacySubmitProposal"]
	s.Require().False(hasMethod)
	_, hasEvent := s.abi.Events["LegacySubmitProposal"]
	s.Require().False(hasEvent)

	selector := crypto.Keccak256([]byte("legacySubmitProposal(string,string,(string,uint256)[])"))[:4]
	precompileAddr := gov.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, selector, true, false, nil)
	s.Require().Error(err)
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	s.Require().Contains(err.Error(), "execution reverted")
	reason, uErr := abi.UnpackRevert(res.Ret)
	s.Require().NoError(uErr)
	s.Require().Contains(reason, "no method with id")
}
