package storage

import (
	"bytes"
	"encoding/hex"
	"errors"

	cmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	permissiontypes "github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

const (
	// ListBucketsMethodName is the ABI name for the listBuckets query.
	ListBucketsMethodName = "listBuckets"
	// ListObjectsMethodName is the ABI name for the listObjects query.
	ListObjectsMethodName = "listObjects"
	// ListGroupsMethodName is the ABI name for the listGroups query.
	ListGroupsMethodName = "listGroups"
	// ListObjectsByBucketIDMethodName is the ABI name for the listObjectsByBucketId query.
	ListObjectsByBucketIDMethodName = "listObjectsByBucketId"
	// HeadBucketMethodName is the ABI name for the headBucket query.
	HeadBucketMethodName = "headBucket"
	// HeadGroupMethodName is the ABI name for the headGroup query.
	HeadGroupMethodName = "headGroup"
	// HeadGroupMemberMethodName is the ABI name for the headGroupMember query.
	HeadGroupMemberMethodName = "headGroupMember"
	// HeadObjectMethodName is the ABI name for the headObject query.
	HeadObjectMethodName = "headObject"
	// HeadObjectByIDMethodName is the ABI name for the headObjectById query.
	HeadObjectByIDMethodName = "headObjectById"
	// HeadBucketByIDMethodName is the ABI name for the headBucketById query.
	HeadBucketByIDMethodName = "headBucketById"
	// HeadBucketNFTMethodName is the ABI name for the headBucketNFT query.
	HeadBucketNFTMethodName = "headBucketNFT"
	// HeadShadowObjectMethodName is the ABI name for the headShadowObject query.
	HeadShadowObjectMethodName = "headShadowObject"
	// HeadObjectNFTMethodName is the ABI name for the headObjectNFT query.
	HeadObjectNFTMethodName = "headObjectNFT"
	// HeadGroupNFTMethodName is the ABI name for the headGroupNFT query.
	HeadGroupNFTMethodName = "headGroupNFT"
	// HeadBucketExtraMethodName is the ABI name for the headBucketExtra query.
	HeadBucketExtraMethodName = "headBucketExtra"
	// QueryPolicyForGroupMethodName is the ABI name for the queryPolicyForGroup query.
	QueryPolicyForGroupMethodName = "queryPolicyForGroup"
	// QueryPolicyForAccountMethodName is the ABI name for the queryPolicyForAccount query.
	QueryPolicyForAccountMethodName = "queryPolicyForAccount"
	// QueryParamsByTimestampMethodName is the ABI name for the queryParamsByTimestamp query.
	QueryParamsByTimestampMethodName = "queryParamsByTimestamp"
	// QueryPolicyByIDMethodName is the ABI name for the queryPolicyById query.
	QueryPolicyByIDMethodName = "queryPolicyById"
	// QueryLockFeeMethodName is the ABI name for the queryLockFee query.
	QueryLockFeeMethodName = "queryLockFee"
	// QueryIsPriceChangedMethodName is the ABI name for the queryIsPriceChanged query.
	QueryIsPriceChangedMethodName = "queryIsPriceChanged"
	// QueryQuotaUpdateTimeMethodName is the ABI name for the queryQuotaUpdateTime query.
	QueryQuotaUpdateTimeMethodName = "queryQuotaUpdateTime"
	// QueryGroupMembersExistMethodName is the ABI name for the queryGroupMembersExist query.
	QueryGroupMembersExistMethodName = "queryGroupMembersExist"
	// QueryGroupsExistMethodName is the ABI name for the queryGroupsExist query.
	QueryGroupsExistMethodName = "queryGroupsExist"
	// QueryGroupsExistByIDMethodName is the ABI name for the queryGroupsExistById query.
	QueryGroupsExistByIDMethodName = "queryGroupsExistById"
	// QueryPaymentAccountBucketFlowRateLimitMethodName is the ABI name for the
	// queryPaymentAccountBucketFlowRateLimit query.
	QueryPaymentAccountBucketFlowRateLimitMethodName = "queryPaymentAccountBucketFlowRateLimit"
	// ParamsMethodName is the ABI name for the params query.
	ParamsMethodName = "params"
	// VerifyPermissionMethodName is the ABI name for the verifyPermission query.
	VerifyPermissionMethodName = "verifyPermission"
)

// ListBuckets queries the total buckets.
func (p Precompile) ListBuckets(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ListBucketsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}
	msg := &storagetypes.QueryListBucketsRequest{
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	}
	res, err := p.storageKeeper.ListBuckets(ctx, msg)
	if err != nil {
		return nil, err
	}
	bucketInfos := make([]BucketInfo, 0, len(res.BucketInfos))
	for _, bucketInfo := range res.BucketInfos {
		bucketInfos = append(bucketInfos, BucketInfo{
			Owner:                      common.HexToAddress(bucketInfo.Owner),
			BucketName:                 bucketInfo.BucketName,
			Visibility:                 uint8(bucketInfo.Visibility),
			Id:                         bucketInfo.Id.BigInt(),
			SourceType:                 uint8(bucketInfo.SourceType),
			CreateAt:                   bucketInfo.CreateAt,
			PaymentAddress:             common.HexToAddress(bucketInfo.PaymentAddress),
			GlobalVirtualGroupFamilyId: bucketInfo.GlobalVirtualGroupFamilyId,
			ChargedReadQuota:           bucketInfo.ChargedReadQuota,
			BucketStatus:               uint8(bucketInfo.BucketStatus),
			Tags:                       outputTags(bucketInfo.Tags),
			SpAsDelegatedAgentDisabled: bucketInfo.SpAsDelegatedAgentDisabled,
		})
	}
	return method.Outputs.Pack(bucketInfos, outputPageResponse(res.Pagination))
}

