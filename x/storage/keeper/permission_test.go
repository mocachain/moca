package keeper_test

import (
	"errors"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/types/common"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
	gnfdresource "github.com/mocachain/moca/v2/types/resource"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	"github.com/mocachain/moca/v2/x/storage/types"
	"go.uber.org/mock/gomock"
)

func (s *TestSuite) TestVerifyBucketPermission_PublicReadAllowed() {
	bucketInfo := &types.BucketInfo{Visibility: types.VISIBILITY_TYPE_PUBLIC_READ, Owner: sample.RandAccAddress().String()}

	effect := s.storageKeeper.VerifyBucketPermission(s.ctx, bucketInfo, sample.RandAccAddress(), permtypes.ACTION_GET_OBJECT, nil)
	s.Require().Equal(permtypes.EFFECT_ALLOW, effect)
}

func (s *TestSuite) TestVerifyBucketPermission_EmptyOperatorDenied() {
	bucketInfo := &types.BucketInfo{Owner: sample.RandAccAddress().String()}

	effect := s.storageKeeper.VerifyBucketPermission(s.ctx, bucketInfo, sdk.AccAddress{}, permtypes.ACTION_GET_OBJECT, nil)
	s.Require().Equal(permtypes.EFFECT_DENY, effect)
}

func (s *TestSuite) TestVerifyObjectPermission_PublicReadAllowed() {
	owner := sample.RandAccAddress().String()
	objectInfo := &types.ObjectInfo{Visibility: types.VISIBILITY_TYPE_PUBLIC_READ, Owner: owner}
	bucketInfo := &types.BucketInfo{Owner: owner}

	effect := s.storageKeeper.VerifyObjectPermission(s.ctx, bucketInfo, objectInfo, sample.RandAccAddress(), permtypes.ACTION_GET_OBJECT)
	s.Require().Equal(permtypes.EFFECT_ALLOW, effect)
}

func (s *TestSuite) TestVerifyObjectPermission_EmptyOperatorDenied() {
	owner := sample.RandAccAddress().String()
	objectInfo := &types.ObjectInfo{Owner: owner}
	bucketInfo := &types.BucketInfo{Owner: owner}

	effect := s.storageKeeper.VerifyObjectPermission(s.ctx, bucketInfo, objectInfo, sdk.AccAddress{}, permtypes.ACTION_GET_OBJECT)
	s.Require().Equal(permtypes.EFFECT_DENY, effect)
}

// TestVerifyObjectPermission_BucketDenyShortCircuits: a bucket-level Deny
// rejects the request without ever consulting the object-level policy.
func (s *TestSuite) TestVerifyObjectPermission_BucketDenyShortCircuits() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress()
	bucketInfo := &types.BucketInfo{Id: sdkmath.NewUint(301), Owner: owner.String(), BucketName: "vop-bucket-deny"}
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(302), Owner: owner.String(), BucketName: "vop-bucket-deny", ObjectName: "obj"}

	denyPolicy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect: permtypes.EFFECT_DENY, Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
	}}}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), bucketInfo.Id, gnfdresource.RESOURCE_TYPE_BUCKET, operator).Return(denyPolicy, true)

	effect := s.storageKeeper.VerifyObjectPermission(s.ctx, bucketInfo, objectInfo, operator, permtypes.ACTION_GET_OBJECT)
	s.Require().Equal(permtypes.EFFECT_DENY, effect)
}

// TestVerifyObjectPermission_ObjectDenyShortCircuits: an unspecified
// bucket-level effect falls through to the object-level policy, whose Deny wins.
func (s *TestSuite) TestVerifyObjectPermission_ObjectDenyShortCircuits() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress()
	bucketInfo := &types.BucketInfo{Id: sdkmath.NewUint(303), Owner: owner.String(), BucketName: "vop-object-deny"}
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(304), Owner: owner.String(), BucketName: "vop-object-deny", ObjectName: "obj"}

	denyPolicy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect: permtypes.EFFECT_DENY, Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
	}}}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), bucketInfo.Id, gnfdresource.RESOURCE_TYPE_BUCKET, operator).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), bucketInfo.Id, gnfdresource.RESOURCE_TYPE_BUCKET).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), objectInfo.Id, gnfdresource.RESOURCE_TYPE_OBJECT, operator).Return(denyPolicy, true)

	effect := s.storageKeeper.VerifyObjectPermission(s.ctx, bucketInfo, objectInfo, operator, permtypes.ACTION_GET_OBJECT)
	s.Require().Equal(permtypes.EFFECT_DENY, effect)
}

