package types

import (
	"context"
	time "time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mocachain/moca/v2/types/resource"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
	GetSequence(context.Context, sdktypes.AccAddress) (uint64, error)
	GetAccount(ctx context.Context, addr sdktypes.AccAddress) sdktypes.AccountI
	GetModuleAddress(name string) sdktypes.AccAddress
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	SpendableCoins(ctx context.Context, addr sdktypes.AccAddress) sdktypes.Coins
	GetBalance(ctx context.Context, addr sdktypes.AccAddress, denom string) sdktypes.Coin
	GetAllBalances(ctx context.Context, addr sdktypes.AccAddress) sdktypes.Coins
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdktypes.AccAddress, amt sdktypes.Coins) error
	// Methods imported from bank should be defined here
}

type SpKeeper interface {
	GetStorageProvider(ctx sdktypes.Context, id uint32) (*sptypes.StorageProvider, bool)
	MustGetStorageProvider(ctx sdktypes.Context, id uint32) *sptypes.StorageProvider
	GetStorageProviderByOperatorAddr(ctx sdktypes.Context, addr sdktypes.AccAddress) (sp *sptypes.StorageProvider, found bool)
	GetStorageProviderBySealAddr(ctx sdktypes.Context, sealAddr sdktypes.AccAddress) (sp *sptypes.StorageProvider, found bool)
	GetStorageProviderByGcAddr(ctx sdktypes.Context, gcAddr sdktypes.AccAddress) (sp *sptypes.StorageProvider, found bool)
	GetGlobalSpStorePriceByTime(ctx sdktypes.Context, time int64) (val sptypes.GlobalSpStorePrice, err error)
}

type PaymentKeeper interface {
	GetVersionedParamsWithTs(ctx sdktypes.Context, time int64) (paymenttypes.VersionedParams, error)
	IsPaymentAccountOwner(ctx sdktypes.Context, addr, owner sdktypes.AccAddress) bool
	ApplyUserFlowsList(ctx sdktypes.Context, userFlows []paymenttypes.UserFlows) (err error)
	UpdateStreamRecordByAddr(ctx sdktypes.Context, change *paymenttypes.StreamRecordChange) (ret *paymenttypes.StreamRecord, err error)
	GetStreamRecord(ctx sdktypes.Context, account sdktypes.AccAddress) (ret *paymenttypes.StreamRecord, found bool)
	MergeOutFlows(flows []paymenttypes.OutFlow) []paymenttypes.OutFlow
	GetAllStreamRecord(ctx sdktypes.Context) (list []paymenttypes.StreamRecord)
	GetOutFlows(ctx sdktypes.Context, addr sdktypes.AccAddress) []paymenttypes.OutFlow
}