// ListObjects queries the total objects.
func (p Precompile) ListObjects(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ListObjectsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}
	msg := &storagetypes.QueryListObjectsRequest{
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
		BucketName: input.BucketName,
	}
	res, err := p.storageKeeper.ListObjects(ctx, msg)
	if err != nil {
		return nil, err
	}
	objectInfos := make([]ObjectInfo, 0, len(res.ObjectInfos))
	for _, objectInfo := range res.ObjectInfos {
		objectInfos = append(objectInfos, *outputObjectInfo(objectInfo))
	}
	return method.Outputs.Pack(objectInfos, outputPageResponse(res.Pagination))
}

// ListGroups queries the user's total groups.
func (p Precompile) ListGroups(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ListGroupsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}
	msg := &storagetypes.QueryListGroupsRequest{
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
		GroupOwner: input.GroupOwner.String(),
	}
	res, err := p.storageKeeper.ListGroups(ctx, msg)
	if err != nil {
		return nil, err
	}
	groupInfos := make([]GroupInfo, 0, len(res.GroupInfos))
	for _, groupInfo := range res.GroupInfos {
		groupInfos = append(groupInfos, GroupInfo{
			Owner:      common.HexToAddress(groupInfo.Owner),
			GroupName:  groupInfo.GroupName,
			SourceType: uint8(groupInfo.SourceType),
			Id:         groupInfo.Id.BigInt(),
			Extra:      groupInfo.Extra,
			Tags:       outputTags(groupInfo.Tags),
		})
	}
	return method.Outputs.Pack(groupInfos, outputPageResponse(res.Pagination))
}

// ListObjectsByBucketID queries a list of object items under the bucket.
func (p Precompile) ListObjectsByBucketID(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ListObjectsByBucketIDArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}
	msg := &storagetypes.QueryListObjectsByBucketIdRequest{
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
		BucketId: input.BucketID,
	}
	res, err := p.storageKeeper.ListObjectsByBucketId(ctx, msg)
	if err != nil {
		return nil, err
	}
	objectInfos := make([]ObjectInfo, 0, len(res.ObjectInfos))
	for _, objectInfo := range res.ObjectInfos {
		objectInfos = append(objectInfos, *outputObjectInfo(objectInfo))
	}
	return method.Outputs.Pack(objectInfos, outputPageResponse(res.Pagination))
}