// TestVerifyObjectPermission_AllowedViaBucketPolicy: an Allow at the bucket
// level is sufficient even when the object level has no opinion.
func (s *TestSuite) TestVerifyObjectPermission_AllowedViaBucketPolicy() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress()
	bucketInfo := &types.BucketInfo{Id: sdkmath.NewUint(305), Owner: owner.String(), BucketName: "vop-bucket-allow"}
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(306), Owner: owner.String(), BucketName: "vop-bucket-allow", ObjectName: "obj"}

	// The bucket-level statement must name the sub-resource explicitly: a
	// statement with no Resources is bucket-scoped only and would evaluate to
	// EFFECT_UNSPECIFIED against VerifyObjectPermission's per-object opts.
	allowPolicy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect: permtypes.EFFECT_ALLOW, Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT}, Resources: []string{"*"},
	}}}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), bucketInfo.Id, gnfdresource.RESOURCE_TYPE_BUCKET, operator).Return(allowPolicy, true)
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), objectInfo.Id, gnfdresource.RESOURCE_TYPE_OBJECT, operator).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), objectInfo.Id, gnfdresource.RESOURCE_TYPE_OBJECT).Return(nil, false)

	effect := s.storageKeeper.VerifyObjectPermission(s.ctx, bucketInfo, objectInfo, operator, permtypes.ACTION_GET_OBJECT)
	s.Require().Equal(permtypes.EFFECT_ALLOW, effect)
}

// TestVerifyObjectPermission_NoPolicyDenied: neither level has an opinion, so
// the request falls to the default-deny outcome.
func (s *TestSuite) TestVerifyObjectPermission_NoPolicyDenied() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress()
	bucketInfo := &types.BucketInfo{Id: sdkmath.NewUint(307), Owner: owner.String(), BucketName: "vop-nopolicy"}
	objectInfo := &types.ObjectInfo{Id: sdkmath.NewUint(308), Owner: owner.String(), BucketName: "vop-nopolicy", ObjectName: "obj"}

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	effect := s.storageKeeper.VerifyObjectPermission(s.ctx, bucketInfo, objectInfo, operator, permtypes.ACTION_GET_OBJECT)
	s.Require().Equal(permtypes.EFFECT_DENY, effect)
}

func (s *TestSuite) TestVerifyGroupPermission_OwnerAllowed() {
	owner := sample.RandAccAddress()
	groupInfo := &types.GroupInfo{Id: sdkmath.NewUint(1), Owner: owner.String(), GroupName: "vgp-owner"}

	effect := s.storageKeeper.VerifyGroupPermission(s.ctx, groupInfo, owner, permtypes.ACTION_DELETE_GROUP)
	s.Require().Equal(permtypes.EFFECT_ALLOW, effect)
}

func (s *TestSuite) TestVerifyGroupPermission_PolicyAllows() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress()
	groupInfo := &types.GroupInfo{Id: sdkmath.NewUint(2), Owner: owner.String(), GroupName: "vgp-policy"}

	policy := &permtypes.Policy{
		Principal: permtypes.NewPrincipalWithAccount(operator),
		Statements: []*permtypes.Statement{{
			Effect:  permtypes.EFFECT_ALLOW,
			Actions: []permtypes.ActionType{permtypes.ACTION_UPDATE_GROUP_MEMBER},
		}},
	}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), groupInfo.Id, gnfdresource.RESOURCE_TYPE_GROUP, operator).Return(policy, true)

	effect := s.storageKeeper.VerifyGroupPermission(s.ctx, groupInfo, operator, permtypes.ACTION_UPDATE_GROUP_MEMBER)
	s.Require().Equal(permtypes.EFFECT_ALLOW, effect)
}

func (s *TestSuite) TestVerifyGroupPermission_NoPolicyDenied() {
	owner := sample.RandAccAddress()
	operator := sample.RandAccAddress()
	groupInfo := &types.GroupInfo{Id: sdkmath.NewUint(3), Owner: owner.String(), GroupName: "vgp-denied"}

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)

	effect := s.storageKeeper.VerifyGroupPermission(s.ctx, groupInfo, operator, permtypes.ACTION_UPDATE_GROUP_MEMBER)
	s.Require().Equal(permtypes.EFFECT_DENY, effect)
}

