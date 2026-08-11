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
	"github.com/stretchr/testify/require"
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
	"github.com/cosmos/evm/x/vm/store/snapshotmulti"
	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/contracts"
	"github.com/mocachain/moca/v2/internal/sequence"
	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/testutil"
	"github.com/mocachain/moca/v2/testutil/sample"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	gnfdresource "github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/utils"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
	"github.com/stretchr/testify/require"
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

// TestPutPolicy_ObjectScopedWithNoResources drives an object-scoped policy
// through the precompile with no Resources, which is how moca-cmd and any EVM
// caller submit one.
//
// The statement goes through a real ABI Pack/Unpack round-trip, so Resources
// arrives the way the EVM path actually delivers it: go-ethereum decodes a
// zero-length dynamic array to an empty non-nil slice. ValidateRuntime tested
// Resources != nil rather than its length, so every object- and group-scoped
// PutPolicy submitted over the precompile was rejected and the transaction
// reverted, while the same policy over the native tx path succeeded (protobuf
// decodes an absent repeated field to nil).
func (s *PutPolicyPrecompileTestSuite) TestPutPolicy_ObjectScopedWithNoResources() {
	principal := common.HexToAddress("0x7777777777777777777777777777777777777777")
	objectName := "putpolicy-precompile-object"

	s.app.StorageKeeper.StoreObjectInfo(s.ctx, &storagetypes.ObjectInfo{
		Owner:       sdk.AccAddress(s.address.Bytes()).String(),
		BucketName:  s.bucketName,
		ObjectName:  objectName,
		Id:          sdkmath.NewUint(2),
		PayloadSize: 1,
	})

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
		"grn:o::"+s.bucketName+"/"+objectName,
		[]storage.Statement{
			{
				Effect:  int32(permtypes.EFFECT_ALLOW),
				Actions: []int32{int32(permtypes.ACTION_GET_OBJECT)},
				// No Resources: an object-scoped policy names its resource in
				// the GRN. The ABI round-trip turns this into []string{}.
				Resources: []string{},
			},
		},
		int64(0),
	)
	s.Require().NoError(err)
	args, err := method.Inputs.Unpack(packed)
	s.Require().NoError(err)

	p := storage.NewPrecompile(storagekeeper.NewMsgServerImpl(s.app.StorageKeeper), s.app.StorageKeeper, s.app.BankKeeper)
	_, err = p.PutPolicy(s.ctx, evm, contract, &method, args)
	s.Require().NoError(err, "an object-scoped policy with no Resources must be storable over the EVM precompile")
}

// An object belongs to a global virtual group only once it is sealed, so the query returns none
// for one that is not. The ABI tuple cannot be absent, so it comes back zeroed rather than
// faulting; the object status returned alongside it says why.
func TestHeadObject_UnsealedObjectHasNoVirtualGroup(t *testing.T) {
	mocaApp := app.EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)
	store := ctx.KVStore(mocaApp.GetKey(storagetypes.StoreKey))

	owner := sample.RandAccAddress()
	bucketName, objectName := "unsealedbucket", "unsealedobject"
	bucketID, objectID := sdkmath.NewUint(1), sdkmath.NewUint(1)

	mocaApp.StorageKeeper.SetBucketInfo(ctx, &storagetypes.BucketInfo{
		Id: bucketID, Owner: owner.String(), BucketName: bucketName,
	})
	store.Set(storagetypes.GetBucketKey(bucketName),
		sequence.Sequence[sdkmath.Uint]{}.EncodeSequence(bucketID))

	mocaApp.StorageKeeper.SetObjectInfo(ctx, &storagetypes.ObjectInfo{
		Id: objectID, Owner: owner.String(), BucketName: bucketName, ObjectName: objectName,
		ObjectStatus: storagetypes.OBJECT_STATUS_CREATED,
	})
	store.Set(storagetypes.GetObjectKey(bucketName, objectName),
		sequence.Sequence[sdkmath.Uint]{}.EncodeSequence(objectID))

	res, err := mocaApp.StorageKeeper.HeadObject(ctx, &storagetypes.QueryHeadObjectRequest{
		BucketName: bucketName, ObjectName: objectName,
	})
	require.NoError(t, err)
	require.Nil(t, res.GlobalVirtualGroup, "an unsealed object has no group")

	p := storage.NewPrecompile(
		storagekeeper.NewMsgServerImpl(mocaApp.StorageKeeper), mocaApp.StorageKeeper, mocaApp.BankKeeper)

	for _, name := range []string{"headObject", "headObjectById"} {
		m := p.Methods[name]
		var (
			out []byte
			err error
		)
		require.NotPanics(t, func() {
			switch name {
			case "headObject":
				out, err = p.HeadObject(ctx, &m, []interface{}{bucketName, objectName})
			default:
				out, err = p.HeadObjectByID(ctx, &m, []interface{}{objectID.String()})
			}
		}, "%s must not fault on an unsealed object", name)
		require.NoError(t, err, name)
		require.NotEmpty(t, out, name)
	}
}

type DeleteGCBookkeepingTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *app.Moca
	address common.Address
}

func TestDeleteGCBookkeepingTestSuite(t *testing.T) {
	suite.Run(t, new(DeleteGCBookkeepingTestSuite))
}

func (s *DeleteGCBookkeepingTestSuite) SetupTest() {
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
	// "account 0x...dEaD does not exist" before we ever reach the code under test.
	controlHub := sdk.AccAddress(contracts.GroupControlHubAddress.Bytes())
	s.app.AccountKeeper.SetAccount(s.ctx, s.app.AccountKeeper.NewAccountWithAddress(s.ctx, controlHub))

	evmParams := s.app.EvmKeeper.GetParams(s.ctx)
	evmParams.EvmDenom = utils.BaseDenom
	evmParams.ActiveStaticPrecompiles = app.MocaActiveStaticPrecompiles()
	s.Require().NoError(s.app.EvmKeeper.SetParams(s.ctx, evmParams))

	// this context does not carry the permission module's genesis params, which
	// leaves the statement cap reading zero and rejects every policy write.
	s.Require().NoError(s.app.PermissionKeeper.SetParams(s.ctx, permtypes.DefaultParams()))
}

// callPrecompile runs input against the storage precompile through the real EVM
// entry point, so the keeper sees the snapshot multi-store rather than the
// plain block context.
func (s *DeleteGCBookkeepingTestSuite) callPrecompile(input []byte) {
	precompileAddr := storage.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, true, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "precompile call failed: %s", res.VmError)
}

func (s *DeleteGCBookkeepingTestSuite) packed(method string, args ...interface{}) []byte {
	m := storage.GetAbiMethod(method)
	packed, err := m.Inputs.Pack(args...)
	s.Require().NoError(err)
	return append(append([]byte{}, m.ID...), packed...)
}

// createGroupWithMember leaves the group in the state that makes DeleteGroup
// reach the GC bookkeeping: a group that still has at least one member. It
// returns the group id and the member, so callers can assert on the stale
// membership the GC pipeline is supposed to reap.
func (s *DeleteGCBookkeepingTestSuite) createGroupWithMember(groupName string) (sdkmath.Uint, sdk.AccAddress) {
	s.callPrecompile(s.packed(storage.CreateGroupMethodName, groupName, ""))

	member := common.HexToAddress("0x2222222222222222222222222222222222222222")
	s.callPrecompile(s.packed(storage.UpdateGroupMethodName,
		s.address, groupName, []common.Address{member}, []int64{0}, []common.Address{},
	))

	group, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, sdk.AccAddress(s.address.Bytes()), groupName)
	s.Require().True(found)
	memberAddr := sdk.AccAddress(member.Bytes())
	_, found = s.app.PermissionKeeper.GetGroupMember(s.ctx, group.Id, memberAddr)
	s.Require().True(found, "member must be recorded before delete, or the test does not reach the GC bookkeeping")
	return group.Id, memberAddr
}

const (
	testGVGFamilyID = uint32(1)
	testGVGID       = uint32(1)
	testLVGID       = uint32(1)
	// module params have to predate the resources charged against them, and an
	// object has to be older than the reserve time or its delete bills for the
	// unused remainder and drags the whole payment stack into the fixture.
	paramsAge   = 400 * 24 * time.Hour
	resourceAge = 365 * 24 * time.Hour
)

// resourceTime is the creation time shared by the planted bucket and object.
func (s *DeleteGCBookkeepingTestSuite) resourceTime() int64 {
	return s.ctx.BlockTime().Add(-resourceAge).Unix()
}

