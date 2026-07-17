package storage

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/mocachain/moca/v2/precompiles/types"
	storagekeeper "github.com/mocachain/moca/v2/x/storage/keeper"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile is the storage precompile. It follows the cosmos/evm precompile
// layout — Run -> RunNativeAction -> Execute -> cmn.SetupABI dispatch — so keeper
// coin moves stay reconciled with the EVM StateDB. The moca-specific surface is the
// hex (0x) address encoding, moca's storage method set, the ERC721 NFT Transfer
// mirror logs, and the non-payable RejectValue guard. The storage msg server
// executes the transactions and the storage keeper serves the read queries.
type Precompile struct {
	cmn.Precompile
	abi.ABI

	storageMsgServer storagetypes.MsgServer
	storageKeeper    storagekeeper.Keeper
}

// NewPrecompile creates a new storage Precompile as a vm.PrecompiledContract. The
// msg server is built from the storage keeper at wiring time; the storage keeper
// serves queries and the bank keeper reconciles coin moves with the EVM StateDB.
func NewPrecompile(
	storageMsgServer storagetypes.MsgServer,
	storageKeeper storagekeeper.Keeper,
	bankKeeper bankkeeper.Keeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      storageAddress,
			// Reconciles bank keeper coin moves with the EVM StateDB balances.
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:              storageABI,
		storageMsgServer: storageMsgServer,
		storageKeeper:    storageKeeper,
	}
}

// Address returns the precompile's fixed hex address.
func (p Precompile) Address() common.Address {
	return storageAddress
}

// RequiredGas calculates the base gas via the cosmos/evm common flat+per-byte model.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}

	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run dispatches the call through cosmos/evm's native-action protocol so keeper coin
// moves stay reconciled with the EVM StateDB: FlushToCacheCtx + the BalanceHandler
// translate the bank coin_spent/coin_received events into StateDB
// SubBalance/AddBalance, the multistore is snapshotted for atomic revert
// (AddPrecompileFn), and store gas is metered against contract.Gas. Without this,
// StateDB.Commit's balance reconciliation would mint a debited amount back to a
// 7702-dirtied caller (native-token inflation). moca precompiles are not payable, so
// any attached value is rejected up front.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if err := types.RejectValue(contract); err != nil {
		return types.PackRetError(err.Error())
	}
	if len(contract.Input) < 4 {
		return types.PackRetError("invalid input")
	}

	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm, contract, readonly)
	})
}

