package storage

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	storageAddress = common.HexToAddress(types.StorageAddress)
	storageABI     = types.MustABIJson(IStorageMetaData.ABI)
)

// GetAddress returns the storage precompile's fixed hex address.
func GetAddress() common.Address {
	return storageAddress
}

// GetMethod resolves an ABI method by name.
func GetMethod(name string) (abi.Method, error) {
	method := storageABI.Methods[name]
	if method.ID == nil {
		return abi.Method{}, fmt.Errorf("method %s is not exist", name)
	}
	return method, nil
}

// MustMethod resolves an ABI method by name and panics if it does not exist.
func MustMethod(name string) abi.Method {
	method, err := GetMethod(name)
	if err != nil {
		panic(err)
	}
	return method
}

// GetAbiMethod resolves an ABI method by name (zero method if absent).
func GetAbiMethod(name string) abi.Method {
	return storageABI.Methods[name]
}

// GetEvent resolves an ABI event by name.
func GetEvent(name string) (abi.Event, error) {
	event := storageABI.Events[name]
	if event.ID == (common.Hash{}) {
		return abi.Event{}, fmt.Errorf("event %s is not exist", name)
	}
	return event, nil
}

// MustEvent resolves an ABI event by name and panics if it does not exist.
func MustEvent(name string) abi.Event {
	event, err := GetEvent(name)
	if err != nil {
		panic(err)
	}
	return event
}

// GetAbiEvent resolves an ABI event by name (zero event if absent).
func GetAbiEvent(name string) abi.Event {
	return storageABI.Events[name]
}

// The arg structs below are decode targets for cmn.SetupABI's positional args via
// abi.Arguments.Copy; their fields carry the ABI names. The keeper's msg
// ValidateBasic validates message contents, so these carry no extra validation
// except where a handler inlines a check the original enforced.

// CreateBucketArgs is the decode target for the createBucket calldata.
type CreateBucketArgs struct {
	BucketName        string         `abi:"bucketName"`
	Visibility        uint8          `abi:"visibility"`
	PaymentAddress    common.Address `abi:"paymentAddress"`
	PrimarySpAddress  common.Address `abi:"primarySpAddress"`
	PrimarySpApproval Approval       `abi:"primarySpApproval"`
	ChargedReadQuota  uint64         `abi:"chargedReadQuota"`
}

// UpdateBucketInfoArgs is the decode target for the updateBucketInfo calldata.
type UpdateBucketInfoArgs struct {
	BucketName       string         `abi:"bucketName"`
	ChargedReadQuota *big.Int       `abi:"chargedReadQuota"`
	PaymentAddress   common.Address `abi:"paymentAddress"`
	Visibility       uint8          `abi:"visibility"`
}

// ListBucketsArgs is the decode target for the listBuckets calldata.
type ListBucketsArgs struct {
	Pagination PageRequest `abi:"pagination"`
}

// HeadBucketArgs is the decode target for the headBucket calldata.
type HeadBucketArgs struct {
	BucketName string `abi:"bucketName"`
}

// HeadBucketExtraArgs is the decode target for the headBucketExtra calldata.
type HeadBucketExtraArgs struct {
	BucketName string `abi:"bucketName"`
}

// HeadBucketByIDArgs is the decode target for the headBucketById calldata.
type HeadBucketByIDArgs struct {
	BucketID string `abi:"bucketId"`
}

// HeadBucketNFTArgs is the decode target for the headBucketNFT calldata.
type HeadBucketNFTArgs struct {
	TokenID string `abi:"tokenId"`
}

// HeadObjectNFTArgs is the decode target for the headObjectNFT calldata.
type HeadObjectNFTArgs struct {
	TokenID string `abi:"tokenId"`
}

// HeadGroupNFTArgs is the decode target for the headGroupNFT calldata.
type HeadGroupNFTArgs struct {
	TokenID string `abi:"tokenId"`
}

// DeleteBucketArgs is the decode target for the deleteBucket calldata.
type DeleteBucketArgs struct {
	BucketName string `abi:"bucketName"`
}

// DiscontinueBucketArgs is the decode target for the discontinueBucket calldata.
type DiscontinueBucketArgs struct {
	BucketName string `abi:"bucketName"`
	Reason     string `abi:"reason"`
}