// setupVirtualGroups plants the primary SP's family, group and a zero price, which
// is what the object charge path walks before the delete itself runs. Zero prices
// keep the flow list empty so no stream record has to exist.
func (s *DeleteGCBookkeepingTestSuite) setupVirtualGroups() {
	paramsCtx := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-paramsAge))
	s.Require().NoError(s.app.StorageKeeper.SetParams(paramsCtx, storagetypes.DefaultParams()))
	s.Require().NoError(s.app.PaymentKeeper.SetParams(paramsCtx, paymenttypes.DefaultParams()))

	paymentAddr := sample.RandAccAddress().String()
	s.app.VirtualgroupKeeper.SetGVGFamily(s.ctx, &vgtypes.GlobalVirtualGroupFamily{
		Id:                    testGVGFamilyID,
		PrimarySpId:           1,
		GlobalVirtualGroupIds: []uint32{testGVGID},
		VirtualPaymentAddress: paymentAddr,
	})
	s.app.VirtualgroupKeeper.SetGVG(s.ctx, &vgtypes.GlobalVirtualGroup{
		Id:                    testGVGID,
		FamilyId:              testGVGFamilyID,
		PrimarySpId:           1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.app.SpKeeper.SetGlobalSpStorePrice(s.ctx, sptypes.GlobalSpStorePrice{
		UpdateTimeSec:       s.ctx.BlockTime().Add(-paramsAge).Unix(),
		ReadPrice:           sdkmath.LegacyZeroDec(),
		PrimaryStorePrice:   sdkmath.LegacyZeroDec(),
		SecondaryStorePrice: sdkmath.LegacyZeroDec(),
	})
}

// grantPolicyOn leaves a policy on a resource held by an account other than the
// owner. A group reaches the GC bookkeeping on membership alone, but a bucket or
// an object only reaches it while a policy still refers to it.
func (s *DeleteGCBookkeepingTestSuite) grantPolicyOn(resourceType gnfdresource.ResourceType, id sdkmath.Uint, action permtypes.ActionType) {
	grantee := sdk.AccAddress(common.HexToAddress("0x3333333333333333333333333333333333333333").Bytes())
	_, err := s.app.PermissionKeeper.PutPolicy(s.ctx, &permtypes.Policy{
		Principal:    permtypes.NewPrincipalWithAccount(grantee),
		ResourceType: resourceType,
		ResourceId:   id,
		Statements: []*permtypes.Statement{{
			Effect:  permtypes.EFFECT_ALLOW,
			Actions: []permtypes.ActionType{action},
		}},
	})
	s.Require().NoError(err)
	s.Require().True(s.app.PermissionKeeper.ExistAccountPolicyForResource(s.ctx, resourceType, id),
		"the policy must be recorded, or the delete takes the early return and never reaches the bookkeeping")
}

// createBucketWithPolicy writes the least bucket state DeleteBucket accepts: no
// objects and no charged read quota, which is what lets the bucket case skip the
// virtual-group fixture entirely. Pass lvgs only when an object needs one.
func (s *DeleteGCBookkeepingTestSuite) createBucketWithPolicy(bucketName string, bucketID sdkmath.Uint, lvgs []*storagetypes.LocalVirtualGroup) {
	owner := sdk.AccAddress(s.address.Bytes())
	s.app.StorageKeeper.SetBucketInfo(s.ctx, &storagetypes.BucketInfo{
		Owner:                      owner.String(),
		BucketName:                 bucketName,
		Id:                         bucketID,
		PaymentAddress:             owner.String(),
		Visibility:                 storagetypes.VISIBILITY_TYPE_PRIVATE,
		SourceType:                 storagetypes.SOURCE_TYPE_ORIGIN,
		BucketStatus:               storagetypes.BUCKET_STATUS_CREATED,
		ChargedReadQuota:           0,
		GlobalVirtualGroupFamilyId: testGVGFamilyID,
	})
	s.app.StorageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &storagetypes.InternalBucketInfo{
		PriceTime:          s.resourceTime(),
		LocalVirtualGroups: lvgs,
	})

	// SetBucketInfo only writes the by-id record; the name index is what a lookup
	// by bucket name resolves through.
	store := s.ctx.KVStore(s.app.GetKey(storagetypes.StoreKey))
	bucketSeq := sequence.NewSequence[sdkmath.Uint](storagetypes.BucketSequencePrefix)
	store.Set(storagetypes.GetBucketKey(bucketName), bucketSeq.EncodeSequence(bucketID))

	s.grantPolicyOn(gnfdresource.RESOURCE_TYPE_BUCKET, bucketID, permtypes.ACTION_UPDATE_BUCKET_INFO)
}