// Execute parses the calldata against the ABI and routes to the matching handler.
func (p Precompile) Execute(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte
	switch method.Name {
	// Storage transactions
	case CreateBucketMethodName:
		bz, err = p.CreateBucket(ctx, evm, contract, method, args)
	case DeleteBucketMethodName:
		bz, err = p.DeleteBucket(ctx, evm, contract, method, args)
	case DiscontinueBucketMethodName:
		bz, err = p.DiscontinueBucket(ctx, evm, contract, method, args)
	case MigrateBucketMethodName:
		bz, err = p.MigrateBucket(ctx, evm, contract, method, args)
	case CompleteMigrateBucketMethodName:
		bz, err = p.CompleteMigrateBucket(ctx, evm, contract, method, args)
	case RejectMigrateBucketMethodName:
		bz, err = p.RejectMigrateBucket(ctx, evm, contract, method, args)
	case CancelMigrateBucketMethodName:
		bz, err = p.CancelMigrateBucket(ctx, evm, contract, method, args)
	case SetBucketFlowRateLimitMethodName:
		bz, err = p.SetBucketFlowRateLimit(ctx, evm, contract, method, args)
	case CreateObjectMethodName:
		bz, err = p.CreateObject(ctx, evm, contract, method, args)
	case CopyObjectMethodName:
		bz, err = p.CopyObject(ctx, evm, contract, method, args)
	case DeleteObjectMethodName:
		bz, err = p.DeleteObject(ctx, evm, contract, method, args)
	case CancelCreateObjectMethodName:
		bz, err = p.CancelCreateObject(ctx, evm, contract, method, args)
	case SealObjectMethodName:
		bz, err = p.SealObject(ctx, evm, contract, method, args)
	case SealObjectV2MethodName:
		bz, err = p.SealObjectV2(ctx, evm, contract, method, args)
	case RejectSealObjectMethodName:
		bz, err = p.RejectSealObject(ctx, evm, contract, method, args)
	case DelegateCreateObjectMethodName:
		bz, err = p.DelegateCreateObject(ctx, evm, contract, method, args)
	case DelegateUpdateObjectContentMethodName:
		bz, err = p.DelegateUpdateObjectContent(ctx, evm, contract, method, args)
	case UpdateObjectInfoMethodName:
		bz, err = p.UpdateObjectInfo(ctx, evm, contract, method, args)
	case UpdateObjectContentMethodName:
		bz, err = p.UpdateObjectContent(ctx, evm, contract, method, args)
	case DiscontinueObjectMethodName:
		bz, err = p.DiscontinueObject(ctx, evm, contract, method, args)
	case CreateGroupMethodName:
		bz, err = p.CreateGroup(ctx, evm, contract, method, args)
	case UpdateGroupMethodName:
		bz, err = p.UpdateGroup(ctx, evm, contract, method, args)
	case UpdateGroupExtraMethodName:
		bz, err = p.UpdateGroupExtra(ctx, evm, contract, method, args)
	case DeleteGroupMethodName:
		bz, err = p.DeleteGroup(ctx, evm, contract, method, args)
	case LeaveGroupMethodName:
		bz, err = p.LeaveGroup(ctx, evm, contract, method, args)
	case RenewGroupMemberMethodName:
		bz, err = p.RenewGroupMember(ctx, evm, contract, method, args)
	case SetTagMethodName:
		bz, err = p.SetTag(ctx, evm, contract, method, args)
	case UpdateBucketInfoMethodName:
		bz, err = p.UpdateBucketInfo(ctx, evm, contract, method, args)
	case PutPolicyMethodName:
		bz, err = p.PutPolicy(ctx, evm, contract, method, args)
	case DeletePolicyMethodName:
		bz, err = p.DeletePolicy(ctx, evm, contract, method, args)
	case ToggleSPAsDelegatedAgentMethodName:
		bz, err = p.ToggleSPAsDelegatedAgent(ctx, evm, contract, method, args)
	case CancelUpdateObjectContentMethodName:
		bz, err = p.CancelUpdateObjectContent(ctx, evm, contract, method, args)
	// Storage queries
	case ListBucketsMethodName:
		bz, err = p.ListBuckets(ctx, method, args)
	case ListObjectsMethodName:
		bz, err = p.ListObjects(ctx, method, args)
	case ListGroupsMethodName:
		bz, err = p.ListGroups(ctx, method, args)
	case ListObjectsByBucketIDMethodName:
		bz, err = p.ListObjectsByBucketID(ctx, method, args)
	case HeadBucketMethodName:
		bz, err = p.HeadBucket(ctx, method, args)
	case HeadGroupMethodName:
		bz, err = p.HeadGroup(ctx, method, args)
	case HeadGroupMemberMethodName:
		bz, err = p.HeadGroupMember(ctx, method, args)
	case HeadObjectMethodName:
		bz, err = p.HeadObject(ctx, method, args)
	case HeadObjectByIDMethodName:
		bz, err = p.HeadObjectByID(ctx, method, args)
	case HeadBucketByIDMethodName:
		bz, err = p.HeadBucketByID(ctx, method, args)
	case HeadBucketNFTMethodName:
		bz, err = p.HeadBucketNFT(ctx, method, args)
	case HeadShadowObjectMethodName:
		bz, err = p.HeadShadowObject(ctx, method, args)
	case HeadObjectNFTMethodName:
		bz, err = p.HeadObjectNFT(ctx, method, args)
	case HeadGroupNFTMethodName:
		bz, err = p.HeadGroupNFT(ctx, method, args)
	case HeadBucketExtraMethodName:
		bz, err = p.HeadBucketExtra(ctx, method, args)
	case QueryPolicyForGroupMethodName:
		bz, err = p.QueryPolicyForGroup(ctx, method, args)
	case QueryPolicyForAccountMethodName:
		bz, err = p.QueryPolicyForAccount(ctx, method, args)
	case QueryParamsByTimestampMethodName:
		bz, err = p.QueryParamsByTimestamp(ctx, method, args)
	case QueryPolicyByIDMethodName:
		bz, err = p.QueryPolicyByID(ctx, method, args)
	case QueryLockFeeMethodName:
		bz, err = p.QueryLockFee(ctx, method, args)
	case QueryIsPriceChangedMethodName:
		bz, err = p.QueryIsPriceChanged(ctx, method, args)
	case QueryQuotaUpdateTimeMethodName:
		bz, err = p.QueryQuotaUpdateTime(ctx, method, args)
	case QueryGroupMembersExistMethodName:
		bz, err = p.QueryGroupMembersExist(ctx, method, args)
	case QueryGroupsExistMethodName:
		bz, err = p.QueryGroupsExist(ctx, method, args)
	case QueryGroupsExistByIDMethodName:
		bz, err = p.QueryGroupsExistByID(ctx, method, args)
	case QueryPaymentAccountBucketFlowRateLimitMethodName:
		bz, err = p.QueryPaymentAccountBucketFlowRateLimit(ctx, method, args)
	case ParamsMethodName:
		bz, err = p.Params(ctx, method, args)
	case VerifyPermissionMethodName:
		bz, err = p.VerifyPermission(ctx, contract, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case CreateBucketMethodName,
		DeleteBucketMethodName,
		DiscontinueBucketMethodName,
		MigrateBucketMethodName,
		CompleteMigrateBucketMethodName,
		RejectMigrateBucketMethodName,
		CancelMigrateBucketMethodName,
		SetBucketFlowRateLimitMethodName,
		CreateObjectMethodName,
		CopyObjectMethodName,
		DeleteObjectMethodName,
		CancelCreateObjectMethodName,
		SealObjectMethodName,
		SealObjectV2MethodName,
		RejectSealObjectMethodName,
		DelegateCreateObjectMethodName,
		DelegateUpdateObjectContentMethodName,
		UpdateObjectInfoMethodName,
		UpdateObjectContentMethodName,
		DiscontinueObjectMethodName,
		CreateGroupMethodName,
		UpdateGroupMethodName,
		UpdateGroupExtraMethodName,
		DeleteGroupMethodName,
		LeaveGroupMethodName,
		RenewGroupMemberMethodName,
		SetTagMethodName,
		UpdateBucketInfoMethodName,
		PutPolicyMethodName,
		DeletePolicyMethodName,
		ToggleSPAsDelegatedAgentMethodName,
		CancelUpdateObjectContentMethodName:
		return true
	default:
		return false
	}
}
