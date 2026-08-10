package storage_test

// Regression/characterization tests for the storage precompile on top of the
// cosmos/evm native-action migration (#332). createGroup is the representative
// transactional write: its only precondition is a funded creator, so
// dispatch/EOA-only/failure behaviors can be pinned cheaply and deterministically.

import (
	"math/big"
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtestutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	sdkmath "cosmossdk.io/math"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/contracts"
	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	gnfdresource "github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/utils"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
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

	// createGroup mints a group NFT via an internal CallEVM whose sender is the
	// group control hub (contracts.GroupControlHubAddress, 0x...dead). EthSetup's
	// genesis does not create that account, so register it here or the mint fails
	// with "account 0x...dEaD does not exist".
	controlHub := sdk.AccAddress(contracts.GroupControlHubAddress.Bytes())
	s.app.AccountKeeper.SetAccount(s.ctx, s.app.AccountKeeper.NewAccountWithAddress(s.ctx, controlHub))
}

// TestCreateGroup_EVMDispatchSuccess drives createGroup end-to-end through the EVM
// keeper and asserts the static precompile was dispatched (a group was created,
// owned by the caller).
func (s *CreateGroupTestSuite) TestCreateGroup_EVMDispatchSuccess() {
	s.mustEnableStaticPrecompiles()

	const groupName = "regression-group-ok"
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

// TestCreateGroup_AllowsContractForwarding asserts that the immediate contract
// caller, rather than the transaction origin, owns a forwarded native action.
func (s *CreateGroupTestSuite) TestCreateGroup_AllowsContractForwarding() {
	caller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	const groupName = "regression-group-fwd"
	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, sdk.AccAddress(caller.Bytes()), 1))

	contract := vm.NewContract(caller, storage.GetAddress(), uint256.NewInt(0), 60_000, nil)
	contract.Input = s.mustPackCreateGroupInput(groupName, "")

	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	p := storage.NewPrecompile(storagekeeper.NewMsgServerImpl(s.app.StorageKeeper), s.app.StorageKeeper, s.app.BankKeeper)
	method := storage.MustMethod(storage.CreateGroupMethodName)
	args, err := method.Inputs.Unpack(contract.Input[4:])
	s.Require().NoError(err)
	_, err = p.CreateGroup(s.ctx, evm, contract, &method, args)
	s.Require().NoError(err)

	group, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, sdk.AccAddress(caller.Bytes()), groupName)
	s.Require().True(found)
	s.Require().Equal(sdk.AccAddress(caller.Bytes()).String(), group.Owner)
}

// TestCreateGroup_FailureDoesNotMutateState pre-creates a group, then dispatches
// createGroup with the same name so the msg server fails with "Group already
// exists". It pins a clean revert with the existing group untouched.
func (s *CreateGroupTestSuite) TestCreateGroup_FailureDoesNotMutateState() {
	s.mustEnableStaticPrecompiles()

	const groupName = "regression-group-dup"
	creator := sdk.AccAddress(s.address.Bytes())

	server := storagekeeper.NewMsgServerImpl(s.app.StorageKeeper)
	_, err := server.CreateGroup(s.ctx, &storagetypes.MsgCreateGroup{
		Creator:   creator.String(),
		GroupName: groupName,
	})
	s.Require().NoError(err)
	original, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, creator, groupName)
	s.Require().True(found)

	input := s.mustPackCreateGroupInput(groupName, "")
	precompileAddr := storage.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().Error(err)
	s.Require().NotNil(res)
	s.Require().True(res.Failed())
	s.Require().Contains(err.Error(), "execution reverted")
	reason, uErr := abi.UnpackRevert(res.Ret)
	s.Require().NoError(uErr)
	s.Require().Contains(reason, "Group already exists")

	// The failed EVM call exhausts s.ctx's gas meter, so read via a fresh context.
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

type PutPolicyPrecompileTestSuite struct {
	suite.Suite
	ctx        sdk.Context
	app        *app.Moca
	address    common.Address
	bucketName string
}

func TestPutPolicyPrecompileTestSuite(t *testing.T) {
	suite.Run(t, new(PutPolicyPrecompileTestSuite))
}

func (s *PutPolicyPrecompileTestSuite) SetupTest() {
	checkTx := false
	chainID := utils.TestnetChainID + "-1"

	s.app = app.EthSetup(checkTx, nil)
	s.ctx = s.app.NewContext(checkTx)
	s.address = common.HexToAddress("0x4444444444444444444444444444444444444444")
	s.bucketName = "putpolicy-precompile-bucket"

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

	// EthSetup deliberately builds its genesis from an empty simapp.GenesisState
	// (see NewTestGenesisState's doc comment) so that any module whose key is
	// absent - including x/permission here - has its InitGenesis skipped
	// entirely. That leaves GetParams() at its zero value (MaximumStatementsNum
	// = 0), which would make every PutPolicy call fail the new cap check
	// regardless of the regexp fix under test. Seed default params directly,
	// the same way the x/storage and x/permission keeper test suites already
	// do in their own SetupTest.
	s.Require().NoError(s.app.PermissionKeeper.SetParams(s.ctx, permtypes.DefaultParams()))

	// PutPolicy requires the caller to own the resource the policy attaches
	// to; seed the bucket directly rather than driving createBucket (which
	// pulls in SP/payment preconditions unrelated to this test).
	s.app.StorageKeeper.StoreBucketInfo(s.ctx, &storagetypes.BucketInfo{
		Owner:            sdk.AccAddress(s.address.Bytes()).String(),
		BucketName:       s.bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   sdk.AccAddress(s.address.Bytes()).String(),
		ChargedReadQuota: 100,
		BucketStatus:     storagetypes.BUCKET_STATUS_CREATED,
	})
}