// TestVerifyPolicy_GroupGrant_SkipsUnknownGroup covers the loop's hasGroup
// guard: a policy-group item naming a group that no longer exists in the
// storage module's own group store must be skipped, not queried for a policy.
func (s *TestSuite) TestVerifyPolicy_GroupGrant_SkipsUnknownGroup() {
	resourceID := sdkmath.NewUint(100)
	operator := sample.RandAccAddress()
	unknownGroupID := sdkmath.NewUint(555) // never stored via SetGroupInfo

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), resourceID, gnfdresource.RESOURCE_TYPE_BUCKET).
		Return(&permtypes.PolicyGroup{Items: []*permtypes.PolicyGroup_Item{{GroupId: unknownGroupID, PolicyId: sdkmath.NewUint(1)}}}, true)

	effect := s.storageKeeper.VerifyPolicy(s.ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator, permtypes.ACTION_GET_OBJECT, nil)
	s.Require().Equal(permtypes.EFFECT_UNSPECIFIED, effect)
}

func (s *TestSuite) TestVerifyPolicy_GroupGrant_DenyShortCircuits() {
	resourceID := sdkmath.NewUint(101)
	operator := sample.RandAccAddress()
	groupID := sdkmath.NewUint(11)
	policyID := sdkmath.NewUint(21)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Id: groupID, GroupName: "deny-group"})

	denyPolicy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect:  permtypes.EFFECT_DENY,
		Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
	}}}

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), resourceID, gnfdresource.RESOURCE_TYPE_BUCKET).
		Return(&permtypes.PolicyGroup{Items: []*permtypes.PolicyGroup_Item{{GroupId: groupID, PolicyId: policyID}}}, true)
	s.permissionKeeper.EXPECT().MustGetPolicyByID(gomock.Any(), policyID).Return(denyPolicy)
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupID, operator).Return(&permtypes.GroupMember{}, true)

	effect := s.storageKeeper.VerifyPolicy(s.ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator, permtypes.ACTION_GET_OBJECT, nil)
	s.Require().Equal(permtypes.EFFECT_DENY, effect)
}

func (s *TestSuite) TestVerifyPolicy_GroupGrant_ExpiredMembershipSkipped() {
	resourceID := sdkmath.NewUint(102)
	operator := sample.RandAccAddress()
	groupID := sdkmath.NewUint(12)
	policyID := sdkmath.NewUint(22)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Id: groupID, GroupName: "expired-group"})

	allowPolicy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect:  permtypes.EFFECT_ALLOW,
		Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
	}}}
	expired := s.ctx.BlockTime().Add(-time.Hour)

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), resourceID, gnfdresource.RESOURCE_TYPE_BUCKET).
		Return(&permtypes.PolicyGroup{Items: []*permtypes.PolicyGroup_Item{{GroupId: groupID, PolicyId: policyID}}}, true)
	s.permissionKeeper.EXPECT().MustGetPolicyByID(gomock.Any(), policyID).Return(allowPolicy)
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupID, operator).Return(&permtypes.GroupMember{ExpirationTime: &expired}, true)

	effect := s.storageKeeper.VerifyPolicy(s.ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator, permtypes.ACTION_GET_OBJECT, nil)
	s.Require().Equal(permtypes.EFFECT_UNSPECIFIED, effect, "an expired group membership must not grant the policy's effect")
}

func (s *TestSuite) TestVerifyPolicy_GroupGrant_NonMemberSkipped() {
	resourceID := sdkmath.NewUint(103)
	operator := sample.RandAccAddress()
	groupID := sdkmath.NewUint(13)
	policyID := sdkmath.NewUint(23)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Id: groupID, GroupName: "nonmember-group"})

	allowPolicy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect:  permtypes.EFFECT_ALLOW,
		Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
	}}}

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), resourceID, gnfdresource.RESOURCE_TYPE_BUCKET).
		Return(&permtypes.PolicyGroup{Items: []*permtypes.PolicyGroup_Item{{GroupId: groupID, PolicyId: policyID}}}, true)
	s.permissionKeeper.EXPECT().MustGetPolicyByID(gomock.Any(), policyID).Return(allowPolicy)
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupID, operator).Return(nil, false)

	effect := s.storageKeeper.VerifyPolicy(s.ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator, permtypes.ACTION_GET_OBJECT, nil)
	s.Require().Equal(permtypes.EFFECT_UNSPECIFIED, effect, "a non-member must not benefit from their group's grant")
}