type PermissionKeeper interface {
	PutPolicy(ctx sdktypes.Context, policy *permtypes.Policy) (math.Uint, error)
	DeletePolicy(ctx sdktypes.Context, principal *permtypes.Principal, resourceType resource.ResourceType,
		resourceID math.Uint) (math.Uint, error)
	AddGroupMember(ctx sdktypes.Context, groupID math.Uint, member sdktypes.AccAddress, expiration *time.Time) error
	UpdateGroupMember(ctx sdktypes.Context, groupID math.Uint, member sdktypes.AccAddress, memberID math.Uint, expiration *time.Time)
	MustGetPolicyByID(ctx sdktypes.Context, policyID math.Uint) *permtypes.Policy
	GetPolicyGroupForResource(ctx sdktypes.Context, resourceID math.Uint, resourceType resource.ResourceType) (*permtypes.PolicyGroup, bool)
	RemoveGroupMember(ctx sdktypes.Context, groupID math.Uint, member sdktypes.AccAddress) error
	GetPolicyByID(ctx sdktypes.Context, policyID math.Uint) (*permtypes.Policy, bool)
	GetPolicyForAccount(ctx sdktypes.Context, resourceID math.Uint, resourceType resource.ResourceType, addr sdktypes.AccAddress) (policy *permtypes.Policy, isFound bool)
	GetPolicyForGroup(ctx sdktypes.Context, resourceID math.Uint, resourceType resource.ResourceType,
		groupID math.Uint) (policy *permtypes.Policy, isFound bool)
	GetGroupMember(ctx sdktypes.Context, groupID math.Uint, member sdktypes.AccAddress) (*permtypes.GroupMember, bool)
	GetGroupMemberByID(ctx sdktypes.Context, groupMemberID math.Uint) (*permtypes.GroupMember, bool)
	ForceDeleteAccountPolicyForResource(ctx sdktypes.Context, maxDelete, deletedCount uint64, resourceType resource.ResourceType, resourceID math.Uint) (uint64, bool)
	ForceDeleteGroupPolicyForResource(ctx sdktypes.Context, maxDelete, deletedCount uint64, resourceType resource.ResourceType, resourceID math.Uint) (uint64, bool)
	ForceDeleteGroupMembers(ctx sdktypes.Context, maxDelete, deletedTotal uint64, groupID math.Uint) (uint64, bool)
	ExistAccountPolicyForResource(ctx sdktypes.Context, resourceType resource.ResourceType, resourceID math.Uint) bool
	ExistGroupPolicyForResource(ctx sdktypes.Context, resourceType resource.ResourceType, resourceID math.Uint) bool
	ExistGroupMemberForGroup(ctx sdktypes.Context, groupID math.Uint) bool
}

type VirtualGroupKeeper interface {
	SetGVGAndEmitUpdateEvent(ctx sdktypes.Context, gvg *vgtypes.GlobalVirtualGroup) error
	GetGVGFamily(ctx sdktypes.Context, familyID uint32) (*vgtypes.GlobalVirtualGroupFamily, bool)
	GetGVG(ctx sdktypes.Context, gvgID uint32) (*vgtypes.GlobalVirtualGroup, bool)
	SettleAndDistributeGVGFamily(ctx sdktypes.Context, sp *sptypes.StorageProvider, family *vgtypes.GlobalVirtualGroupFamily) error
	SettleAndDistributeGVG(ctx sdktypes.Context, gvg *vgtypes.GlobalVirtualGroup) error
	GetAndCheckGVGFamilyAvailableForNewBucket(ctx sdktypes.Context, familyID uint32) (*vgtypes.GlobalVirtualGroupFamily, error)
	GetGlobalVirtualGroupIfAvailable(ctx sdktypes.Context, gvgID uint32, expectedStoreSize uint64) (*vgtypes.GlobalVirtualGroup, error)
	GetSwapInInfo(ctx sdktypes.Context, familyID, gvgID uint32) (*vgtypes.SwapInInfo, bool)
}

