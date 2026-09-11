package keeper_test

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func makeKeeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)

	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		&types.MockAccountKeeper{},
		&types.MockSpKeeper{},
		&types.MockPaymentKeeper{},
		&types.MockPermissionKeeper{},
		&types.MockVirtualGroupKeeper{},
		&types.MockEVMKeeper{},
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	return k, testCtx.Ctx
}

func (s *TestSuite) TestQueryParams() {
	res, err := s.queryClient.Params(context.Background(), &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().Equal(s.storageKeeper.GetParams(s.ctx), res.GetParams())
}

func (s *TestSuite) TestQueryVersionedParams() {
	params := types.DefaultParams()
	params.VersionedParams.MaxSegmentSize = 1
	blockTimeT1 := s.ctx.BlockTime().Unix()
	paramsT1 := params
	err := s.storageKeeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(1 * time.Hour))
	blockTimeT2 := s.ctx.BlockTime().Unix()
	params.VersionedParams.MaxSegmentSize = 2
	paramsT2 := params
	err = s.storageKeeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	responseT1, err := s.storageKeeper.QueryParamsByTimestamp(s.ctx, &types.QueryParamsByTimestampRequest{Timestamp: blockTimeT1 + 1})
	s.Require().NoError(err)
	s.Require().Equal(&types.QueryParamsByTimestampResponse{Params: paramsT1}, responseT1)
	getParams := responseT1.GetParams()
	s.Require().Equal(getParams.GetMaxSegmentSize(), uint64(1))

	responseT2, err := s.storageKeeper.QueryParamsByTimestamp(s.ctx, &types.QueryParamsByTimestampRequest{Timestamp: blockTimeT2 + 1})
	s.Require().NoError(err)
	s.Require().Equal(&types.QueryParamsByTimestampResponse{Params: paramsT2}, responseT2)
	p := responseT2.GetParams()
	s.Require().Equal(p.GetMaxSegmentSize(), uint64(2))

	responseT3, err := s.storageKeeper.QueryParamsByTimestamp(s.ctx, &types.QueryParamsByTimestampRequest{Timestamp: 0})
	s.Require().NoError(err)
	s.Require().Equal(&types.QueryParamsByTimestampResponse{Params: paramsT2}, responseT3)
	p = responseT2.GetParams()
	s.Require().Equal(p.GetMaxSegmentSize(), uint64(2))
}

func (s *TestSuite) TestQueryGroupMembersExist() {
	groupId := rand.Intn(1000) //nolint
	members := make([]string, 3)
	exists := make(map[string]bool)
	for i := 0; i < 3; i++ {
		members[i] = sample.RandAccAddressHex()
		exist := rand.Intn(2) //nolint
		if exist == 0 {
			exists[members[i]] = false
			s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).Times(1)
		} else {
			exists[members[i]] = true
			s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, true).Times(1)
		}
	}

	req := &types.QueryGroupMembersExistRequest{
		GroupId: strconv.Itoa(groupId),
		Members: members,
	}
	res, err := s.queryClient.QueryGroupMembersExist(context.Background(), req)
	s.Require().NoError(err)
	s.Require().Equal(exists, res.GetExists())
}

func (s *TestSuite) TestQueryGroupsExist() {
	groupOwner := sample.RandAccAddress()
	groupNames := make([]string, 3)
	exists := make(map[string]bool)
	for i := 0; i < 3; i++ {
		groupNames[i] = string(sample.RandStr(10))
		exist := rand.Intn(2) //nolint
		if exist == 0 {
			exists[groupNames[i]] = false
		} else {
			exists[groupNames[i]] = true
			_, err := s.storageKeeper.CreateGroup(s.ctx, groupOwner, groupNames[i], types.CreateGroupOptions{})
			s.Require().NoError(err)
		}
	}

	req := &types.QueryGroupsExistRequest{
		GroupOwner: groupOwner.String(),
		GroupNames: groupNames,
	}
	res, err := s.queryClient.QueryGroupsExist(context.Background(), req)
	s.Require().NoError(err)
	s.Require().Equal(exists, res.GetExists())
}

func (s *TestSuite) TestQueryGroupsExistByID() {
	groupIDs := make([]string, 3)
	exists := make(map[string]bool)
	for i := 0; i < 3; i++ {
		//  make sure there's no conflict
		groupIDs[i] = strconv.Itoa(rand.Intn(1000) + 10) //nolint
		exist := rand.Intn(2)                            //nolint
		if exist == 0 {
			exists[groupIDs[i]] = false
		} else {
			id, err := s.storageKeeper.CreateGroup(s.ctx, sample.RandAccAddress(), string(sample.RandStr(10)), types.CreateGroupOptions{})
			s.Require().NoError(err)
			groupIDs[i] = id.String()
			exists[groupIDs[i]] = true
		}
	}

	req := &types.QueryGroupsExistByIdRequest{
		GroupIds: groupIDs,
	}
	res, err := s.queryClient.QueryGroupsExistById(context.Background(), req)
	s.Require().NoError(err)
	s.Require().Equal(exists, res.GetExists())
}