func (s *TestSuite) TestVerifyPolicy_GroupGrant_Allowed() {
	resourceID := sdkmath.NewUint(104)
	operator := sample.RandAccAddress()
	groupID := sdkmath.NewUint(14)
	policyID := sdkmath.NewUint(24)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Id: groupID, GroupName: "allow-group"})

	allowPolicy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect:  permtypes.EFFECT_ALLOW,
		Actions: []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
	}}}

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), resourceID, gnfdresource.RESOURCE_TYPE_BUCKET).
		Return(&permtypes.PolicyGroup{Items: []*permtypes.PolicyGroup_Item{{GroupId: groupID, PolicyId: policyID}}}, true)
	s.permissionKeeper.EXPECT().MustGetPolicyByID(gomock.Any(), policyID).Return(allowPolicy)
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupID, operator).Return(&permtypes.GroupMember{}, true)

	effect := s.storageKeeper.VerifyPolicy(s.ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator, permtypes.ACTION_GET_OBJECT, nil)
	s.Require().Equal(permtypes.EFFECT_ALLOW, effect)
}

// TestVerifyPolicy_GroupGrant_CreateObjectQuotaConsumed drives the group-grant
// CreateObject quota self-update through a real x/permission keeper (mirroring
// TestPutPolicy_OverCapQuotaGrantStillConsumable's realPermissionKeeper
// pattern), so the LimitSize decrement and its PutPolicy persistence are
// exercised with production policy-eval semantics instead of a hand-rolled stub.
func (s *TestSuite) TestVerifyPolicy_GroupGrant_CreateObjectQuotaConsumed() {
	permKeeper := s.realPermissionKeeper()
	groupID := sdkmath.NewUint(900)
	resourceID := sdkmath.NewUint(901)
	operator := sample.RandAccAddress()

	// the storage module's own group must exist for hasGroup to let the item through
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Id: groupID, GroupName: "quota-group"})

	_, err := permKeeper.PutPolicy(s.ctx, &permtypes.Policy{
		Principal:    permtypes.NewPrincipalWithGroupID(groupID),
		ResourceType: gnfdresource.RESOURCE_TYPE_BUCKET,
		ResourceId:   resourceID,
		Statements: []*permtypes.Statement{{
			Effect:    permtypes.EFFECT_ALLOW,
			Actions:   []permtypes.ActionType{permtypes.ACTION_CREATE_OBJECT},
			LimitSize: &common.UInt64Value{Value: 1024 * 1024},
		}},
	})
	s.Require().NoError(err)
	s.Require().NoError(permKeeper.AddGroupMember(s.ctx, groupID, operator, nil))

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.GetPolicyGroupForResource).AnyTimes()
	s.permissionKeeper.EXPECT().MustGetPolicyByID(gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.MustGetPolicyByID).AnyTimes()
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.GetGroupMember).AnyTimes()
	s.permissionKeeper.EXPECT().PutPolicy(gomock.Any(), gomock.Any()).
		DoAndReturn(permKeeper.PutPolicy).AnyTimes()

	wanted := uint64(1000)
	ctx := s.ctx.WithTxBytes([]byte{0x01}) // VerifyPolicy only self-updates inside a tx
	var effect permtypes.Effect
	s.Require().NotPanics(func() {
		effect = s.storageKeeper.VerifyPolicy(ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator,
			permtypes.ACTION_CREATE_OBJECT, &permtypes.VerifyOptions{WantedSize: &wanted})
	})
	s.Require().Equal(permtypes.EFFECT_ALLOW, effect)

	policy, found := permKeeper.GetPolicyForGroup(s.ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, groupID)
	s.Require().True(found)
	s.Require().Equal(uint64(1024*1024-1000), policy.Statements[0].LimitSize.GetValue(),
		"the group grant's quota must be decremented and persisted")
}