// MigrateBucketArgs is the decode target for the migrateBucket calldata.
type MigrateBucketArgs struct {
	BucketName           string   `abi:"bucketName"`
	DstPrimarySpID       uint32   `abi:"dstPrimarySpId"`
	DstPrimarySpApproval Approval `abi:"dstPrimarySpApproval"`
}

// CompleteMigrateBucketArgs is the decode target for the completeMigrateBucket calldata.
type CompleteMigrateBucketArgs struct {
	BucketName  string       `abi:"bucketName"`
	GvgFamilyID uint32       `abi:"gvgFamilyId"`
	GvgMappings []GVGMapping `abi:"gvgMappings"`
}

// RejectMigrateBucketArgs is the decode target for the rejectMigrateBucket calldata.
type RejectMigrateBucketArgs struct {
	BucketName string `abi:"bucketName"`
}

// CancelMigrateBucketArgs is the decode target for the cancelMigrateBucket calldata.
type CancelMigrateBucketArgs struct {
	BucketName string `abi:"bucketName"`
}

// SetBucketFlowRateLimitArgs is the decode target for the setBucketFlowRateLimit calldata.
type SetBucketFlowRateLimitArgs struct {
	BucketName     string   `abi:"bucketName"`
	BucketOwner    string   `abi:"bucketOwner"`
	PaymentAddress string   `abi:"paymentAddress"`
	FlowRateLimit  *big.Int `abi:"flowRateLimit"`
}

// CreateObjectArgs is the decode target for the createObject calldata.
type CreateObjectArgs struct {
	BucketName        string   `abi:"bucketName"`
	ObjectName        string   `abi:"objectName"`
	PayloadSize       uint64   `abi:"payloadSize"`
	Visibility        uint8    `abi:"visibility"`
	ContentType       string   `abi:"contentType"`
	PrimarySpApproval Approval `abi:"primarySpApproval"`
	ExpectChecksums   []string `abi:"expectChecksums"`
	RedundancyType    uint8    `abi:"redundancyType"`
}

// CopyObjectArgs is the decode target for the copyObject calldata.
type CopyObjectArgs struct {
	SrcBucketName        string   `abi:"srcBucketName"`
	DstBucketName        string   `abi:"dstBucketName"`
	SrcObjectName        string   `abi:"srcObjectName"`
	DstObjectName        string   `abi:"dstObjectName"`
	DstPrimarySpApproval Approval `abi:"dstPrimarySpApproval"`
}

// DeleteObjectArgs is the decode target for the deleteObject calldata.
type DeleteObjectArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
}

// CancelCreateObjectArgs is the decode target for the cancelCreateObject calldata.
type CancelCreateObjectArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
}

// CancelUpdateObjectContentArgs is the decode target for the cancelUpdateObjectContent calldata.
type CancelUpdateObjectContentArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
}

// ListObjectsArgs is the decode target for the listObjects calldata.
type ListObjectsArgs struct {
	Pagination PageRequest `abi:"pagination"`
	BucketName string      `abi:"bucketName"`
}

// ListObjectsByBucketIDArgs is the decode target for the listObjectsByBucketId calldata.
type ListObjectsByBucketIDArgs struct {
	Pagination PageRequest `abi:"pagination"`
	BucketID   string      `abi:"bucketId"`
}

// SealObjectArgs is the decode target for the sealObject calldata.
type SealObjectArgs struct {
	BucketName                  string `abi:"bucketName"`
	ObjectName                  string `abi:"objectName"`
	GlobalVirtualGroupID        uint32 `abi:"globalVirtualGroupId"`
	SecondarySpBlsAggSignatures string `abi:"secondarySpBlsAggSignatures"`
}

// SealObjectV2Args is the decode target for the sealObjectV2 calldata.
type SealObjectV2Args struct {
	BucketName                  string   `abi:"bucketName"`
	ObjectName                  string   `abi:"objectName"`
	GlobalVirtualGroupID        uint32   `abi:"globalVirtualGroupId"`
	SecondarySpBlsAggSignatures string   `abi:"secondarySpBlsAggSignatures"`
	ExpectChecksums             []string `abi:"expectChecksums"`
}

// RejectSealObjectArgs is the decode target for the rejectSealObject calldata.
type RejectSealObjectArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
}