// HeadBucket queries the bucket's info.
func (p Precompile) HeadBucket(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadBucketArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryHeadBucketRequest{
		BucketName: input.BucketName,
	}
	res, err := p.storageKeeper.HeadBucket(ctx, msg)
	if err != nil {
		return nil, err
	}
	bucketInfo := BucketInfo{
		Owner:                      common.HexToAddress(res.BucketInfo.Owner),
		BucketName:                 res.BucketInfo.BucketName,
		Visibility:                 uint8(res.BucketInfo.Visibility),
		Id:                         res.BucketInfo.Id.BigInt(),
		SourceType:                 uint8(res.BucketInfo.SourceType),
		CreateAt:                   res.BucketInfo.CreateAt,
		PaymentAddress:             common.HexToAddress(res.BucketInfo.PaymentAddress),
		GlobalVirtualGroupFamilyId: res.BucketInfo.GlobalVirtualGroupFamilyId,
		ChargedReadQuota:           res.BucketInfo.ChargedReadQuota,
		BucketStatus:               uint8(res.BucketInfo.BucketStatus),
		Tags:                       outputTags(res.BucketInfo.Tags),
		SpAsDelegatedAgentDisabled: res.BucketInfo.SpAsDelegatedAgentDisabled,
	}
	extraInfo := BucketExtraInfo{
		IsRateLimited:   res.ExtraInfo.IsRateLimited,
		FlowRateLimit:   res.ExtraInfo.FlowRateLimit.BigInt(),
		CurrentFlowRate: res.ExtraInfo.CurrentFlowRate.BigInt(),
	}

	return method.Outputs.Pack(bucketInfo, extraInfo)
}

// HeadBucketByID queries the bucket's info by id.
func (p Precompile) HeadBucketByID(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadBucketByIDArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryHeadBucketByIdRequest{
		BucketId: input.BucketID,
	}
	res, err := p.storageKeeper.HeadBucketById(ctx, msg)
	if err != nil {
		return nil, err
	}
	bucketInfo := BucketInfo{
		Owner:                      common.HexToAddress(res.BucketInfo.Owner),
		BucketName:                 res.BucketInfo.BucketName,
		Visibility:                 uint8(res.BucketInfo.Visibility),
		Id:                         res.BucketInfo.Id.BigInt(),
		SourceType:                 uint8(res.BucketInfo.SourceType),
		CreateAt:                   res.BucketInfo.CreateAt,
		PaymentAddress:             common.HexToAddress(res.BucketInfo.PaymentAddress),
		GlobalVirtualGroupFamilyId: res.BucketInfo.GlobalVirtualGroupFamilyId,
		ChargedReadQuota:           res.BucketInfo.ChargedReadQuota,
		BucketStatus:               uint8(res.BucketInfo.BucketStatus),
		Tags:                       outputTags(res.BucketInfo.Tags),
		SpAsDelegatedAgentDisabled: res.BucketInfo.SpAsDelegatedAgentDisabled,
	}
	extraInfo := BucketExtraInfo{
		IsRateLimited:   res.ExtraInfo.IsRateLimited,
		FlowRateLimit:   res.ExtraInfo.FlowRateLimit.BigInt(),
		CurrentFlowRate: res.ExtraInfo.CurrentFlowRate.BigInt(),
	}

	return method.Outputs.Pack(bucketInfo, extraInfo)
}

// HeadBucketExtra queries a bucket extra info (with gvg bindings and price time) with specify name.
func (p Precompile) HeadBucketExtra(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadBucketExtraArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryHeadBucketExtraRequest{
		BucketName: input.BucketName,
	}
	res, err := p.storageKeeper.HeadBucketExtra(ctx, msg)
	if err != nil {
		return nil, err
	}
	localVirtualGroups := make([]LocalVirtualGroup, 0)
	for _, localVirtualGroup := range res.ExtraInfo.LocalVirtualGroups {
		localVirtualGroups = append(localVirtualGroups, LocalVirtualGroup{
			Id:                   localVirtualGroup.Id,
			GlobalVirtualGroupId: localVirtualGroup.GlobalVirtualGroupId,
			StoredSize:           localVirtualGroup.StoredSize,
			TotalChargeSize:      localVirtualGroup.TotalChargeSize,
		})
	}
	extraInfo := InternalBucketInfo{
		PriceTime:               res.ExtraInfo.PriceTime,
		TotalChargeSize:         res.ExtraInfo.TotalChargeSize,
		LocalVirtualGroups:      localVirtualGroups,
		NextLocalVirtualGroupId: res.ExtraInfo.NextLocalVirtualGroupId,
	}

	return method.Outputs.Pack(extraInfo)
}