// createObjectWithPolicy seals an empty object into an existing bucket. Sealed is
// the status that reaches DeleteObject proper; a created object is routed to
// CancelCreateObject instead, which is a different path.
func (s *DeleteGCBookkeepingTestSuite) createObjectWithPolicy(bucketName, objectName string, objectID sdkmath.Uint) {
	owner := sdk.AccAddress(s.address.Bytes())
	s.app.StorageKeeper.SetObjectInfo(s.ctx, &storagetypes.ObjectInfo{
		Owner:               owner.String(),
		Creator:             owner.String(),
		BucketName:          bucketName,
		ObjectName:          objectName,
		Id:                  objectID,
		PayloadSize:         0,
		Visibility:          storagetypes.VISIBILITY_TYPE_PRIVATE,
		ObjectStatus:        storagetypes.OBJECT_STATUS_SEALED,
		SourceType:          storagetypes.SOURCE_TYPE_ORIGIN,
		LocalVirtualGroupId: testLVGID,
		CreateAt:            s.resourceTime(),
	})

	store := s.ctx.KVStore(s.app.GetKey(storagetypes.StoreKey))
	objectSeq := sequence.NewSequence[sdkmath.Uint](storagetypes.ObjectSequencePrefix)
	store.Set(storagetypes.GetObjectKey(bucketName, objectName), objectSeq.EncodeSequence(objectID))

	s.grantPolicyOn(gnfdresource.RESOURCE_TYPE_OBJECT, objectID, permtypes.ACTION_GET_OBJECT)
}

// TestDeleteBucket_WithPolicy_ThroughPrecompile covers the second of the three
// delete paths that reach the shared bookkeeping. The resource type only selects
// a branch of the switch, but deleteBucket is a distinct precompile entry point
// and its own gate condition, so it is pinned separately from deleteGroup.
func (s *DeleteGCBookkeepingTestSuite) TestDeleteBucket_WithPolicy_ThroughPrecompile() {
	const bucketName = "gc-bookkeeping-bucket"
	s.createBucketWithPolicy(bucketName, sdkmath.NewUint(101), nil)

	s.callPrecompile(s.packed(storage.DeleteBucketMethodName, bucketName))

	_, found := s.app.StorageKeeper.GetBucketInfo(s.ctx, bucketName)
	s.Require().False(found, "bucket should be deleted")
	s.Require().NotNil(s.currentBlockDeleteInfo(), "delete-GC bookkeeping should have been recorded")
}

// TestDeleteObject_WithPolicy_ThroughPrecompile covers the third path.
func (s *DeleteGCBookkeepingTestSuite) TestDeleteObject_WithPolicy_ThroughPrecompile() {
	const bucketName = "gc-bookkeeping-obj-bucket"
	const objectName = "gc-bookkeeping-object"
	s.setupVirtualGroups()
	s.createBucketWithPolicy(bucketName, sdkmath.NewUint(201), []*storagetypes.LocalVirtualGroup{
		{Id: testLVGID, GlobalVirtualGroupId: testGVGID},
	})
	s.createObjectWithPolicy(bucketName, objectName, sdkmath.NewUint(202))

	s.callPrecompile(s.packed(storage.DeleteObjectMethodName, bucketName, objectName))

	_, found := s.app.StorageKeeper.GetObjectInfo(s.ctx, bucketName, objectName)
	s.Require().False(found, "object should be deleted")
	s.Require().NotNil(s.currentBlockDeleteInfo(), "delete-GC bookkeeping should have been recorded")
}

func (s *DeleteGCBookkeepingTestSuite) currentBlockDeleteInfo() []byte {
	store := s.ctx.KVStore(s.app.GetKey(storagetypes.StoreKey))
	return store.Get(storagetypes.CurrentBlockDeleteStalePoliciesKey)
}

// TestDeleteGroup_WithMember_ThroughPrecompile is the regression for the panic:
// deleting a group that still has members reaches the delete-GC bookkeeping,
// which used to panic inside the EVM's precompile snapshot store.
func (s *DeleteGCBookkeepingTestSuite) TestDeleteGroup_WithMember_ThroughPrecompile() {
	const groupName = "gc-bookkeeping-group"
	s.createGroupWithMember(groupName)

	s.callPrecompile(s.packed(storage.DeleteGroupMethodName, groupName))

	_, found := s.app.StorageKeeper.GetGroupInfo(s.ctx, sdk.AccAddress(s.address.Bytes()), groupName)
	s.Require().False(found, "group should be deleted")

	// Guard against the test silently passing through appendResourceIDForGarbageCollection's
	// early return: the bookkeeping must actually have been written.
	s.Require().NotNil(s.currentBlockDeleteInfo(), "delete-GC bookkeeping should have been recorded")
}

