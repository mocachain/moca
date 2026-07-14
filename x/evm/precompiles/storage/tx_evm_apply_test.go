package storage_test

// Migration-baseline characterization tests for the storage precompile.
//
// The migration plan's default representative write method was createBucket, but
// its success fixture is heavy: it requires a registered primary SP, a global
// virtual group family, payment prerequisites, and a valid SP approval signature.
// Per the plan's Open Risk ("if the createBucket fixture is too costly, pick
// another deterministic transactional storage method"), these baseline tests use
// createGroup instead — a transactional storage write whose only precondition is a
// funded creator, so the success/EOA-only/failure behaviors can be pinned cheaply
// and deterministically. The EOA-only guard and Run() snapshot/revert semantics
// exercised here are shared by every storage tx method, including createBucket.

import (
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/contracts"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
	"github.com/mocachain/moca/v2/x/evm/precompiles/storage"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

type CreateGroupTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestCreateGroupTestSuite(t *testing.T) {
	suite.Run(t, new(CreateGroupTestSuite))
}

func (s *CreateGroupTestSuite) SetupTest() {
	checkTx := false
	chainID := utils.TestnetChainID + "-1"

	s.app = app.EthSetup(checkTx, nil)
	s.ctx = s.app.NewContext(checkTx)
	s.address = common.HexToAddress("0x1111111111111111111111111111111111111111")

	// Prepare a valid proposer/validator for EVM coinbase resolution.
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

	err = testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(s.address.Bytes()), 1_000_000_000_000)
	s.Require().NoError(err)

	// createGroup mints a group NFT via an internal CallEVM whose sender is the
	// group control hub (contracts.GroupControlHubAddress, 0x...dead). EthSetup's
	// genesis does not create that account, so register it here or the mint fails
	// with "account 0x...dEaD does not exist: unknown address".
	controlHub := sdk.AccAddress(contracts.GroupControlHubAddress.Bytes())
	s.app.AccountKeeper.SetAccount(s.ctx, s.app.AccountKeeper.NewAccountWithAddress(s.ctx, controlHub))
}

// TestCreateGroup_EVMDispatchSuccess drives createGroup end-to-end THROUGH the EVM
// keeper and asserts the static precompile was dispatched (a group was actually
// created in storage state, owned by the caller).
func (s *CreateGroupTestSuite) TestCreateGroup_EVMDispatchSuccess() {
	s.mustEnableStaticPrecompiles()

	const groupName = "baseline-group-success"
	input := s.mustPackCreateGroupInput(groupName, "")

	precompileAddr := storage.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "evm call reverted: %s", res.VmError)

	group, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, sdk.AccAddress(s.address.Bytes()), groupName)
	s.Require().True(found, "group created via EVM dispatch")
	s.Require().Equal(sdk.AccAddress(s.address.Bytes()).String(), group.Owner, "group owner == caller")
}

// TestCreateGroup_RejectsContractForwarding freezes the current EOA-only guard:
// when the direct caller differs from the tx origin (a contract forwarding the
// call), createGroup is rejected before any state is touched. The migration to
// direct-caller semantics must consciously change this, so we pin it here.
func (s *CreateGroupTestSuite) TestCreateGroup_RejectsContractForwarding() {
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	input := s.mustPackCreateGroupInput("baseline-group-fwd", "")

	contract := vm.NewContract(caller, storage.GetAddress(), uint256.NewInt(0), 60_000, nil)
	contract.Input = input

	evm := &vm.EVM{}
	evm.SetTxContext(vm.TxContext{Origin: s.address}) // origin != caller

	c := storage.NewPrecompiledContract(s.app.StorageKeeper)
	_, err := c.CreateGroup(s.ctx, evm, contract, false)
	s.Require().EqualError(err, "only allow EOA can call this method")
}

// TestCreateGroup_FailureDoesNotMutateState pre-creates a group, then dispatches
// createGroup with the same name through the EVM keeper so the msg server fails
// with "Group already exists". It pins that a failed precompile call reverts
// cleanly (Contract.Run snapshot/revert) and leaves the existing group untouched.
func (s *CreateGroupTestSuite) TestCreateGroup_FailureDoesNotMutateState() {
	s.mustEnableStaticPrecompiles()

	const groupName = "baseline-group-dup"
	creator := sdk.AccAddress(s.address.Bytes())

	// Pre-create the group directly so the EVM dispatch collides with it.
	server := storagekeeper.NewMsgServerImpl(s.app.StorageKeeper)
	_, err := server.CreateGroup(s.ctx, &storagetypes.MsgCreateGroup{
		Creator:   creator.String(),
		GroupName: groupName,
	})
	s.Require().NoError(err)
	original, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, creator, groupName)
	s.Require().True(found)

	// Dispatch createGroup with the duplicate name — must fail and revert.
	input := s.mustPackCreateGroupInput(groupName, "")
	precompileAddr := storage.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().Error(err)
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	// Native mode surfaces the failure as an EVM revert; the underlying
	// "Group already exists" reason is ABI-encoded in the revert data.
	s.Require().Contains(err.Error(), "execution reverted")
	reason, uErr := abi.UnpackRevert(res.Ret)
	s.Require().NoError(uErr)
	s.Require().Contains(reason, "Group already exists")

	// The failed EVM call exhausts s.ctx's gas meter, so read post-call state
	// through a fresh context (mirrors the bank/storageprovider baselines). The
	// original group must be intact and unchanged.
	checkCtx := s.app.BaseApp.NewContext(false).
		WithBlockHeader(s.ctx.BlockHeader()).
		WithChainID(s.ctx.ChainID()).
		WithGasMeter(storetypes.NewInfiniteGasMeter()).
		WithBlockGasMeter(storetypes.NewInfiniteGasMeter())
	after, found := s.app.StorageKeeper.GetGroupInfo(checkCtx, creator, groupName)
	s.Require().True(found, "original group must still exist after failed duplicate create")
	s.Require().Equal(original.Id.String(), after.Id.String(), "failed create must not mutate the existing group")
}

func (s *CreateGroupTestSuite) mustEnableStaticPrecompiles() {
	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

func (s *CreateGroupTestSuite) mustPackCreateGroupInput(groupName, extra string) []byte {
	method := storage.GetAbiMethod(storage.CreateGroupMethodName)
	packedArgs, err := method.Inputs.Pack(groupName, extra)
	s.Require().NoError(err)
	return append(append([]byte{}, method.ID...), packedArgs...)
}