// HeadBucketNFT queries a bucket with EIP712 standard metadata info.
func (p Precompile) HeadBucketNFT(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadBucketNFTArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryNFTRequest{
		TokenId: input.TokenID,
	}
	res, err := p.storageKeeper.HeadBucketNFT(ctx, msg)
	if err != nil {
		return nil, err
	}
	attributes := make([]Trait, 0)
	for _, attribute := range res.MetaData.Attributes {
		attributes = append(attributes, Trait{
			TraitType: attribute.TraitType,
			Value:     attribute.Value,
		})
	}
	bucketMetaData := BucketMetaData{
		Description: res.MetaData.Description,
		ExternalUrl: res.MetaData.ExternalUrl,
		BucketName:  res.MetaData.BucketName,
		Image:       res.MetaData.Image,
		Attributes:  attributes,
	}

	return method.Outputs.Pack(bucketMetaData)
}

// HeadObjectNFT queries an object with EIP712 standard metadata info.
func (p Precompile) HeadObjectNFT(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadObjectNFTArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryNFTRequest{
		TokenId: input.TokenID,
	}
	res, err := p.storageKeeper.HeadObjectNFT(ctx, msg)
	if err != nil {
		return nil, err
	}
	attributes := make([]Trait, 0)
	for _, attribute := range res.MetaData.Attributes {
		attributes = append(attributes, Trait{
			TraitType: attribute.TraitType,
			Value:     attribute.Value,
		})
	}
	objectMetaData := ObjectMetaData{
		Description: res.MetaData.Description,
		ExternalUrl: res.MetaData.ExternalUrl,
		ObjectName:  res.MetaData.ObjectName,
		Image:       res.MetaData.Image,
		Attributes:  attributes,
	}

	return method.Outputs.Pack(objectMetaData)
}

// HeadGroupNFT queries a group with EIP712 standard metadata info.
func (p Precompile) HeadGroupNFT(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadGroupNFTArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryNFTRequest{
		TokenId: input.TokenID,
	}
	res, err := p.storageKeeper.HeadGroupNFT(ctx, msg)
	if err != nil {
		return nil, err
	}
	attributes := make([]Trait, 0)
	for _, attribute := range res.MetaData.Attributes {
		attributes = append(attributes, Trait{
			TraitType: attribute.TraitType,
			Value:     attribute.Value,
		})
	}
	groupMetaData := GroupMetaData{
		Description: res.MetaData.Description,
		ExternalUrl: res.MetaData.ExternalUrl,
		GroupName:   res.MetaData.GroupName,
		Image:       res.MetaData.Image,
		Attributes:  attributes,
	}

	return method.Outputs.Pack(groupMetaData)
}

// HeadGroup queries the group's info.
func (p Precompile) HeadGroup(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryHeadGroupRequest{
		GroupOwner: input.GroupOwner.String(),
		GroupName:  input.GroupName,
	}
	res, err := p.storageKeeper.HeadGroup(ctx, msg)
	if err != nil {
		return nil, err
	}
	groupInfo := GroupInfo{
		Owner:      common.HexToAddress(res.GroupInfo.Owner),
		GroupName:  res.GroupInfo.GroupName,
		SourceType: uint8(res.GroupInfo.SourceType),
		Id:         res.GroupInfo.Id.BigInt(),
		Extra:      res.GroupInfo.Extra,
		Tags:       outputTags(res.GroupInfo.Tags),
	}
	return method.Outputs.Pack(groupInfo)
}