// DelegateCreateObjectArgs is the decode target for the delegateCreateObject calldata.
type DelegateCreateObjectArgs struct {
	Creator         string   `abi:"creator"`
	BucketName      string   `abi:"bucketName"`
	ObjectName      string   `abi:"objectName"`
	PayloadSize     uint64   `abi:"payloadSize"`
	ContentType     string   `abi:"contentType"`
	Visibility      uint8    `abi:"visibility"`
	ExpectChecksums []string `abi:"expectChecksums"`
	RedundancyType  uint8    `abi:"redundancyType"`
}

// DelegateUpdateObjectContentArgs is the decode target for the delegateUpdateObjectContent calldata.
type DelegateUpdateObjectContentArgs struct {
	Updater         string   `abi:"updater"`
	BucketName      string   `abi:"bucketName"`
	ObjectName      string   `abi:"objectName"`
	PayloadSize     uint64   `abi:"payloadSize"`
	ContentType     string   `abi:"contentType"`
	ExpectChecksums []string `abi:"expectChecksums"`
}

// UpdateObjectInfoArgs is the decode target for the updateObjectInfo calldata.
type UpdateObjectInfoArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
	Visibility uint8  `abi:"visibility"`
}

// UpdateObjectContentArgs is the decode target for the updateObjectContent calldata.
type UpdateObjectContentArgs struct {
	BucketName      string   `abi:"bucketName"`
	ObjectName      string   `abi:"objectName"`
	PayloadSize     uint64   `abi:"payloadSize"`
	ContentType     string   `abi:"contentType"`
	ExpectChecksums []string `abi:"expectChecksums"`
}

// DiscontinueObjectArgs is the decode target for the discontinueObject calldata.
type DiscontinueObjectArgs struct {
	BucketName string     `abi:"bucketName"`
	ObjectIDs  []*big.Int `abi:"objectIds"`
	Reason     string     `abi:"reason"`
}

// CreateGroupArgs is the decode target for the createGroup calldata.
type CreateGroupArgs struct {
	GroupName string `abi:"groupName"`
	Extra     string `abi:"extra"`
}

// ListGroupsArgs is the decode target for the listGroups calldata.
type ListGroupsArgs struct {
	Pagination PageRequest    `abi:"pagination"`
	GroupOwner common.Address `abi:"groupOwner"`
}

// UpdateGroupArgs is the decode target for the updateGroup calldata.
type UpdateGroupArgs struct {
	GroupOwner      common.Address   `abi:"groupOwner"`
	GroupName       string           `abi:"groupName"`
	MembersToAdd    []common.Address `abi:"membersToAdd"`
	ExpirationTime  []int64          `abi:"expirationTime"`
	MembersToDelete []common.Address `abi:"membersToDelete"`
}

// UpdateGroupExtraArgs is the decode target for the updateGroupExtra calldata.
type UpdateGroupExtraArgs struct {
	GroupOwner common.Address `abi:"groupOwner"`
	GroupName  string         `abi:"groupName"`
	Extra      string         `abi:"extra"`
}

// HeadGroupArgs is the decode target for the headGroup calldata.
type HeadGroupArgs struct {
	GroupOwner common.Address `abi:"groupOwner"`
	GroupName  string         `abi:"groupName"`
}

// DeleteGroupArgs is the decode target for the deleteGroup calldata.
type DeleteGroupArgs struct {
	GroupName string `abi:"groupName"`
}

// LeaveGroupArgs is the decode target for the leaveGroup calldata.
type LeaveGroupArgs struct {
	GroupOwner common.Address `abi:"groupOwner"`
	GroupName  string         `abi:"groupName"`
}

// HeadGroupMemberArgs is the decode target for the headGroupMember calldata.
type HeadGroupMemberArgs struct {
	Member     common.Address `abi:"member"`
	GroupOwner common.Address `abi:"groupOwner"`
	GroupName  string         `abi:"groupName"`
}

// RenewGroupMemberArgs is the decode target for the renewGroupMember calldata.
type RenewGroupMemberArgs struct {
	GroupOwner     common.Address   `abi:"groupOwner"`
	GroupName      string           `abi:"groupName"`
	Members        []common.Address `abi:"members"`
	ExpirationTime []int64          `abi:"expirationTime"`
}

