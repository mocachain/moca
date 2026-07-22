package storage_test

// Regression test documenting a known bug: deleting a group that still has
// members panics deep inside cosmos/evm's precompile-execution snapshot
// store, because moca's x/storage transient store ("transient_storage",
// touched by appendResourceIDForGarbageCollection whenever the deleted
// resource has an existing policy, or -- for groups -- still has members)
// is never mounted into the scoped multi-store the EVM builds around
// precompile calls. cosmos/evm's own EVM execution loop recovers the panic
// and surfaces it as an ordinary EVM revert, so this is observable as a
// failed precompile call, not a node crash -- see the assertions below.
//
// Root cause: cosmos/evm's StateDB.cache() (x/vm/statedb/statedb.go) builds
// that snapshot multi-store from Keeper.KVStoreKeys(), which is typed
// map[string]*storetypes.KVStoreKey -- structurally incapable of holding a
// TransientStoreKey. moca's app.go correctly registers x/storage's
// TransientStoreKey at the root CommitMultiStore, but only ever injects the
// regular KVStoreKeys into the EVM keeper, so that registration never
// reaches the snapshot precompile calls execute inside. This is dormant in
// x/virtualgroup and x/challenge too (both hold a TStoreKey; virtualgroup's
// isn't used yet per PR #281's own commit message).
//
// Not fixed here -- this test exists to pin the exact failure mode so a fix
// (either threading transient keys into the EVM keeper's snapshot
// construction, or moving x/storage's GC bookkeeping off a transient store)
// has a concrete regression test to flip once it lands.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtestutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/contracts"
	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	"github.com/mocachain/moca/v2/utils"
)

type DeleteGroupTransientStoreTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestDeleteGroupTransientStoreTestSuite(t *testing.T) {
	suite.Run(t, new(DeleteGroupTransientStoreTestSuite))
}

func (s *DeleteGroupTransientStoreTestSuite) SetupTest() {
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

	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(s.address.Bytes()), 1_000_000_000_000))

	// createGroup mints a group NFT via an internal CallEVM whose sender is
	// the group control hub; register that account or the mint fails with
	// "account 0x...dEaD does not exist" before we ever reach the bug.
	controlHub := sdk.AccAddress(contracts.GroupControlHubAddress.Bytes())
	s.app.AccountKeeper.SetAccount(s.ctx, s.app.AccountKeeper.NewAccountWithAddress(s.ctx, controlHub))

	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))
}

// TestDeleteGroup_WithMember_PanicsOnTransientStore reproduces the bug: a
// group with at least one member reaches appendResourceIDForGarbageCollection's
// tStore branch, which panics with "has not been registered in stores"
// because the snapshot multi-store built around this precompile call never
// mounted x/storage's TransientStoreKey. cosmos/evm's EVM execution loop
// recovers that panic into an ordinary revert.
func (s *DeleteGroupTransientStoreTestSuite) TestDeleteGroup_WithMember_PanicsOnTransientStore() {
	const groupName = "transient-store-bug-group"
	owner := sdk.AccAddress(s.address.Bytes())
	precompileAddr := storage.GetAddress()

	createMethod := storage.GetAbiMethod(storage.CreateGroupMethodName)
	createArgs, err := createMethod.Inputs.Pack(groupName, "")
	s.Require().NoError(err)
	createInput := append(append([]byte{}, createMethod.ID...), createArgs...)

	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, createInput, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "createGroup should succeed: %s", res.VmError)

	member := common.HexToAddress("0x2222222222222222222222222222222222222222")
	updateMethod := storage.GetAbiMethod(storage.UpdateGroupMethodName)
	updateArgs, err := updateMethod.Inputs.Pack(
		s.address, groupName, []common.Address{member}, []int64{0}, []common.Address{},
	)
	s.Require().NoError(err)
	updateInput := append(append([]byte{}, updateMethod.ID...), updateArgs...)

	stateDB = statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err = s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, updateInput, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "updateGroup (add member) should succeed: %s", res.VmError)

	group, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, owner, groupName)
	s.Require().True(found)
	_, found = s.app.PermissionKeeper.GetGroupMember(s.ctx, group.Id, sdk.AccAddress(member.Bytes()))
	s.Require().True(found, "member must actually be recorded before delete, or this test doesn't reach the bug")

	deleteMethod := storage.GetAbiMethod(storage.DeleteGroupMethodName)
	deleteArgs, err := deleteMethod.Inputs.Pack(groupName)
	s.Require().NoError(err)
	deleteInput := append(append([]byte{}, deleteMethod.ID...), deleteArgs...)

	// This does NOT surface as a clean VM revert at this level. cosmos/evm's
	// own precompile-call recovery (common.HandleGasError) only catches
	// storetypes.ErrorOutOfGas and re-panics everything else
	// (precompiles/common/precompile.go's `default: panic(r)` branch), so a
	// non-gas panic like this one propagates raw out of CallEVMWithData. In
	// a real chain it's baseapp's own top-level runTx recover that catches
	// it further up and turns it into an ordinary failed transaction --
	// demonstrated separately against a live localup chain, where it
	// presents as "tx reverted; revert reason: execution reverted: kv store
	// with key TransientStoreKey{...} has not been registered in stores".
	//
	// The panic value's exact text embeds a runtime pointer
	// (TransientStoreKey{0x<addr>, transient_storage}), so it's asserted by
	// substring rather than exact value.
	stateDB = statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	recovered := func() (r any) {
		defer func() { r = recover() }()
		_, _ = s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, deleteInput, true, false, nil)
		return nil
	}()
	s.Require().NotNil(recovered, "deleteGroup on a group with a member currently panics on the missing transient-store "+
		"registration -- if this now completes cleanly, the underlying gap has been fixed and this test should be "+
		"rewritten to assert success instead")
	msg, ok := recovered.(string)
	s.Require().True(ok, "expected a string panic value, got %T: %v", recovered, recovered)
	s.Require().Contains(msg, "transient_storage")
	s.Require().Contains(msg, "has not been registered in stores")
}