// HeadGroupMember queries the group member's info.
func (p Precompile) HeadGroupMember(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadGroupMemberArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryHeadGroupMemberRequest{
		Member:     input.Member.String(),
		GroupOwner: input.GroupOwner.String(),
		GroupName:  input.GroupName,
	}
	res, err := p.storageKeeper.HeadGroupMember(ctx, msg)
	if err != nil {
		return nil, err
	}
	var expirationTime int64
	if res.GroupMember.ExpirationTime != nil {
		expirationTime = res.GroupMember.ExpirationTime.Unix()
	} else {
		expirationTime = 0
	}
	groupMemberInfo := GroupMember{
		Id:             res.GroupMember.Id.BigInt(),
		GroupId:        res.GroupMember.GroupId.BigInt(),
		Member:         common.HexToAddress(res.GroupMember.Member),
		ExpirationTime: expirationTime,
	}
	return method.Outputs.Pack(groupMemberInfo)
}

// HeadObject queries the object's info.
func (p Precompile) HeadObject(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if input.BucketName == "" {
		return nil, errors.New("empty bucket name")
	}
	if input.ObjectName == "" {
		return nil, errors.New("empty object name")
	}
	msg := &storagetypes.QueryHeadObjectRequest{
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
	}
	res, err := p.storageKeeper.HeadObject(ctx, msg)
	if err != nil {
		return nil, err
	}
	objectInfo := outputObjectInfo(res.ObjectInfo)
	gvg := outputsGlobalVirtualGroup(res.GlobalVirtualGroup)
	return method.Outputs.Pack(objectInfo, gvg)
}

// HeadObjectByID queries the object's info.
func (p Precompile) HeadObjectByID(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadObjectByIDArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if input.ObjectID == "" {
		return nil, errors.New("empty object id")
	}
	msg := &storagetypes.QueryHeadObjectByIdRequest{
		ObjectId: input.ObjectID,
	}
	res, err := p.storageKeeper.HeadObjectById(ctx, msg)
	if err != nil {
		return nil, err
	}
	objectInfo := outputObjectInfo(res.ObjectInfo)
	gvg := outputsGlobalVirtualGroup(res.GlobalVirtualGroup)
	return method.Outputs.Pack(objectInfo, gvg)
}

// HeadShadowObject queries a shadow object with specify name.
func (p Precompile) HeadShadowObject(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input HeadShadowObjectArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	if input.BucketName == "" {
		return nil, errors.New("empty bucket name")
	}
	if input.ObjectName == "" {
		return nil, errors.New("empty object name")
	}
	msg := &storagetypes.QueryHeadShadowObjectRequest{
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
	}
	res, err := p.storageKeeper.HeadShadowObject(ctx, msg)
	if err != nil {
		return nil, err
	}
	checksums := []string{}
	for i := range res.ObjectInfo.Checksums {
		checksums = append(checksums, hex.EncodeToString(res.ObjectInfo.Checksums[i]))
	}
	objectInfo := ShadowObjectInfo{
		Operator:    res.ObjectInfo.Operator,
		Id:          res.ObjectInfo.Id.BigInt(),
		ContentType: res.ObjectInfo.ContentType,
		PayloadSize: res.ObjectInfo.PayloadSize,
		Checksums:   checksums,
		UpdatedAt:   res.ObjectInfo.UpdatedAt,
		Version:     res.ObjectInfo.Version,
	}
	return method.Outputs.Pack(objectInfo)
}

// QueryPolicyForGroup queries the group's policy.
func (p Precompile) QueryPolicyForGroup(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryPolicyForGroupArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryPolicyForGroupRequest{
		Resource:         input.Resource,
		PrincipalGroupId: cmath.NewUintFromBigInt(input.GroupID).String(),
	}
	res, err := p.storageKeeper.QueryPolicyForGroup(ctx, msg)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(outputPolicy(res.Policy))
}

// QueryPolicyForAccount queries the account's policy.
func (p Precompile) QueryPolicyForAccount(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryPolicyForAccountArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryPolicyForAccountRequest{
		Resource:         input.Resource,
		PrincipalAddress: input.PrincipalAddr,
	}
	res, err := p.storageKeeper.QueryPolicyForAccount(ctx, msg)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(outputPolicy(res.Policy))
}