// TestPutPolicy_RunsValidateRuntime drives the precompile's PutPolicy Go method
// directly (bypassing full EVM gas/dispatch machinery, same style as
// TestCreateGroup_AllowsContractForwarding in tx_evm_apply_test.go) with a
// Statement that clears ValidateBasic but not ValidateRuntime, and asserts it is
// rejected post-fix. A bucket-level statement naming a group-only action is such
// a case: ValidateBasic never consults BucketAllowedActionsAfterPampas.
func (s *PutPolicyPrecompileTestSuite) TestPutPolicy_RunsValidateRuntime() {
	principal := common.HexToAddress("0x5555555555555555555555555555555555555555")

	contract := vm.NewContract(s.address, storage.GetAddress(), uint256.NewInt(0), 200_000, nil)
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	method := storage.MustMethod(storage.PutPolicyMethodName)
	packed, err := method.Inputs.Pack(
		storage.Principal{
			PrincipalType: int32(permtypes.PRINCIPAL_TYPE_GNFD_ACCOUNT),
			Value:         sdk.AccAddress(principal.Bytes()).String(),
		},
		"grn:b::"+s.bucketName,
		[]storage.Statement{
			{
				Effect: int32(permtypes.EFFECT_ALLOW),
				// A group-only action on a bucket resource: accepted by
				// ValidateBasic, rejected by ValidateRuntime.
				Actions:   []int32{int32(permtypes.ACTION_UPDATE_GROUP_MEMBER)},
				Resources: []string{"grn:o::" + s.bucketName + "/obj"},
			},
		},
		int64(0),
	)
	s.Require().NoError(err)
	args, err := method.Inputs.Unpack(packed)
	s.Require().NoError(err)

	p := storage.NewPrecompile(storagekeeper.NewMsgServerImpl(s.app.StorageKeeper), s.app.StorageKeeper, s.app.BankKeeper)
	_, err = p.PutPolicy(s.ctx, evm, contract, &method, args)
	s.Require().Error(err, "the EVM precompile PutPolicy path must run MsgPutPolicy.ValidateRuntime")
	s.Require().ErrorIs(err, permtypes.ErrInvalidStatement)
}

// TestPutPolicy_AcceptsRegexpMetacharacterInObjectName pins that the same entry
// point still accepts a Resources entry that is a legal object name but not a
// legal Go regexp. Resources are wildcard patterns, not regexps, so such a name
// must be storable and must not be rejected by ValidateRuntime.
func (s *PutPolicyPrecompileTestSuite) TestPutPolicy_AcceptsRegexpMetacharacterInObjectName() {
	principal := common.HexToAddress("0x6666666666666666666666666666666666666666")

	contract := vm.NewContract(s.address, storage.GetAddress(), uint256.NewInt(0), 200_000, nil)
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	method := storage.MustMethod(storage.PutPolicyMethodName)
	packed, err := method.Inputs.Pack(
		storage.Principal{
			PrincipalType: int32(permtypes.PRINCIPAL_TYPE_GNFD_ACCOUNT),
			Value:         sdk.AccAddress(principal.Bytes()).String(),
		},
		"grn:b::"+s.bucketName,
		[]storage.Statement{
			{
				Effect:    int32(permtypes.EFFECT_ALLOW),
				Actions:   []int32{int32(permtypes.ACTION_GET_OBJECT)},
				Resources: []string{"grn:o::" + s.bucketName + "/obj["},
			},
		},
		int64(0),
	)
	s.Require().NoError(err)
	args, err := method.Inputs.Unpack(packed)
	s.Require().NoError(err)

	p := storage.NewPrecompile(storagekeeper.NewMsgServerImpl(s.app.StorageKeeper), s.app.StorageKeeper, s.app.BankKeeper)
	_, err = p.PutPolicy(s.ctx, evm, contract, &method, args)
	s.Require().NoError(err, "a legal object name that is not a legal regexp must remain storable")

	_, found := s.app.PermissionKeeper.GetPolicyForAccount(s.ctx, sdkmath.NewUint(1),
		gnfdresource.RESOURCE_TYPE_BUCKET, sdk.AccAddress(principal.Bytes()))
	s.Require().True(found, "the policy must actually be persisted")
}
