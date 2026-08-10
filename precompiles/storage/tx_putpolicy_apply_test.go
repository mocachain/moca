package storage_test

// Regression test for MOCA-963/964 () on top of the
// tx_evm_apply_test.go harness: the EVM storage precompile's putPolicy
// bypassed MsgPutPolicy.ValidateRuntime the same way the native tx path did,
// so a Statement with a Resources entry that fails regexp.Compile could be
// written to state through a smart contract call. The precompile's PutPolicy
// handler (precompiles/storage/tx.go) delegates to storageMsgServer.PutPolicy,
// the exact same keeper.msgServer implementation app.go wires up for the
// native Cosmos MsgServiceRouter (x/storage/module.go RegisterServices), so
// this drives the fix through the EVM entry point specifically rather than
// just asserting the wiring by inspection.

import (
	"math/big"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
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
	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/testutil"
	utiltx "github.com/mocachain/moca/v2/testutil/tx"
	gnfdresource "github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/utils"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

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