// QueryPolicyByID queries a policy by policy id.
func (p Precompile) QueryPolicyByID(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryPolicyByIDArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryPolicyByIdRequest{
		PolicyId: input.PolicyID,
	}
	res, err := p.storageKeeper.QueryPolicyById(ctx, msg)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(outputPolicy(res.Policy))
}

// QueryLockFee queries lock fee for storing an object.
func (p Precompile) QueryLockFee(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryLockFeeArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryLockFeeRequest{
		PrimarySpAddress: input.PrimarySpAddress,
		CreateAt:         input.CreateAt,
		PayloadSize:      input.PayloadSize,
	}
	res, err := p.storageKeeper.QueryLockFee(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.Amount.BigInt())
}

// QueryIsPriceChanged queries whether read and storage prices changed for the bucket.
func (p Precompile) QueryIsPriceChanged(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryIsPriceChangedArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryIsPriceChangedRequest{
		BucketName: input.BucketName,
	}
	res, err := p.storageKeeper.QueryIsPriceChanged(ctx, msg)
	if err != nil {
		return nil, err
	}
	isPriceChanged := IsPriceChanged{
		Changed:                    res.Changed,
		CurrentReadPrice:           res.CurrentReadPrice.BigInt(),
		CurrentPrimaryStorePrice:   res.CurrentPrimaryStorePrice.BigInt(),
		CurrentSecondaryStorePrice: res.CurrentSecondaryStorePrice.BigInt(),
		CurrentValidatorTaxRate:    res.CurrentValidatorTaxRate.BigInt(),
		NewReadPrice:               res.NewReadPrice.BigInt(),
		NewPrimaryStorePrice:       res.NewPrimaryStorePrice.BigInt(),
		NewSecondaryStorePrice:     res.NewSecondaryStorePrice.BigInt(),
		NewValidatorTaxRate:        res.NewValidatorTaxRate.BigInt(),
	}

	return method.Outputs.Pack(isPriceChanged)
}

// QueryQuotaUpdateTime queries quota update time for the bucket.
func (p Precompile) QueryQuotaUpdateTime(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryQuotaUpdateTimeArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryQuoteUpdateTimeRequest{
		BucketName: input.BucketName,
	}
	res, err := p.storageKeeper.QueryQuotaUpdateTime(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.UpdateAt)
}

// QueryGroupMembersExist queries whether some members are in the group.
func (p Precompile) QueryGroupMembersExist(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryGroupMembersExistArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryGroupMembersExistRequest{
		GroupId: input.GroupID,
		Members: input.Members,
	}
	res, err := p.storageKeeper.QueryGroupMembersExist(ctx, msg)
	if err != nil {
		return nil, err
	}
	exists := make([]bool, 0)
	for _, member := range input.Members {
		exists = append(exists, res.Exists[member])
	}

	return method.Outputs.Pack(input.Members, exists)
}

// QueryGroupsExist queries whether some groups exist.
func (p Precompile) QueryGroupsExist(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryGroupsExistArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryGroupsExistRequest{
		GroupOwner: input.GroupOwner,
		GroupNames: input.GroupNames,
	}
	res, err := p.storageKeeper.QueryGroupsExist(ctx, msg)
	if err != nil {
		return nil, err
	}
	exists := make([]bool, 0)
	for _, groupName := range input.GroupNames {
		exists = append(exists, res.Exists[groupName])
	}

	return method.Outputs.Pack(input.GroupNames, exists)
}

// QueryGroupsExistByID queries whether some groups exist by id.
func (p Precompile) QueryGroupsExistByID(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryGroupsExistByIDArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryGroupsExistByIdRequest{
		GroupIds: input.GroupIDs,
	}
	res, err := p.storageKeeper.QueryGroupsExistById(ctx, msg)
	if err != nil {
		return nil, err
	}
	exists := make([]bool, 0)
	for _, groupID := range input.GroupIDs {
		exists = append(exists, res.Exists[groupID])
	}

	return method.Outputs.Pack(input.GroupIDs, exists)
}