// TestVerifyPolicy_AccountGrant_ConsumeErrorPanics pins that VerifyPolicy does
// not swallow a failure from the account-grant self-update PutPolicy call.
func (s *TestSuite) TestVerifyPolicy_AccountGrant_ConsumeErrorPanics() {
	resourceID := sdkmath.NewUint(200)
	operator := sample.RandAccAddress()
	wanted := uint64(10)

	policy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect:    permtypes.EFFECT_ALLOW,
		Actions:   []permtypes.ActionType{permtypes.ACTION_CREATE_OBJECT},
		LimitSize: &common.UInt64Value{Value: 100},
	}}}

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(policy, true)
	s.permissionKeeper.EXPECT().PutPolicy(gomock.Any(), gomock.Any()).Return(sdkmath.ZeroUint(), errors.New("store full"))

	ctx := s.ctx.WithTxBytes([]byte{0x01})
	s.Require().Panics(func() {
		s.storageKeeper.VerifyPolicy(ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator,
			permtypes.ACTION_CREATE_OBJECT, &permtypes.VerifyOptions{WantedSize: &wanted})
	}, "a self-update failure must not be silently swallowed")
}

// TestVerifyPolicy_GroupGrant_ConsumeErrorPanics is the group-grant analogue
// of TestVerifyPolicy_AccountGrant_ConsumeErrorPanics.
func (s *TestSuite) TestVerifyPolicy_GroupGrant_ConsumeErrorPanics() {
	resourceID := sdkmath.NewUint(201)
	operator := sample.RandAccAddress()
	groupID := sdkmath.NewUint(15)
	policyID := sdkmath.NewUint(25)
	wanted := uint64(10)
	s.storageKeeper.SetGroupInfo(s.ctx, &types.GroupInfo{Id: groupID, GroupName: "panic-group"})

	policy := &permtypes.Policy{Statements: []*permtypes.Statement{{
		Effect:    permtypes.EFFECT_ALLOW,
		Actions:   []permtypes.ActionType{permtypes.ACTION_CREATE_OBJECT},
		LimitSize: &common.UInt64Value{Value: 100},
	}}}

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false)
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), resourceID, gnfdresource.RESOURCE_TYPE_BUCKET).
		Return(&permtypes.PolicyGroup{Items: []*permtypes.PolicyGroup_Item{{GroupId: groupID, PolicyId: policyID}}}, true)
	s.permissionKeeper.EXPECT().MustGetPolicyByID(gomock.Any(), policyID).Return(policy)
	s.permissionKeeper.EXPECT().GetGroupMember(gomock.Any(), groupID, operator).Return(&permtypes.GroupMember{}, true)
	s.permissionKeeper.EXPECT().PutPolicy(gomock.Any(), gomock.Any()).Return(sdkmath.ZeroUint(), errors.New("store full"))

	ctx := s.ctx.WithTxBytes([]byte{0x01})
	s.Require().Panics(func() {
		s.storageKeeper.VerifyPolicy(ctx, resourceID, gnfdresource.RESOURCE_TYPE_BUCKET, operator,
			permtypes.ACTION_CREATE_OBJECT, &permtypes.VerifyOptions{WantedSize: &wanted})
	})
}

func (s *TestSuite) TestGetPolicy_AccountFound() {
	bucketName := "getpolicy-account-bucket"
	owner := sample.RandAccAddress()
	principalAcc := sample.RandAccAddress()
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(1)})

	want := &permtypes.Policy{Id: sdkmath.NewUint(9)}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), sdkmath.NewUint(1), gnfdresource.RESOURCE_TYPE_BUCKET, principalAcc).Return(want, true)

	got, err := s.storageKeeper.GetPolicy(s.ctx, *types2.NewBucketGRN(bucketName), permtypes.NewPrincipalWithAccount(principalAcc))
	s.Require().NoError(err)
	s.Require().Equal(want, got)
}

func (s *TestSuite) TestGetPolicy_AccountNotFound() {
	bucketName := "getpolicy-account-notfound"
	owner := sample.RandAccAddress()
	principalAcc := sample.RandAccAddress()
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(2)})

	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), sdkmath.NewUint(2), gnfdresource.RESOURCE_TYPE_BUCKET, principalAcc).Return(nil, false)

	_, err := s.storageKeeper.GetPolicy(s.ctx, *types2.NewBucketGRN(bucketName), permtypes.NewPrincipalWithAccount(principalAcc))
	s.Require().ErrorIs(err, types.ErrNoSuchPolicy)
}