// TestDeleteGroup_BookkeepingDrainedByEndBlocker pins the invariant that keeps
// this off the app hash: the bookkeeping key is written during the block and
// removed again by EndBlocker, so it never reaches committed state.
func (s *DeleteGCBookkeepingTestSuite) TestDeleteGroup_BookkeepingDrainedByEndBlocker() {
	// EthSetup leaves x/storage params at their zero value, which would send
	// EndBlocker down its DiscontinueDeletionMax == 0 early return; the full
	// path is the one under test here.
	s.Require().NoError(s.app.StorageKeeper.SetParams(s.ctx, storagetypes.DefaultParams()))

	const groupName = "gc-drain-group"
	groupID, member := s.createGroupWithMember(groupName)

	s.callPrecompile(s.packed(storage.DeleteGroupMethodName, groupName))
	s.Require().NotNil(s.currentBlockDeleteInfo(), "bookkeeping should exist before EndBlocker")

	s.Require().NoError(storagekeeper.EndBlocker(s.ctx, s.app.StorageKeeper))

	s.Require().Nil(s.currentBlockDeleteInfo(), "EndBlocker must drain the bookkeeping key")

	// The bookkeeping is only worth keeping if it actually drives the cleanup:
	// the stale membership of the deleted group must be gone.
	_, found := s.app.PermissionKeeper.GetGroupMember(s.ctx, groupID, member)
	s.Require().False(found, "stale group membership should have been garbage collected")
}

// TestDeleteGroup_BookkeepingDrainedWhenEndBlockerReturnsEarly covers
// EndBlocker's early returns, which previously relied on the transient store
// being thrown away at the end of the block. With DiscontinueDeletionMax == 0
// EndBlocker bails out before persisting, and the key must still be gone.
func (s *DeleteGCBookkeepingTestSuite) TestDeleteGroup_BookkeepingDrainedWhenEndBlockerReturnsEarly() {
	s.Require().Zero(s.app.StorageKeeper.DiscontinueDeletionMax(s.ctx),
		"this case relies on EndBlocker taking its DiscontinueDeletionMax == 0 early return")

	const groupName = "gc-drain-early-return-group"
	s.createGroupWithMember(groupName)

	s.callPrecompile(s.packed(storage.DeleteGroupMethodName, groupName))
	s.Require().NotNil(s.currentBlockDeleteInfo(), "bookkeeping should exist before EndBlocker")

	s.Require().NoError(storagekeeper.EndBlocker(s.ctx, s.app.StorageKeeper))

	s.Require().Nil(s.currentBlockDeleteInfo(),
		"EndBlocker must drain the bookkeeping key even when it returns early")
}

// TestStorageStoreIsSnapshottedForPrecompiles pins both halves of the store
// choice. The x/storage KV key is in the map cosmos/evm builds the precompile
// snapshot store from, so a reverted frame rolls the bookkeeping back; the
// transient key structurally cannot be there (the map is
// map[string]*storetypes.KVStoreKey), which is what used to panic.
func TestStorageStoreIsSnapshottedForPrecompiles(t *testing.T) {
	a := app.EthSetup(false, nil)

	keys := a.EvmKeeper.KVStoreKeys()
	_, ok := keys[storagetypes.StoreKey]
	require.True(t, ok, "x/storage KV store key must be in the precompile snapshot store")
	_, tOK := keys[storagetypes.TStoreKey]
	require.False(t, tOK, "a transient key can never be in the precompile snapshot store")

	cms := a.CommitMultiStore().CacheMultiStore()
	snap := snapshotmulti.NewStore(cms, keys)
	storeKey := a.GetKey(storagetypes.StoreKey)
	blob := []byte{0x0a, 0x00, 0x12, 0x00, 0x1a, 0x03, 0x0a, 0x01, 0x37}

	idx := snap.Snapshot()
	snap.GetKVStore(storeKey).Set(storagetypes.CurrentBlockDeleteStalePoliciesKey, blob)
	require.NotNil(t, snap.GetKVStore(storeKey).Get(storagetypes.CurrentBlockDeleteStalePoliciesKey))

	snap.RevertToSnapshot(idx)
	require.Nil(t, snap.GetKVStore(storeKey).Get(storagetypes.CurrentBlockDeleteStalePoliciesKey),
		"a reverted precompile frame must roll the delete-GC bookkeeping back")

	snap.Snapshot()
	snap.GetKVStore(storeKey).Set(storagetypes.CurrentBlockDeleteStalePoliciesKey, blob)
	snap.Write()
	require.NotNil(t, cms.GetKVStore(storeKey).Get(storagetypes.CurrentBlockDeleteStalePoliciesKey),
		"a committed precompile frame must keep it for EndBlocker to drain")
}