func TestHeadBucket(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadBucket(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadBucket(ctx, &types.QueryHeadBucketRequest{
		BucketName: "bucket",
	})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func TestHeadGroupNFT(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadGroupNFT(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadGroupNFT(ctx, &types.QueryNFTRequest{
		TokenId: "xxx",
	})
	require.ErrorContains(t, err, "invalid token id")

	// group not exist
	_, err = k.HeadGroupNFT(ctx, &types.QueryNFTRequest{
		TokenId: "0",
	})
	require.ErrorIs(t, err, types.ErrNoSuchGroup)
}

func TestHeadObjectNFT(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadObjectNFT(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadObjectNFT(ctx, &types.QueryNFTRequest{
		TokenId: "xxx",
	})
	require.ErrorContains(t, err, "invalid token id")

	// object not exist
	_, err = k.HeadObjectNFT(ctx, &types.QueryNFTRequest{
		TokenId: "0",
	})
	require.ErrorIs(t, err, types.ErrNoSuchObject)
}

func TestHeadBucketNFT(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadBucketNFT(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadBucketNFT(ctx, &types.QueryNFTRequest{
		TokenId: "xxx",
	})
	require.ErrorContains(t, err, "invalid token id")

	// bucket not exist
	_, err = k.HeadBucketNFT(ctx, &types.QueryNFTRequest{
		TokenId: "0",
	})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func TestHeadBucketById(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadBucketById(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadBucketById(ctx, &types.QueryHeadBucketByIdRequest{
		BucketId: "xxx",
	})
	require.ErrorContains(t, err, "invalid bucket id")

	// bucket not exist
	_, err = k.HeadBucketById(ctx, &types.QueryHeadBucketByIdRequest{
		BucketId: "0",
	})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func TestHeadObject(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadObject(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	// object not exist
	_, err = k.HeadObject(ctx, &types.QueryHeadObjectRequest{
		BucketName: "bucket",
		ObjectName: "object",
	})
	require.ErrorIs(t, err, types.ErrNoSuchObject)
}

func TestHeadObjectById(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadBucketById(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadObjectById(ctx, &types.QueryHeadObjectByIdRequest{
		ObjectId: "xxx",
	})
	require.ErrorContains(t, err, "invalid object id")

	// bucket not exist
	_, err = k.HeadObjectById(ctx, &types.QueryHeadObjectByIdRequest{
		ObjectId: "1",
	})
	require.ErrorIs(t, err, types.ErrNoSuchObject)
}

func TestListBuckets(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.ListBuckets(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.ListBuckets(ctx, &types.QueryListBucketsRequest{
		Pagination: &query.PageRequest{
			Limit: types.MaxPaginationLimit + 1,
		},
	})
	require.ErrorContains(t, err, "exceed pagination limit")
}

func TestListObjects(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.ListObjects(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.ListObjects(ctx, &types.QueryListObjectsRequest{
		Pagination: &query.PageRequest{
			Limit: types.MaxPaginationLimit,
		},
	})
	require.ErrorContains(t, err, "bucket name should not be empty")

	_, err = k.ListObjects(ctx, &types.QueryListObjectsRequest{
		BucketName: "abc",
		Pagination: &query.PageRequest{
			Limit: types.MaxPaginationLimit + 1,
		},
	})
	require.ErrorContains(t, err, "exceed pagination limit")
}

func TestListObjectsByBucketId(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.ListObjectsByBucketId(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.ListObjectsByBucketId(ctx, &types.QueryListObjectsByBucketIdRequest{
		Pagination: &query.PageRequest{
			Limit: types.MaxPaginationLimit + 1,
		},
	})
	require.ErrorContains(t, err, "exceed pagination limit")

	_, err = k.ListObjectsByBucketId(ctx, &types.QueryListObjectsByBucketIdRequest{
		BucketId: "xxx",
	})
	require.ErrorContains(t, err, "invalid bucket id")

	_, err = k.ListObjectsByBucketId(ctx, &types.QueryListObjectsByBucketIdRequest{
		BucketId: "0",
	})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func TestQueryPolicyForAccount(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.QueryPolicyForAccount(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.QueryPolicyForAccount(ctx, &types.QueryPolicyForAccountRequest{
		PrincipalAddress: "xxxx",
	})
	require.ErrorContains(t, err, "invalid address hex length")

	_, err = k.QueryPolicyForAccount(ctx, &types.QueryPolicyForAccountRequest{
		PrincipalAddress: sample.RandAccAddressHex(),
		Resource:         "xxx",
	})
	require.ErrorContains(t, err, "regex match error")
}

func TestQueryPolicyForGroup(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.QueryPolicyForGroup(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.QueryPolicyForGroup(ctx, &types.QueryPolicyForGroupRequest{
		PrincipalGroupId: "xxx",
	})
	require.ErrorContains(t, err, "invalid group id")

	_, err = k.QueryPolicyForGroup(ctx, &types.QueryPolicyForGroupRequest{
		PrincipalGroupId: "10",
		Resource:         "xxx",
	})
	require.ErrorContains(t, err, "regex match error")
}

func TestVerifyPermission(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.VerifyPermission(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.VerifyPermission(ctx, &types.QueryVerifyPermissionRequest{
		Operator: "xxx",
	})
	require.ErrorContains(t, err, "invalid operator address")

	_, err = k.VerifyPermission(ctx, &types.QueryVerifyPermissionRequest{
		Operator:   sample.RandAccAddressHex(),
		BucketName: "",
	})
	require.ErrorContains(t, err, "No bucket specified")

	_, err = k.VerifyPermission(ctx, &types.QueryVerifyPermissionRequest{
		Operator:   sample.RandAccAddressHex(),
		BucketName: "bucket",
	})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func TestHeadGroup(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.HeadGroup(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadGroup(ctx, &types.QueryHeadGroupRequest{
		GroupOwner: "xxx",
	})
	require.ErrorContains(t, err, "invalid address hex length")

	_, err = k.HeadGroup(ctx, &types.QueryHeadGroupRequest{
		GroupOwner: sample.RandAccAddressHex(),
		GroupName:  "group",
	})
	require.ErrorIs(t, err, types.ErrNoSuchGroup)
}

func TestListGroup(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.ListGroups(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.ListGroups(ctx, &types.QueryListGroupsRequest{
		Pagination: &query.PageRequest{
			Limit: types.MaxPaginationLimit + 1,
		},
	})
	require.ErrorContains(t, err, "exceed pagination limit")

	_, err = k.ListGroups(ctx, &types.QueryListGroupsRequest{
		GroupOwner: "xxx",
	})
	require.ErrorContains(t, err, "invalid address hex length")
}

func TestHeadGroupMember(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.ListGroups(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadGroupMember(ctx, &types.QueryHeadGroupMemberRequest{
		Member: "xxx",
	})
	require.ErrorContains(t, err, "invalid address hex length")

	_, err = k.HeadGroupMember(ctx, &types.QueryHeadGroupMemberRequest{
		Member:     sample.RandAccAddressHex(),
		GroupOwner: "xxx",
	})
	require.ErrorContains(t, err, "invalid address hex length")

	_, err = k.HeadGroupMember(ctx, &types.QueryHeadGroupMemberRequest{
		Member:     sample.RandAccAddressHex(),
		GroupOwner: sample.RandAccAddressHex(),
		GroupName:  "group",
	})
	require.ErrorIs(t, err, types.ErrNoSuchGroup)
}

func TestQueryPolicyById(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.QueryPolicyById(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.QueryPolicyById(ctx, &types.QueryPolicyByIdRequest{
		PolicyId: "xxx",
	})
	require.ErrorContains(t, err, "invalid policy id")
}

func TestQueryQuotaUpdateTime(t *testing.T) {
	// invalid argument
	k, ctx := makeKeeper(t)
	_, err := k.QueryQuotaUpdateTime(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	// bucket not exist
	_, err = k.QueryQuotaUpdateTime(ctx, &types.QueryQuoteUpdateTimeRequest{
		BucketName: "xxx",
	})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestQueryQuotaUpdateTime_Found() {
	bucketInfo := &types.BucketInfo{
		Owner:      sample.RandAccAddressHex(),
		BucketName: "grpc-quota-bucket",
		Id:         sdkmath.NewUint(61),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	res, err := s.storageKeeper.QueryQuotaUpdateTime(s.ctx, &types.QueryQuoteUpdateTimeRequest{BucketName: bucketInfo.BucketName})
	s.Require().NoError(err)
	s.Require().Equal(int64(0), res.UpdateAt)
}

// ---- HeadBucket / HeadBucketById ----

func (s *TestSuite) TestHeadBucket_Found() {
	bucketInfo := &types.BucketInfo{
		Owner:            sample.RandAccAddressHex(),
		BucketName:       "grpc-head-bucket",
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   sample.RandAccAddressHex(),
		ChargedReadQuota: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	// Empty internal bucket info + zero read quota short-circuits GetBucketReadStoreBill
	// so GetBucketExtraInfo needs no virtualGroupKeeper/spKeeper/paymentKeeper mocks.
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	res, err := s.storageKeeper.HeadBucket(s.ctx, &types.QueryHeadBucketRequest{BucketName: bucketInfo.BucketName})
	s.Require().NoError(err)
	s.Require().Equal(bucketInfo.BucketName, res.BucketInfo.BucketName)
	s.Require().NotNil(res.ExtraInfo)
	s.Require().True(res.ExtraInfo.FlowRateLimit.Equal(sdkmath.NewInt(-1)), "no rate limit set must report -1")
}

func (s *TestSuite) TestHeadBucketById_Found() {
	bucketInfo := &types.BucketInfo{
		Owner:      sample.RandAccAddressHex(),
		BucketName: "grpc-head-bucket-by-id",
		Id:         sdkmath.NewUint(42),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	res, err := s.storageKeeper.HeadBucketById(s.ctx, &types.QueryHeadBucketByIdRequest{BucketId: "42"})
	s.Require().NoError(err)
	s.Require().Equal(bucketInfo.BucketName, res.BucketInfo.BucketName)
}

// ---- HeadObject / HeadObjectById ----

func (s *TestSuite) TestHeadObject_Unsealed() {
	bucketName := "grpc-head-object-bucket"
	objectName := "grpc-head-object"
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: sdkmath.NewUint(1)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	objectInfo := &types.ObjectInfo{
		Owner:        bucketInfo.Owner,
		BucketName:   bucketName,
		ObjectName:   objectName,
		Id:           sdkmath.NewUint(1),
		ObjectStatus: types.OBJECT_STATUS_CREATED,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	res, err := s.storageKeeper.HeadObject(s.ctx, &types.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	s.Require().NoError(err)
	s.Require().Equal(objectName, res.ObjectInfo.ObjectName)
	s.Require().Nil(res.GlobalVirtualGroup, "an unsealed object must not resolve a GVG")
}

func (s *TestSuite) TestHeadObject_SealedResolvesGVG() {
	bucketName := "grpc-head-sealed-bucket"
	objectName := "grpc-head-sealed-object"
	bucketID := sdkmath.NewUint(2)
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: bucketID}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	objectInfo := &types.ObjectInfo{
		Owner:               bucketInfo.Owner,
		BucketName:          bucketName,
		ObjectName:          objectName,
		Id:                  sdkmath.NewUint(2),
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		LocalVirtualGroupId: 1,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 7}},
	})
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 7}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(7)).Return(gvg, true).AnyTimes()

	res, err := s.storageKeeper.HeadObject(s.ctx, &types.QueryHeadObjectRequest{BucketName: bucketName, ObjectName: objectName})
	s.Require().NoError(err)
	s.Require().Equal(gvg, res.GlobalVirtualGroup)
}

func (s *TestSuite) TestHeadObjectById_Found() {
	bucketName := "grpc-head-object-by-id-bucket"
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: sdkmath.NewUint(3)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	objectInfo := &types.ObjectInfo{
		Owner:        bucketInfo.Owner,
		BucketName:   bucketName,
		ObjectName:   "grpc-object-by-id",
		Id:           sdkmath.NewUint(3),
		ObjectStatus: types.OBJECT_STATUS_CREATED,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	res, err := s.storageKeeper.HeadObjectById(s.ctx, &types.QueryHeadObjectByIdRequest{ObjectId: "3"})
	s.Require().NoError(err)
	s.Require().Equal(objectInfo.ObjectName, res.ObjectInfo.ObjectName)
	s.Require().Nil(res.GlobalVirtualGroup)
}

func (s *TestSuite) TestHeadObjectById_SealedResolvesGVG() {
	bucketName := "grpc-head-sealed-by-id-bucket"
	bucketID := sdkmath.NewUint(4)
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: bucketID}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	objectInfo := &types.ObjectInfo{
		Owner:               bucketInfo.Owner,
		BucketName:          bucketName,
		ObjectName:          "grpc-sealed-object-by-id",
		Id:                  sdkmath.NewUint(4),
		ObjectStatus:        types.OBJECT_STATUS_SEALED,
		LocalVirtualGroupId: 1,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketID, &types.InternalBucketInfo{
		LocalVirtualGroups: []*types.LocalVirtualGroup{{Id: 1, GlobalVirtualGroupId: 8}},
	})
	gvg := &virtualgroupmoduletypes.GlobalVirtualGroup{Id: 8}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), uint32(8)).Return(gvg, true).AnyTimes()

	res, err := s.storageKeeper.HeadObjectById(s.ctx, &types.QueryHeadObjectByIdRequest{ObjectId: "4"})
	s.Require().NoError(err)
	s.Require().Equal(gvg, res.GlobalVirtualGroup)
}

// ---- HeadShadowObject ----

func TestHeadShadowObject(t *testing.T) {
	k, ctx := makeKeeper(t)
	_, err := k.HeadShadowObject(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadShadowObject(ctx, &types.QueryHeadShadowObjectRequest{BucketName: "b", ObjectName: "o"})
	require.ErrorIs(t, err, types.ErrNoSuchObject)
}

func (s *TestSuite) TestHeadShadowObject_Found() {
	bucketName := "grpc-shadow-bucket"
	objectName := "grpc-shadow-object"
	shadow := &types.ShadowObjectInfo{
		Operator:    sample.RandAccAddressHex(),
		Id:          sdkmath.NewUint(9),
		PayloadSize: 2048,
	}
	// No exported setter exists for shadow objects (only produced internally during
	// UpdateObjectContent); write the same key production code reads, mirroring
	// setupMigratingBucket's direct-store-write style in keeper_test.go.
	s.ctx.KVStore(s.storeKey).Set(types.GetShadowObjectKey(bucketName, objectName), s.cdc.MustMarshal(shadow))

	res, err := s.storageKeeper.HeadShadowObject(s.ctx, &types.QueryHeadShadowObjectRequest{
		BucketName: bucketName,
		ObjectName: objectName,
	})
	s.Require().NoError(err)
	s.Require().Equal(shadow.PayloadSize, res.ObjectInfo.PayloadSize)
	s.Require().Equal(shadow.Operator, res.ObjectInfo.Operator)
}

// ---- ListBuckets / ListObjects / ListObjectsByBucketId ----

func (s *TestSuite) TestListBuckets_ReturnsStored() {
	b1 := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: "list-bucket-one", Id: sdkmath.NewUint(1)}
	b2 := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: "list-bucket-two", Id: sdkmath.NewUint(2)}
	s.storageKeeper.StoreBucketInfo(s.ctx, b1)
	s.storageKeeper.StoreBucketInfo(s.ctx, b2)

	res, err := s.storageKeeper.ListBuckets(s.ctx, &types.QueryListBucketsRequest{})
	s.Require().NoError(err)
	names := make([]string, 0, len(res.BucketInfos))
	for _, b := range res.BucketInfos {
		names = append(names, b.BucketName)
	}
	s.Require().ElementsMatch([]string{b1.BucketName, b2.BucketName}, names)
}

func (s *TestSuite) TestListObjects_ReturnsStored() {
	bucketName := "list-objects-bucket"
	o1 := &types.ObjectInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, ObjectName: "list-obj-one", Id: sdkmath.NewUint(1)}
	o2 := &types.ObjectInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, ObjectName: "list-obj-two", Id: sdkmath.NewUint(2)}
	s.storageKeeper.StoreObjectInfo(s.ctx, o1)
	s.storageKeeper.StoreObjectInfo(s.ctx, o2)

	res, err := s.storageKeeper.ListObjects(s.ctx, &types.QueryListObjectsRequest{BucketName: bucketName})
	s.Require().NoError(err)
	names := make([]string, 0, len(res.ObjectInfos))
	for _, o := range res.ObjectInfos {
		names = append(names, o.ObjectName)
	}
	s.Require().ElementsMatch([]string{o1.ObjectName, o2.ObjectName}, names)
}

func (s *TestSuite) TestListObjectsByBucketId_ReturnsStored() {
	bucketName := "list-objects-by-id-bucket"
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: sdkmath.NewUint(5)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	o1 := &types.ObjectInfo{Owner: bucketInfo.Owner, BucketName: bucketName, ObjectName: "list-by-id-obj-a", Id: sdkmath.NewUint(1)}
	o2 := &types.ObjectInfo{Owner: bucketInfo.Owner, BucketName: bucketName, ObjectName: "list-by-id-obj-b", Id: sdkmath.NewUint(2)}
	s.storageKeeper.StoreObjectInfo(s.ctx, o1)
	s.storageKeeper.StoreObjectInfo(s.ctx, o2)

	res, err := s.storageKeeper.ListObjectsByBucketId(s.ctx, &types.QueryListObjectsByBucketIdRequest{BucketId: "5"})
	s.Require().NoError(err)
	names := make([]string, 0, len(res.ObjectInfos))
	for _, o := range res.ObjectInfos {
		names = append(names, o.ObjectName)
	}
	s.Require().ElementsMatch([]string{o1.ObjectName, o2.ObjectName}, names)
}

// ---- HeadBucketNFT / HeadObjectNFT / HeadGroupNFT ----

func (s *TestSuite) TestHeadBucketNFT_Found() {
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: "nft-bucket", Id: sdkmath.NewUint(11)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	res, err := s.storageKeeper.HeadBucketNFT(s.ctx, &types.QueryNFTRequest{TokenId: "11"})
	s.Require().NoError(err)
	s.Require().Equal(bucketInfo.ToNFTMetadata(), res.MetaData)
}

func (s *TestSuite) TestHeadObjectNFT_Found() {
	objectInfo := &types.ObjectInfo{Owner: sample.RandAccAddressHex(), BucketName: "nft-obj-bucket", ObjectName: "nft-obj", Id: sdkmath.NewUint(12)}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	res, err := s.storageKeeper.HeadObjectNFT(s.ctx, &types.QueryNFTRequest{TokenId: "12"})
	s.Require().NoError(err)
	s.Require().Equal(objectInfo.ToNFTMetadata(), res.MetaData)
}

func (s *TestSuite) TestHeadGroupNFT_Found() {
	owner := sample.RandAccAddress()
	groupID, err := s.storageKeeper.CreateGroup(s.ctx, owner, "nft-group", types.CreateGroupOptions{})
	s.Require().NoError(err)

	res, err := s.storageKeeper.HeadGroupNFT(s.ctx, &types.QueryNFTRequest{TokenId: groupID.String()})
	s.Require().NoError(err)
	s.Require().NotNil(res.MetaData)
	s.Require().Equal("nft-group", res.MetaData.GroupName)
}

// ---- QueryLockFee ----

func TestQueryLockFee(t *testing.T) {
	k, ctx := makeKeeper(t)
	_, err := k.QueryLockFee(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.QueryLockFee(ctx, &types.QueryLockFeeRequest{PrimarySpAddress: "xxx"})
	require.ErrorContains(t, err, "invalid primary storage provider address")
}

func (s *TestSuite) TestQueryLockFee_SpNotFound() {
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	_, err := s.storageKeeper.QueryLockFee(s.ctx, &types.QueryLockFeeRequest{
		PrimarySpAddress: sample.RandAccAddressHex(),
		PayloadSize:      1024,
	})
	s.Require().ErrorIs(err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestQueryLockFee_Success() {
	primaryAddr := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: primaryAddr.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(10),
		SecondaryStorePrice: sdkmath.LegacyNewDec(5),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	params := paymenttypes.DefaultParams()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(params.VersionedParams, nil).AnyTimes()

	// GetObjectChargeSize/GetExpectSecondarySPNumForECObject resolve the storage
	// module's OWN versioned params via a reverse-time iterator that excludes an
	// exact match (see params.go GetVersionedParamsWithTS's doc comment), so the
	// query time must be strictly after the block time SetupTest seeded at.
	res, err := s.storageKeeper.QueryLockFee(s.ctx, &types.QueryLockFeeRequest{
		PrimarySpAddress: primaryAddr.String(),
		CreateAt:         s.ctx.BlockTime().Unix() + 1,
		PayloadSize:      1024,
	})
	s.Require().NoError(err)
	s.Require().True(res.Amount.IsPositive())
}

// ---- HeadBucketExtra ----

func TestHeadBucketExtra(t *testing.T) {
	k, ctx := makeKeeper(t)
	_, err := k.HeadBucketExtra(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.HeadBucketExtra(ctx, &types.QueryHeadBucketExtraRequest{BucketName: "nope"})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestHeadBucketExtra_Found() {
	bucketInfo := &types.BucketInfo{
		Owner:      sample.RandAccAddressHex(),
		BucketName: "grpc-extra-bucket",
		Id:         sdkmath.NewUint(21),
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	internalBucketInfo := &types.InternalBucketInfo{TotalChargeSize: 12345}
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, internalBucketInfo)

	res, err := s.storageKeeper.HeadBucketExtra(s.ctx, &types.QueryHeadBucketExtraRequest{BucketName: bucketInfo.BucketName})
	s.Require().NoError(err)
	s.Require().Equal(internalBucketInfo.TotalChargeSize, res.ExtraInfo.TotalChargeSize)
}

// ---- QueryIsPriceChanged ----

func TestQueryIsPriceChanged(t *testing.T) {
	k, ctx := makeKeeper(t)
	_, err := k.QueryIsPriceChanged(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.QueryIsPriceChanged(ctx, &types.QueryIsPriceChangedRequest{BucketName: "nope"})
	require.ErrorIs(t, err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestQueryIsPriceChanged_Unchanged() {
	primarySpId := uint32(1)
	bucketInfo := &types.BucketInfo{
		Owner:                      sample.RandAccAddressHex(),
		BucketName:                 "grpc-price-bucket",
		Id:                         sdkmath.NewUint(31),
		GlobalVirtualGroupFamilyId: 1,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{PriceTime: s.ctx.BlockTime().Unix()})

	family := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: primarySpId}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), uint32(1)).Return(family, true).AnyTimes()
	sp := &sptypes.StorageProvider{Id: primarySpId}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), primarySpId).Return(sp, true).AnyTimes()

	// Both the "pre" and "current" lookups resolve through these gomock.Any()
	// stubs regardless of the timestamp argument, so the same constant price/tax
	// rate is returned for both and IsPriceChanged must report false.
	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(2),
		SecondaryStorePrice: sdkmath.LegacyNewDec(3),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	payVer := paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(payVer, nil).AnyTimes()

	res, err := s.storageKeeper.QueryIsPriceChanged(s.ctx, &types.QueryIsPriceChangedRequest{BucketName: bucketInfo.BucketName})
	s.Require().NoError(err)
	s.Require().False(res.Changed)
	s.Require().True(res.CurrentReadPrice.Equal(price.ReadPrice))
}

// ---- QueryPaymentAccountBucketFlowRateLimit ----

func TestQueryPaymentAccountBucketFlowRateLimit(t *testing.T) {
	k, ctx := makeKeeper(t)
	_, err := k.QueryPaymentAccountBucketFlowRateLimit(ctx, nil)
	require.ErrorContains(t, err, "invalid request")

	_, err = k.QueryPaymentAccountBucketFlowRateLimit(ctx, &types.QueryPaymentAccountBucketFlowRateLimitRequest{
		PaymentAccount: "xxx",
	})
	require.ErrorContains(t, err, "invalid payment account address")

	_, err = k.QueryPaymentAccountBucketFlowRateLimit(ctx, &types.QueryPaymentAccountBucketFlowRateLimitRequest{
		PaymentAccount: sample.RandAccAddressHex(),
		BucketOwner:    "xxx",
	})
	require.ErrorContains(t, err, "invalid bucket owner address")

	// valid addresses, nothing stored -> IsSet false, no injected keeper touched
	res, err := k.QueryPaymentAccountBucketFlowRateLimit(ctx, &types.QueryPaymentAccountBucketFlowRateLimitRequest{
		PaymentAccount: sample.RandAccAddressHex(),
		BucketOwner:    sample.RandAccAddressHex(),
		BucketName:     "no-limit-set",
	})
	require.NoError(t, err)
	require.False(t, res.IsSet)
}

func (s *TestSuite) TestQueryPaymentAccountBucketFlowRateLimit_IsSet() {
	operator := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	bucketName := "flow-rate-limit-bucket"
	rateLimit := sdkmath.NewInt(500)

	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	// No bucket stored under this name, so SetBucketFlowRateLimit takes its
	// "bucket doesn't use this payment account" branch and stores the limit
	// keyed directly by the given (paymentAccount, bucketOwner, bucketName).
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operator, bucketOwner, paymentAccount, bucketName, rateLimit)
	s.Require().NoError(err)

	res, err := s.storageKeeper.QueryPaymentAccountBucketFlowRateLimit(s.ctx, &types.QueryPaymentAccountBucketFlowRateLimitRequest{
		PaymentAccount: paymentAccount.String(),
		BucketOwner:    bucketOwner.String(),
		BucketName:     bucketName,
	})
	s.Require().NoError(err)
	s.Require().True(res.IsSet)
	s.Require().True(res.FlowRateLimit.Equal(rateLimit))
}

// ---- QueryPolicyForAccount / QueryPolicyForGroup / QueryPolicyById ----

func (s *TestSuite) TestQueryPolicyForAccount_Found() {
	bucketName := "grpc-policy-account-bucket"
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: sdkmath.NewUint(41)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	principal := sample.RandAccAddress()
	policy := &permtypes.Policy{Id: sdkmath.NewUint(1), Principal: permtypes.NewPrincipalWithAccount(principal)}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(policy, true).AnyTimes()

	res, err := s.storageKeeper.QueryPolicyForAccount(s.ctx, &types.QueryPolicyForAccountRequest{
		PrincipalAddress: principal.String(),
		Resource:         types2.NewBucketGRN(bucketName).String(),
	})
	s.Require().NoError(err)
	s.Require().Equal(policy, res.Policy)
}

func (s *TestSuite) TestQueryPolicyForGroup_Found() {
	bucketName := "grpc-policy-group-bucket"
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: sdkmath.NewUint(42)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	policy := &permtypes.Policy{Id: sdkmath.NewUint(2), Principal: permtypes.NewPrincipalWithGroupID(sdkmath.NewUint(7))}
	s.permissionKeeper.EXPECT().GetPolicyForGroup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(policy, true).AnyTimes()

	res, err := s.storageKeeper.QueryPolicyForGroup(s.ctx, &types.QueryPolicyForGroupRequest{
		PrincipalGroupId: "7",
		Resource:         types2.NewBucketGRN(bucketName).String(),
	})
	s.Require().NoError(err)
	s.Require().Equal(policy, res.Policy)
}

func (s *TestSuite) TestQueryPolicyById_Found() {
	policy := &permtypes.Policy{Id: sdkmath.NewUint(3)}
	s.permissionKeeper.EXPECT().GetPolicyByID(gomock.Any(), gomock.Any()).Return(policy, true).AnyTimes()

	res, err := s.storageKeeper.QueryPolicyById(s.ctx, &types.QueryPolicyByIdRequest{PolicyId: "3"})
	s.Require().NoError(err)
	s.Require().Equal(policy, res.Policy)
}

func (s *TestSuite) TestQueryPolicyById_NotFound() {
	s.permissionKeeper.EXPECT().GetPolicyByID(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	_, err := s.storageKeeper.QueryPolicyById(s.ctx, &types.QueryPolicyByIdRequest{PolicyId: "999"})
	s.Require().ErrorIs(err, types.ErrNoSuchPolicy)
}

// ---- VerifyPermission ----

func (s *TestSuite) TestVerifyPermission_BucketOwnerAllow() {
	owner := sample.RandAccAddress()
	bucketInfo := &types.BucketInfo{Owner: owner.String(), BucketName: "grpc-verify-bucket", Id: sdkmath.NewUint(51)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	res, err := s.storageKeeper.VerifyPermission(s.ctx, &types.QueryVerifyPermissionRequest{
		Operator:   owner.String(),
		BucketName: bucketInfo.BucketName,
		ActionType: permtypes.ACTION_GET_OBJECT,
	})
	s.Require().NoError(err)
	s.Require().Equal(permtypes.EFFECT_ALLOW, res.Effect)
}

func (s *TestSuite) TestVerifyPermission_ObjectOwnerAllow() {
	owner := sample.RandAccAddress()
	bucketName := "grpc-verify-object-bucket"
	objectName := "grpc-verify-object"
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: sdkmath.NewUint(52)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	objectInfo := &types.ObjectInfo{Owner: owner.String(), BucketName: bucketName, ObjectName: objectName, Id: sdkmath.NewUint(1)}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	res, err := s.storageKeeper.VerifyPermission(s.ctx, &types.QueryVerifyPermissionRequest{
		Operator:   owner.String(),
		BucketName: bucketName,
		ObjectName: objectName,
		ActionType: permtypes.ACTION_GET_OBJECT,
	})
	s.Require().NoError(err)
	s.Require().Equal(permtypes.EFFECT_ALLOW, res.Effect)
}

func (s *TestSuite) TestVerifyPermission_ObjectNotFound() {
	bucketName := "grpc-verify-missing-object-bucket"
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddressHex(), BucketName: bucketName, Id: sdkmath.NewUint(53)}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	_, err := s.storageKeeper.VerifyPermission(s.ctx, &types.QueryVerifyPermissionRequest{
		Operator:   sample.RandAccAddressHex(),
		BucketName: bucketName,
		ObjectName: "does-not-exist",
		ActionType: permtypes.ACTION_GET_OBJECT,
	})
	s.Require().ErrorIs(err, types.ErrNoSuchObject)
}

// ---- HeadGroup / ListGroups / HeadGroupMember ----

func (s *TestSuite) TestHeadGroup_Found() {
	owner := sample.RandAccAddress()
	groupID, err := s.storageKeeper.CreateGroup(s.ctx, owner, "grpc-head-group", types.CreateGroupOptions{})
	s.Require().NoError(err)

	res, err := s.storageKeeper.HeadGroup(s.ctx, &types.QueryHeadGroupRequest{
		GroupOwner: owner.String(),
		GroupName:  "grpc-head-group",
	})
	s.Require().NoError(err)
	s.Require().Equal(groupID, res.GroupInfo.Id)
}

func (s *TestSuite) TestListGroups_ReturnsStored() {
	owner := sample.RandAccAddress()
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, "grpc-list-group-one", types.CreateGroupOptions{})
	s.Require().NoError(err)
	_, err = s.storageKeeper.CreateGroup(s.ctx, owner, "grpc-list-group-two", types.CreateGroupOptions{})
	s.Require().NoError(err)

	res, err := s.storageKeeper.ListGroups(s.ctx, &types.QueryListGroupsRequest{GroupOwner: owner.String()})
	s.Require().NoError(err)
	names := make([]string, 0, len(res.GroupInfos))
	for _, g := range res.GroupInfos {
		names = append(names, g.GroupName)
	}
	s.Require().ElementsMatch([]string{"grpc-list-group-one", "grpc-list-group-two"}, names)
}

func (s *TestSuite) TestHeadGroupMember_Found() {
	owner := sample.RandAccAddress()
	member := sample.RandAccAddress()
	groupID, err := s.storageKeeper.CreateGroup(s.ctx, owner, "grpc-group-member", types.CreateGroupOptions{})
	s.Require().NoError(err)

	groupMember := &permtypes.GroupMember{Id: sdkmath.NewUint(1), GroupId: groupID, Member: member.String()}
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupID, gomock.Any()).Return(groupMember, true).AnyTimes()

	res, err := s.storageKeeper.HeadGroupMember(s.ctx, &types.QueryHeadGroupMemberRequest{
		Member:     member.String(),
		GroupOwner: owner.String(),
		GroupName:  "grpc-group-member",
	})
	s.Require().NoError(err)
	s.Require().Equal(groupMember, res.GroupMember)
}

func (s *TestSuite) TestHeadGroupMember_NotFound() {
	owner := sample.RandAccAddress()
	member := sample.RandAccAddress()
	_, err := s.storageKeeper.CreateGroup(s.ctx, owner, "grpc-group-member-missing", types.CreateGroupOptions{})
	s.Require().NoError(err)

	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	_, err = s.storageKeeper.HeadGroupMember(s.ctx, &types.QueryHeadGroupMemberRequest{
		Member:     member.String(),
		GroupOwner: owner.String(),
		GroupName:  "grpc-group-member-missing",
	})
	s.Require().ErrorIs(err, types.ErrNoSuchGroupMember)
}