// QueryPaymentAccountBucketFlowRateLimit queries the flow rate limit of a bucket for a payment account.
func (p Precompile) QueryPaymentAccountBucketFlowRateLimit(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryPaymentAccountBucketFlowRateLimitArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryPaymentAccountBucketFlowRateLimitRequest{
		PaymentAccount: input.PaymentAccount,
		BucketOwner:    input.BucketOwner,
		BucketName:     input.BucketName,
	}
	res, err := p.storageKeeper.QueryPaymentAccountBucketFlowRateLimit(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.IsSet, res.FlowRateLimit.BigInt())
}

// QueryParamsByTimestamp queries the parameters of the module by timestamp.
func (p Precompile) QueryParamsByTimestamp(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input QueryParamsByTimestampArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryParamsByTimestampRequest{
		Timestamp: input.Timestamp,
	}

	res, err := p.storageKeeper.QueryParamsByTimestamp(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputParams(res.Params))
}

// Params queries the storage parameters.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	msg := &storagetypes.QueryParamsRequest{}

	res, err := p.storageKeeper.Params(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputParams(res.Params))
}

// VerifyPermission queries a list of VerifyPermission items.
func (p Precompile) VerifyPermission(ctx sdk.Context, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input VerifyPermissionArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	msg := &storagetypes.QueryVerifyPermissionRequest{
		Operator:   contract.Caller().String(),
		BucketName: input.BucketName,
		ObjectName: input.ObjectName,
		ActionType: permissiontypes.ActionType(input.ActionType),
	}
	res, err := p.storageKeeper.VerifyPermission(ctx, msg)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(int32(res.Effect))
}

// outputObjectInfo maps a storage ObjectInfo into the ABI tuple.
func outputObjectInfo(o *storagetypes.ObjectInfo) *ObjectInfo {
	n := &ObjectInfo{
		Owner:               common.HexToAddress(o.Owner),
		Creator:             common.HexToAddress(o.Creator),
		BucketName:          o.BucketName,
		ObjectName:          o.ObjectName,
		Id:                  o.Id.BigInt(),
		LocalVirtualGroupId: o.LocalVirtualGroupId,
		PayloadSize:         o.PayloadSize,
		Visibility:          uint8(o.Visibility),
		ContentType:         o.ContentType,
		CreateAt:            o.CreateAt,
		ObjectStatus:        uint8(o.ObjectStatus),
		RedundancyType:      uint8(o.RedundancyType),
		SourceType:          uint8(o.SourceType),
		Checksums:           []string{},
		Tags:                outputTags(o.Tags),
		IsUpdating:          o.IsUpdating,
		UpdatedAt:           o.UpdatedAt,
		UpdatedBy:           common.HexToAddress(o.UpdatedBy),
		Version:             o.Version,
	}
	for i := range o.Checksums {
		n.Checksums = append(n.Checksums, hex.EncodeToString(o.Checksums[i]))
	}
	return n
}

// outputsGlobalVirtualGroup maps a virtualgroup GlobalVirtualGroup into the ABI tuple.
func outputsGlobalVirtualGroup(g *vgtypes.GlobalVirtualGroup) *GlobalVirtualGroup {
	return &GlobalVirtualGroup{
		Id:                    g.Id,
		FamilyId:              g.FamilyId,
		PrimarySpId:           g.PrimarySpId,
		SecondarySpIds:        g.SecondarySpIds,
		StoredSize:            g.StoredSize,
		VirtualPaymentAddress: common.HexToAddress(g.VirtualPaymentAddress),
		TotalDeposit:          g.TotalDeposit.String(),
	}
}