// ToggleSPAsDelegatedAgentArgs is the decode target for the toggleSPAsDelegatedAgent calldata.
type ToggleSPAsDelegatedAgentArgs struct {
	BucketName string `abi:"bucketName"`
}

// SetTagArgs is the decode target for the setTag calldata.
type SetTagArgs struct {
	Resource string `abi:"resource"`
	Tags     []Tag  `abi:"tags"`
}

// HeadObjectArgs is the decode target for the headObject calldata.
type HeadObjectArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
}

// HeadObjectByIDArgs is the decode target for the headObjectById calldata.
type HeadObjectByIDArgs struct {
	ObjectID string `abi:"objectId"`
}

// HeadShadowObjectArgs is the decode target for the headShadowObject calldata.
type HeadShadowObjectArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
}

// PutPolicyArgs is the decode target for the putPolicy calldata.
type PutPolicyArgs struct {
	Principal      Principal   `abi:"principal"`
	Resource       string      `abi:"resource"`
	Statements     []Statement `abi:"statements"`
	ExpirationTime int64       `abi:"expirationTime"`
}

// DeletePolicyArgs is the decode target for the deletePolicy calldata.
type DeletePolicyArgs struct {
	Principal Principal `abi:"principal"`
	Resource  string    `abi:"resource"`
}

// QueryPolicyForGroupArgs is the decode target for the queryPolicyForGroup calldata.
type QueryPolicyForGroupArgs struct {
	Resource string   `abi:"resource"`
	GroupID  *big.Int `abi:"groupId"`
}

// QueryPolicyForAccountArgs is the decode target for the queryPolicyForAccount calldata.
type QueryPolicyForAccountArgs struct {
	Resource      string `abi:"resource"`
	PrincipalAddr string `abi:"principalAddr"`
}

// QueryPolicyByIDArgs is the decode target for the queryPolicyById calldata.
type QueryPolicyByIDArgs struct {
	PolicyID string `abi:"policyId"`
}

// QueryLockFeeArgs is the decode target for the queryLockFee calldata.
type QueryLockFeeArgs struct {
	PrimarySpAddress string `abi:"primarySpAddress"`
	CreateAt         int64  `abi:"createAt"`
	PayloadSize      uint64 `abi:"payloadSize"`
}

// QueryIsPriceChangedArgs is the decode target for the queryIsPriceChanged calldata.
type QueryIsPriceChangedArgs struct {
	BucketName string `abi:"bucketName"`
}

// QueryQuotaUpdateTimeArgs is the decode target for the queryQuotaUpdateTime calldata.
type QueryQuotaUpdateTimeArgs struct {
	BucketName string `abi:"bucketName"`
}

// QueryGroupMembersExistArgs is the decode target for the queryGroupMembersExist calldata.
type QueryGroupMembersExistArgs struct {
	GroupID string   `abi:"groupId"`
	Members []string `abi:"members"`
}

// QueryGroupsExistArgs is the decode target for the queryGroupsExist calldata.
type QueryGroupsExistArgs struct {
	GroupOwner string   `abi:"groupOwner"`
	GroupNames []string `abi:"groupNames"`
}

// QueryGroupsExistByIDArgs is the decode target for the queryGroupsExistById calldata.
type QueryGroupsExistByIDArgs struct {
	GroupIDs []string `abi:"groupIds"`
}

// QueryPaymentAccountBucketFlowRateLimitArgs is the decode target for the
// queryPaymentAccountBucketFlowRateLimit calldata.
type QueryPaymentAccountBucketFlowRateLimitArgs struct {
	PaymentAccount string `abi:"paymentAccount"`
	BucketOwner    string `abi:"bucketOwner"`
	BucketName     string `abi:"bucketName"`
}

// QueryParamsByTimestampArgs is the decode target for the queryParamsByTimestamp calldata.
type QueryParamsByTimestampArgs struct {
	Timestamp int64 `abi:"timestamp"`
}

// VerifyPermissionArgs is the decode target for the verifyPermission calldata.
type VerifyPermissionArgs struct {
	BucketName string `abi:"bucketName"`
	ObjectName string `abi:"objectName"`
	ActionType int32  `abi:"actionType"`
}