func (s *TestSuite) TestGetPolicy_GroupPrincipalFound() {
	objectOwner := sample.RandAccAddress()
	bucketName := "getpolicy-group-bucket"
	objectName := "getpolicy-group-object"
	s.storageKeeper.StoreObjectInfo(s.ctx, &types.ObjectInfo{
		Owner: objectOwner.String(), BucketName: bucketName, ObjectName: objectName, Id: sdkmath.NewUint(3),
	})
	principalGroupID := sdkmath.NewUint(44)

	want := &permtypes.Policy{Id: sdkmath.NewUint(19)}
	s.permissionKeeper.EXPECT().GetPolicyForGroup(gomock.Any(), sdkmath.NewUint(3), gnfdresource.RESOURCE_TYPE_OBJECT, principalGroupID).Return(want, true)

	got, err := s.storageKeeper.GetPolicy(s.ctx, *types2.NewObjectGRN(bucketName, objectName), permtypes.NewPrincipalWithGroupID(principalGroupID))
	s.Require().NoError(err)
	s.Require().Equal(want, got)
}

func (s *TestSuite) TestGetPolicy_GroupPrincipalNotFound() {
	owner := sample.RandAccAddress()
	groupName := "getpolicy-group-lookup"
	groupID, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)

	principalGroupID := sdkmath.NewUint(88)
	s.permissionKeeper.EXPECT().GetPolicyForGroup(gomock.Any(), groupID, gnfdresource.RESOURCE_TYPE_GROUP, principalGroupID).Return(nil, false)

	_, err = s.storageKeeper.GetPolicy(s.ctx, *types2.NewGroupGRN(owner, groupName), permtypes.NewPrincipalWithGroupID(principalGroupID))
	s.Require().ErrorIs(err, types.ErrNoSuchPolicy)
}

func (s *TestSuite) TestGetPolicy_InvalidPrincipalType() {
	bucketName := "getpolicy-badprincipal"
	owner := sample.RandAccAddress()
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(5)})

	_, err := s.storageKeeper.GetPolicy(s.ctx, *types2.NewBucketGRN(bucketName), &permtypes.Principal{})
	s.Require().ErrorIs(err, permtypes.ErrInvalidPrincipal)
}

func (s *TestSuite) TestGetPolicy_BucketResourceNotFound() {
	_, err := s.storageKeeper.GetPolicy(s.ctx, *types2.NewBucketGRN("no-such-bucket-for-getpolicy"), permtypes.NewPrincipalWithAccount(sample.RandAccAddress()))
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestGetPolicy_ObjectResourceNotFound() {
	_, err := s.storageKeeper.GetPolicy(s.ctx, *types2.NewObjectGRN("missing-bucket", "missing-object"), permtypes.NewPrincipalWithAccount(sample.RandAccAddress()))
	s.Require().ErrorIs(err, types.ErrNoSuchObject)
}

// TestGetPolicy_GroupResourceNotFound only asserts that resolving a
// nonexistent group's GRN fails: getResourceOwnerAndIDFromGRN's group branch
// returns ErrNoSuchBucket rather than ErrNoSuchGroup on a miss (see
// "Findings" in the PR body), so the specific error identity is deliberately
// left unpinned here.
func (s *TestSuite) TestGetPolicy_GroupResourceNotFound() {
	owner := sample.RandAccAddress()
	_, err := s.storageKeeper.GetPolicy(s.ctx, *types2.NewGroupGRN(owner, "missing-group-for-getpolicy"), permtypes.NewPrincipalWithAccount(sample.RandAccAddress()))
	s.Require().Error(err)
}

func (s *TestSuite) TestGetPolicy_UnspecifiedResourceType() {
	_, err := s.storageKeeper.GetPolicy(s.ctx, types2.GRN{}, permtypes.NewPrincipalWithAccount(sample.RandAccAddress()))
	s.Require().Error(err)
}

func (s *TestSuite) TestPutPolicy_ResourceLookupErrorPassthrough() {
	_, err := s.storageKeeper.PutPolicy(s.ctx, sample.RandAccAddress(), *types2.NewBucketGRN("no-such-bucket-for-putpolicy"),
		&permtypes.Policy{Principal: permtypes.NewPrincipalWithAccount(sample.RandAccAddress())})
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestPutPolicy_NonOwnerAccessDenied() {
	bucketName := "putpolicy-nonowner"
	owner := sample.RandAccAddress()
	nonOwner := sample.RandAccAddress()
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(21)})

	_, err := s.storageKeeper.PutPolicy(s.ctx, nonOwner, *types2.NewBucketGRN(bucketName),
		&permtypes.Policy{Principal: permtypes.NewPrincipalWithAccount(sample.RandAccAddress())})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

// TestPutPolicy_InvalidPrincipalRejected covers ValidatePrincipal's error
// passthrough: a bucket owner may not grant a policy to themselves.
func (s *TestSuite) TestPutPolicy_InvalidPrincipalRejected() {
	bucketName := "putpolicy-selfgrant"
	owner := sample.RandAccAddress()
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(22)})

	_, err := s.storageKeeper.PutPolicy(s.ctx, owner, *types2.NewBucketGRN(bucketName),
		&permtypes.Policy{Principal: permtypes.NewPrincipalWithAccount(owner)})
	s.Require().ErrorIs(err, gnfderrors.ErrInvalidPrincipal)
}