// outputTags maps storage ResourceTags into the ABI tag tuple slice.
func outputTags(tags *storagetypes.ResourceTags) []Tag {
	t := make([]Tag, 0)
	if tags == nil {
		return t
	}
	for _, tag := range tags.Tags {
		t = append(t, Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	return t
}

// outputPageResponse maps a query.PageResponse into the ABI tuple.
func outputPageResponse(p *query.PageResponse) *PageResponse {
	return &PageResponse{
		NextKey: p.NextKey,
		Total:   p.Total,
	}
}

// outputPolicy maps a permission Policy into the ABI Policy tuple, preserving the
// original statement/expiration flattening.
func outputPolicy(policy *permissiontypes.Policy) Policy {
	var expirationTime int64
	statements := make([]Statement, 0, len(policy.Statements))
	for _, statement := range policy.Statements {
		actions := make([]int32, 0, len(statement.Actions))
		for _, action := range statement.Actions {
			actions = append(actions, int32(action))
		}
		if statement.ExpirationTime != nil {
			expirationTime = statement.ExpirationTime.Unix()
		} else {
			expirationTime = 0
		}
		statements = append(statements, Statement{
			Effect:         int32(statement.Effect),
			Actions:        actions,
			Resources:      statement.Resources,
			ExpirationTime: expirationTime,
			LimitSize:      statement.LimitSize.Value,
		})
	}
	if policy.ExpirationTime != nil {
		expirationTime = policy.ExpirationTime.Unix()
	} else {
		expirationTime = 0
	}
	return Policy{
		Id:             policy.Id.BigInt(),
		Principal:      Principal{PrincipalType: int32(policy.Principal.Type), Value: policy.Principal.Value},
		ResourceType:   int32(policy.ResourceType),
		ResourceId:     policy.ResourceId.BigInt(),
		Statements:     statements,
		ExpirationTime: expirationTime,
	}
}

// outputParams maps storage module Params into the ABI Params tuple.
func outputParams(params storagetypes.Params) Params {
	return Params{
		VersionedParams: VersionedParams{
			MaxSegmentSize:          params.VersionedParams.MaxSegmentSize,
			RedundantDataChunkNum:   params.VersionedParams.RedundantDataChunkNum,
			RedundantParityChunkNum: params.VersionedParams.RedundantParityChunkNum,
			MinChargeSize:           params.VersionedParams.MinChargeSize,
		},
		MaxPayloadSize:                   params.MaxPayloadSize,
		BscMirrorBucketRelayerFee:        params.BscMirrorBucketRelayerFee,
		BscMirrorBucketAckRelayerFee:     params.BscMirrorBucketAckRelayerFee,
		BscMirrorObjectRelayerFee:        params.BscMirrorObjectRelayerFee,
		BscMirrorObjectAckRelayerFee:     params.BscMirrorObjectAckRelayerFee,
		BscMirrorGroupRelayerFee:         params.BscMirrorGroupRelayerFee,
		BscMirrorGroupAckRelayerFee:      params.BscMirrorGroupAckRelayerFee,
		MaxBucketsPerAccount:             params.MaxBucketsPerAccount,
		DiscontinueCountingWindow:        params.DiscontinueCountingWindow,
		DiscontinueObjectMax:             params.DiscontinueObjectMax,
		DiscontinueBucketMax:             params.DiscontinueBucketMax,
		DiscontinueConfirmPeriod:         params.DiscontinueConfirmPeriod,
		DiscontinueDeletionMax:           params.DiscontinueDeletionMax,
		StalePolicyCleanupMax:            params.StalePolicyCleanupMax,
		MinQuotaUpdateInterval:           params.MinQuotaUpdateInterval,
		MaxLocalVirtualGroupNumPerBucket: params.MaxLocalVirtualGroupNumPerBucket,
		OpMirrorBucketRelayerFee:         params.OpMirrorBucketRelayerFee,
		OpMirrorBucketAckRelayerFee:      params.OpMirrorBucketAckRelayerFee,
		OpMirrorObjectRelayerFee:         params.OpMirrorObjectRelayerFee,
		OpMirrorObjectAckRelayerFee:      params.OpMirrorObjectAckRelayerFee,
		OpMirrorGroupRelayerFee:          params.OpMirrorGroupRelayerFee,
		OpMirrorGroupAckRelayerFee:       params.OpMirrorGroupAckRelayerFee,
		PolygonMirrorBucketRelayerFee:    params.PolygonMirrorBucketRelayerFee,
		PolygonMirrorBucketAckRelayerFee: params.PolygonMirrorBucketAckRelayerFee,
	}
}