// StorageKeeper used by storage integrations
type StorageKeeper interface {
	Logger(ctx sdktypes.Context) log.Logger
	GetBucketInfoById(ctx sdktypes.Context, bucketID math.Uint) (*BucketInfo, bool)
	SetBucketInfo(ctx sdktypes.Context, bucketInfo *BucketInfo)
	CreateBucket(
		ctx sdktypes.Context, ownerAcc sdktypes.AccAddress, bucketName string,
		primarySpAcc sdktypes.AccAddress, opts *CreateBucketOptions) (math.Uint, error)
	DeleteBucket(ctx sdktypes.Context, operator sdktypes.AccAddress, bucketName string, opts DeleteBucketOptions) error
	GetGroupInfoById(ctx sdktypes.Context, groupID math.Uint) (*GroupInfo, bool)
	GetGroupInfo(ctx sdktypes.Context, ownerAddr sdktypes.AccAddress, groupName string) (*GroupInfo, bool)
	DeleteGroup(ctx sdktypes.Context, operator sdktypes.AccAddress, groupName string, opts DeleteGroupOptions) error
	CreateGroup(
		ctx sdktypes.Context, owner sdktypes.AccAddress,
		groupName string, opts CreateGroupOptions) (math.Uint, error)
	SetGroupInfo(ctx sdktypes.Context, groupInfo *GroupInfo)
	UpdateGroupMember(ctx sdktypes.Context, operator sdktypes.AccAddress, groupInfo *GroupInfo, opts UpdateGroupMemberOptions) error
	RenewGroupMember(ctx sdktypes.Context, operator sdktypes.AccAddress, groupInfo *GroupInfo, opts RenewGroupMemberOptions) error
	GetObjectInfoById(ctx sdktypes.Context, objectID math.Uint) (*ObjectInfo, bool)
	SetObjectInfo(ctx sdktypes.Context, objectInfo *ObjectInfo)
	DeleteObject(
		ctx sdktypes.Context, operator sdktypes.AccAddress, bucketName, objectName string, opts DeleteObjectOptions) error

	NormalizePrincipal(ctx sdktypes.Context, principal *permtypes.Principal)
	ValidatePrincipal(ctx sdktypes.Context, resOwner sdktypes.AccAddress, principal *permtypes.Principal) error
}

type PaymentMsgServer interface {
	CreatePaymentAccount(context.Context, *paymenttypes.MsgCreatePaymentAccount) (*paymenttypes.MsgCreatePaymentAccountResponse, error)
	Deposit(context.Context, *paymenttypes.MsgDeposit) (*paymenttypes.MsgDepositResponse, error)
	Withdraw(context.Context, *paymenttypes.MsgWithdraw) (*paymenttypes.MsgWithdrawResponse, error)
	DisableRefund(context.Context, *paymenttypes.MsgDisableRefund) (*paymenttypes.MsgDisableRefundResponse, error)
}

type StorageMsgServer interface {
	UpdateBucketInfo(context.Context, *MsgUpdateBucketInfo) (*MsgUpdateBucketInfoResponse, error)
	ToggleSPAsDelegatedAgent(context.Context, *MsgToggleSPAsDelegatedAgent) (*MsgToggleSPAsDelegatedAgentResponse, error)
	CopyObject(context.Context, *MsgCopyObject) (*MsgCopyObjectResponse, error)
	UpdateObjectInfo(context.Context, *MsgUpdateObjectInfo) (*MsgUpdateObjectInfoResponse, error)
	UpdateGroupExtra(context.Context, *MsgUpdateGroupExtra) (*MsgUpdateGroupExtraResponse, error)
	MigrateBucket(context.Context, *MsgMigrateBucket) (*MsgMigrateBucketResponse, error)
	CancelMigrateBucket(context.Context, *MsgCancelMigrateBucket) (*MsgCancelMigrateBucketResponse, error)
	SetTag(context.Context, *MsgSetTag) (*MsgSetTagResponse, error)
	SetBucketFlowRateLimit(context.Context, *MsgSetBucketFlowRateLimit) (*MsgSetBucketFlowRateLimitResponse, error)
}

// EVMKeeper defines the expected EVM keeper interface used by moca's
// storage module to call into ERC-20 / ERC-721 contracts during cross-chain
// mirroring. Only the methods storage actually invokes are listed; the
// ApplyMessage method declared in the pre-cosmos/evm version was dropped
// because cosmos/evm v0.6.0 changed its signature significantly (it now
// takes a *statedb.StateDB plus extra precompile / internal flags). Until
// CallEVM/CallEVMWithData in keeper/evm.go is rewired onto cosmos/evm's
// CallEVM helper, that path is stubbed and ApplyMessage is unused.
type EVMKeeper interface {
	GetParams(ctx sdktypes.Context) evmtypes.Params
	GetAccountWithoutBalance(ctx sdktypes.Context, addr common.Address) *statedb.Account
	EstimateGas(c context.Context, req *evmtypes.EthCallRequest) (*evmtypes.EstimateGasResponse, error)
}