func (s *TestSuite) TestDeletePolicy_AccessDenied() {
	bucketName := "deletepolicy-denied"
	owner := sample.RandAccAddress()
	nonOwner := sample.RandAccAddress()
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(6)})

	_, err := s.storageKeeper.DeletePolicy(s.ctx, nonOwner, permtypes.NewPrincipalWithAccount(sample.RandAccAddress()), *types2.NewBucketGRN(bucketName))
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

func (s *TestSuite) TestDeletePolicy_ResourceNotFound() {
	_, err := s.storageKeeper.DeletePolicy(s.ctx, sample.RandAccAddress(), permtypes.NewPrincipalWithAccount(sample.RandAccAddress()),
		*types2.NewBucketGRN("no-such-bucket-for-deletepolicy"))
	s.Require().ErrorIs(err, types.ErrNoSuchBucket)
}

func (s *TestSuite) TestDeletePolicy_Success() {
	bucketName := "deletepolicy-success"
	owner := sample.RandAccAddress()
	principal := permtypes.NewPrincipalWithAccount(sample.RandAccAddress())
	s.storageKeeper.StoreBucketInfo(s.ctx, &types.BucketInfo{Owner: owner.String(), BucketName: bucketName, Id: sdkmath.NewUint(7)})

	s.permissionKeeper.EXPECT().DeletePolicy(gomock.Any(), principal, gnfdresource.RESOURCE_TYPE_BUCKET, sdkmath.NewUint(7)).Return(sdkmath.NewUint(1), nil)

	id, err := s.storageKeeper.DeletePolicy(s.ctx, owner, principal, *types2.NewBucketGRN(bucketName))
	s.Require().NoError(err)
	s.Require().Equal(sdkmath.NewUint(1), id)
}

func (s *TestSuite) TestNormalizePrincipal_NonGroupNoop() {
	principal := permtypes.NewPrincipalWithAccount(sample.RandAccAddress())
	original := principal.Value
	s.storageKeeper.NormalizePrincipal(s.ctx, principal)
	s.Require().Equal(original, principal.Value)
}

func (s *TestSuite) TestNormalizePrincipal_AlreadyNumericUnchanged() {
	principal := permtypes.NewPrincipalWithGroupID(sdkmath.NewUint(42))
	s.storageKeeper.NormalizePrincipal(s.ctx, principal)
	s.Require().Equal("42", principal.Value)
}

func (s *TestSuite) TestNormalizePrincipal_UnparsableValueUnchanged() {
	principal := &permtypes.Principal{Type: permtypes.PRINCIPAL_TYPE_GNFD_GROUP, Value: "not-a-grn-or-number"}
	s.storageKeeper.NormalizePrincipal(s.ctx, principal)
	s.Require().Equal("not-a-grn-or-number", principal.Value)
}

// TestNormalizePrincipal_NonGroupGRNUnchanged covers the branch where the
// value parses as a syntactically valid GRN whose type is not group (so
// GetGroupOwnerAndAccount errors and the principal is left untouched).
func (s *TestSuite) TestNormalizePrincipal_NonGroupGRNUnchanged() {
	bucketGRN := types2.NewBucketGRN("normalize-non-group-bucket").String()
	principal := &permtypes.Principal{Type: permtypes.PRINCIPAL_TYPE_GNFD_GROUP, Value: bucketGRN}
	s.storageKeeper.NormalizePrincipal(s.ctx, principal)
	s.Require().Equal(bucketGRN, principal.Value)
}

func (s *TestSuite) TestNormalizePrincipal_ResolvesExistingGroup() {
	owner := sample.RandAccAddress()
	groupName := "normalize-existing-group"
	groupID, err := s.storageKeeper.CreateGroup(s.ctx, owner, groupName, types.CreateGroupOptions{})
	s.Require().NoError(err)

	principal := permtypes.NewPrincipalWithGroupInfo(owner, groupName)
	s.storageKeeper.NormalizePrincipal(s.ctx, principal)
	s.Require().Equal(groupID.String(), principal.Value)
}

func (s *TestSuite) TestNormalizePrincipal_UnknownGroupGRNUnchanged() {
	owner := sample.RandAccAddress()
	principal := permtypes.NewPrincipalWithGroupInfo(owner, "does-not-exist-group")
	original := principal.Value
	s.storageKeeper.NormalizePrincipal(s.ctx, principal)
	s.Require().Equal(original, principal.Value, "an unresolvable group GRN must be left as-is")
}

func (s *TestSuite) TestValidatePrincipal_AccountInvalidHex() {
	principal := &permtypes.Principal{Type: permtypes.PRINCIPAL_TYPE_GNFD_ACCOUNT, Value: "not-hex"}
	err := s.storageKeeper.ValidatePrincipal(s.ctx, sample.RandAccAddress(), principal)
	s.Require().Error(err)
}

func (s *TestSuite) TestValidatePrincipal_AccountSelfGrantRejected() {
	resOwner := sample.RandAccAddress()
	principal := permtypes.NewPrincipalWithAccount(resOwner)
	err := s.storageKeeper.ValidatePrincipal(s.ctx, resOwner, principal)
	s.Require().ErrorIs(err, gnfderrors.ErrInvalidPrincipal)
}

func (s *TestSuite) TestValidatePrincipal_AccountValid() {
	principal := permtypes.NewPrincipalWithAccount(sample.RandAccAddress())
	err := s.storageKeeper.ValidatePrincipal(s.ctx, sample.RandAccAddress(), principal)
	s.Require().NoError(err)
}

func (s *TestSuite) TestValidatePrincipal_GroupInvalidID() {
	principal := &permtypes.Principal{Type: permtypes.PRINCIPAL_TYPE_GNFD_GROUP, Value: "not-a-number"}
	err := s.storageKeeper.ValidatePrincipal(s.ctx, sample.RandAccAddress(), principal)
	s.Require().Error(err)
}

func (s *TestSuite) TestValidatePrincipal_GroupNotFound() {
	principal := permtypes.NewPrincipalWithGroupID(sdkmath.NewUint(999))
	err := s.storageKeeper.ValidatePrincipal(s.ctx, sample.RandAccAddress(), principal)
	s.Require().ErrorIs(err, types.ErrNoSuchGroup)
}

func (s *TestSuite) TestValidatePrincipal_GroupFound() {
	owner := sample.RandAccAddress()
	groupID, err := s.storageKeeper.CreateGroup(s.ctx, owner, "validateprincipal-group", types.CreateGroupOptions{})
	s.Require().NoError(err)

	principal := permtypes.NewPrincipalWithGroupID(groupID)
	verr := s.storageKeeper.ValidatePrincipal(s.ctx, sample.RandAccAddress(), principal)
	s.Require().NoError(verr)
}

func (s *TestSuite) TestValidatePrincipal_UnknownTypeRejected() {
	principal := &permtypes.Principal{Type: permtypes.PRINCIPAL_TYPE_UNSPECIFIED}
	err := s.storageKeeper.ValidatePrincipal(s.ctx, sample.RandAccAddress(), principal)
	s.Require().ErrorIs(err, permtypes.ErrInvalidPrincipal)
}
