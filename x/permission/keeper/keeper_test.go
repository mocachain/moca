package keeper_test

import (
	"math/rand"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/x/permission/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
)

func (s *TestSuite) TestPruneAccountPolicies() {
	now := s.ctx.BlockTime()
	oneDayAfter := now.AddDate(0, 0, 1)

	resourceIDs := []math.Uint{math.NewUint(rand.Uint64()), math.NewUint(rand.Uint64()), math.NewUint(rand.Uint64())} //nolint: gosec
	policyIDs := make([]math.Uint, 3)

	// policy without expiry
	policy := types.Policy{
		Principal: &types.Principal{
			Type:  types.PRINCIPAL_TYPE_GNFD_ACCOUNT,
			Value: sample.RandAccAddressHex(),
		},
		ResourceType:   1,
		ResourceId:     resourceIDs[0],
		Statements:     nil,
		ExpirationTime: nil,
	}
	policyID, err := s.permissionKeeper.PutPolicy(s.ctx, &policy)
	s.NoError(err)
	policyIDs[0] = policyID

	policy.ResourceId = resourceIDs[2]
	policyID, err = s.permissionKeeper.PutPolicy(s.ctx, &policy)
	s.NoError(err)
	policyIDs[2] = policyID

	// policy with expiry
	policy.ResourceId = resourceIDs[1]
	policy.ExpirationTime = &oneDayAfter
	policyID, err = s.permissionKeeper.PutPolicy(s.ctx, &policy)
	s.NoError(err)
	policyIDs[1] = policyID

	testCases := []struct {
		name       string
		ctx        sdk.Context
		resourceID math.Uint
		policyID   math.Uint
		found      bool
		preRun     func()
		postRun    func()
	}{
		{
			name:       "no expiry and no prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter),
			resourceID: resourceIDs[0],
			policyID:   policyIDs[0],
			found:      true,
		},
		{
			name:       "expiry and no prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter),
			resourceID: resourceIDs[1],
			policyID:   policyIDs[1],
			found:      true,
		},
		{
			name:       "expiry and prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter.Add(time.Second)),
			resourceID: resourceIDs[1],
			policyID:   policyIDs[1],
		},
		{
			name:       "update from no expiry to expiry and prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter.Add(time.Second)),
			resourceID: resourceIDs[0],
			policyID:   policyIDs[0],
			preRun: func() {
				oldPolicy, found := s.permissionKeeper.GetPolicyByID(s.ctx, policyIDs[0])
				s.True(found)
				oldPolicy.ExpirationTime = &oneDayAfter
				newID, err := s.permissionKeeper.PutPolicy(s.ctx, oldPolicy)
				s.NoError(err)
				s.Equal(policyIDs[0], newID)
			},
		},
		{
			name:       "update from expiry to no expiry and no prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter.Add(time.Second)),
			resourceID: resourceIDs[2],
			policyID:   policyIDs[2],
			found:      true,
			preRun: func() {
				oldPolicy, found := s.permissionKeeper.GetPolicyByID(s.ctx, policyIDs[2])
				s.True(found)
				oldPolicy.ExpirationTime = nil
				newID, err := s.permissionKeeper.PutPolicy(s.ctx, oldPolicy)
				s.NoError(err)
				s.Equal(policyIDs[2], newID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			if tc.preRun != nil {
				tc.preRun()
			}
			_, found := s.permissionKeeper.GetPolicyByID(tc.ctx, tc.policyID)
			s.True(found)
			s.permissionKeeper.RemoveExpiredPolicies(tc.ctx)
			_, found = s.permissionKeeper.GetPolicyByID(tc.ctx, tc.policyID)
			s.Equal(tc.found, found)
			if tc.postRun != nil {
				tc.postRun()
			}
		})
	}
}

// TestPutPolicy_MaximumStatementsNum is a
// : MaximumStatementsNum was defined and readable via
// k.MaximumStatementsNum(ctx), but no caller ever checked a policy's statement
// count against it, so PutPolicy accepted a Policy with an unbounded number of
// Statements. This mirrors the existing MaximumPolicyGroupSize enforcement a
// few lines up in Keeper.PutPolicy.
func (s *TestSuite) TestPutPolicy_MaximumStatementsNum() {
	makeStatements := func(n int) []*types.Statement {
		stmts := make([]*types.Statement, n)
		for i := range stmts {
			stmts[i] = &types.Statement{
				Effect:  types.EFFECT_ALLOW,
				Actions: []types.ActionType{types.ACTION_GET_OBJECT},
			}
		}
		return stmts
	}

	capNum := int(s.permissionKeeper.MaximumStatementsNum(s.ctx)) //nolint:gosec // a statement cap is small
	s.Require().Equal(int(types.DefaultMaxStatementsNum), capNum, "test assumes the default cap is in effect")

	// Exactly at the cap must still be accepted.
	atCap := types.Policy{
		Principal: &types.Principal{
			Type:  types.PRINCIPAL_TYPE_GNFD_ACCOUNT,
			Value: sample.RandAccAddressHex(),
		},
		ResourceType: 1,
		ResourceId:   math.NewUint(rand.Uint64()), //nolint: gosec
		Statements:   makeStatements(capNum),
	}
	_, err := s.permissionKeeper.PutPolicy(s.ctx, &atCap)
	s.Require().NoError(err, "a policy with exactly MaximumStatementsNum statements must be accepted")

	// One over the cap must be rejected.
	overCap := types.Policy{
		Principal: &types.Principal{
			Type:  types.PRINCIPAL_TYPE_GNFD_ACCOUNT,
			Value: sample.RandAccAddressHex(),
		},
		ResourceType: 1,
		ResourceId:   math.NewUint(rand.Uint64()), //nolint: gosec
		Statements:   makeStatements(capNum + 1),
	}
	_, err = s.permissionKeeper.PutPolicy(s.ctx, &overCap)
	s.Require().Error(err, "a policy exceeding MaximumStatementsNum must be rejected")
	s.Require().ErrorIs(err, types.ErrLimitExceeded)
}

func (s *TestSuite) TestPruneGroupPolicies() {
	now := s.ctx.BlockTime()
	oneDayAfter := now.AddDate(0, 0, 1)

	resourceIDs := []math.Uint{math.NewUint(rand.Uint64()), math.NewUint(rand.Uint64()), math.NewUint(rand.Uint64())} //nolint: gosec
	policyIDs := make([]math.Uint, 3)

	// member without expiry
	policy := types.Policy{
		Principal: &types.Principal{
			Type:  types.PRINCIPAL_TYPE_GNFD_GROUP,
			Value: sample.RandAccAddressHex(),
		},
		ResourceType:   1,
		ResourceId:     resourceIDs[0],
		Statements:     nil,
		ExpirationTime: nil,
	}
	policyID, err := s.permissionKeeper.PutPolicy(s.ctx, &policy)
	s.NoError(err)
	policyIDs[0] = policyID

	policy.ResourceId = resourceIDs[2]
	policyID, err = s.permissionKeeper.PutPolicy(s.ctx, &policy)
	s.NoError(err)
	policyIDs[2] = policyID

	// member with expiry
	policy.ResourceId = resourceIDs[1]
	policy.ExpirationTime = &oneDayAfter
	policyID, err = s.permissionKeeper.PutPolicy(s.ctx, &policy)
	s.NoError(err)
	policyIDs[1] = policyID

	testCases := []struct {
		name       string
		ctx        sdk.Context
		resourceID math.Uint
		policyID   math.Uint
		found      bool
		preRun     func()
		postRun    func()
	}{
		{
			name:       "no expiry and no prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter),
			resourceID: resourceIDs[0],
			policyID:   policyIDs[0],
			found:      true,
		},
		{
			name:       "expiry and no prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter),
			resourceID: resourceIDs[1],
			policyID:   policyIDs[1],
			found:      true,
		},
		{
			name:       "expiry and prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter.Add(time.Second)),
			resourceID: resourceIDs[1],
			policyID:   policyIDs[1],
		},
		{
			name:       "update from no expiry to expiry and prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter.Add(time.Second)),
			resourceID: resourceIDs[0],
			policyID:   policyIDs[0],
			preRun: func() {
				oldPolicy, found := s.permissionKeeper.GetPolicyByID(s.ctx, policyIDs[0])
				s.True(found)
				oldPolicy.ExpirationTime = &oneDayAfter
				newID, err := s.permissionKeeper.PutPolicy(s.ctx, oldPolicy)
				s.NoError(err)
				s.Equal(policyIDs[0], newID)
			},
		},
		{
			name:       "update from expiry to no expiry and no prune",
			ctx:        s.ctx.WithBlockTime(oneDayAfter.Add(time.Second)),
			resourceID: resourceIDs[2],
			policyID:   policyIDs[2],
			found:      true,
			preRun: func() {
				oldPolicy, found := s.permissionKeeper.GetPolicyByID(s.ctx, policyIDs[2])
				s.True(found)
				oldPolicy.ExpirationTime = nil
				newID, err := s.permissionKeeper.PutPolicy(s.ctx, oldPolicy)
				s.NoError(err)
				s.Equal(policyIDs[2], newID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			if tc.preRun != nil {
				tc.preRun()
			}
			_, found := s.permissionKeeper.GetPolicyByID(tc.ctx, tc.policyID)
			s.True(found)
			s.permissionKeeper.RemoveExpiredPolicies(tc.ctx)
			_, found = s.permissionKeeper.GetPolicyByID(tc.ctx, tc.policyID)
			s.Equal(tc.found, found)
			if tc.postRun != nil {
				tc.postRun()
			}
		})
	}
}

// TestPutPolicy_StatementsCapDoesNotBrickStoredPolicies pins that the cap bounds
// growth only. A policy stored before the cap was enforced (or before governance
// lowered it) may already exceed it; rejecting a write that adds no statement
// would break the LimitSize self-update in x/storage/keeper/permission.go, which
// panics on any error from PutPolicy.
func (s *TestSuite) TestPutPolicy_StatementsCapDoesNotBrickStoredPolicies() {
	resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
	principal := &types.Principal{
		Type:  types.PRINCIPAL_TYPE_GNFD_ACCOUNT,
		Value: sample.RandAccAddressHex(),
	}
	overCap := int(types.DefaultMaxStatementsNum) + 3

	makeStatements := func(n int) []*types.Statement {
		stmts := make([]*types.Statement, n)
		for i := range stmts {
			stmts[i] = &types.Statement{
				Effect:  types.EFFECT_ALLOW,
				Actions: []types.ActionType{types.ACTION_GET_OBJECT},
			}
		}
		return stmts
	}

	// Store an over-cap policy the way one could exist before the cap was enforced.
	loose := types.DefaultParams()
	loose.MaximumStatementsNum = uint64(overCap)
	s.Require().NoError(s.permissionKeeper.SetParams(s.ctx, loose))
	_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    principal,
		ResourceType: 1,
		ResourceId:   resourceID,
		Statements:   makeStatements(overCap),
	})
	s.Require().NoError(err)
	s.Require().NoError(s.permissionKeeper.SetParams(s.ctx, types.DefaultParams()))

	// Same count: allowed, this is what the quota self-update writes back.
	_, err = s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    principal,
		ResourceType: 1,
		ResourceId:   resourceID,
		Statements:   makeStatements(overCap),
	})
	s.Require().NoError(err, "rewriting a stored over-cap policy without adding statements must be allowed")

	// Fewer: allowed, the owner shrinking back towards the cap.
	_, err = s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    principal,
		ResourceType: 1,
		ResourceId:   resourceID,
		Statements:   makeStatements(overCap - 1),
	})
	s.Require().NoError(err, "shrinking a stored over-cap policy must be allowed")

	// More: still rejected, the cap must keep bounding growth.
	_, err = s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    principal,
		ResourceType: 1,
		ResourceId:   resourceID,
		Statements:   makeStatements(overCap),
	})
	s.Require().ErrorIs(err, types.ErrLimitExceeded,
		"growing an over-cap policy further must still be rejected")
}

func (s *TestSuite) TestGetAuthority() {
	s.Require().Equal(authtypes.NewModuleAddress(govtypes.ModuleName).String(), s.permissionKeeper.GetAuthority())
}

func (s *TestSuite) TestLogger() {
	s.Require().NotNil(s.permissionKeeper.Logger(s.ctx))
}

func (s *TestSuite) TestAddGroupMember() {
	groupID := math.NewUint(rand.Uint64()) //nolint: gosec
	member := sample.RandAccAddress()
	exp := s.ctx.BlockTime().AddDate(0, 0, 1)

	err := s.permissionKeeper.AddGroupMember(s.ctx, groupID, member, &exp)
	s.Require().NoError(err)

	gm, found := s.permissionKeeper.GetGroupMember(s.ctx, groupID, member)
	s.Require().True(found)
	s.Require().True(groupID.Equal(gm.GroupId))
	s.Require().Equal(member.String(), gm.Member)
	s.Require().NotNil(gm.ExpirationTime)
	s.Require().True(exp.Equal(*gm.ExpirationTime))

	// adding the same (groupID, member) pair again must be rejected.
	err = s.permissionKeeper.AddGroupMember(s.ctx, groupID, member, &exp)
	s.Require().ErrorIs(err, storagetypes.ErrGroupMemberAlreadyExists)
}

func (s *TestSuite) TestUpdateGroupMember() {
	groupID := math.NewUint(rand.Uint64()) //nolint: gosec
	member := sample.RandAccAddress()

	s.Require().NoError(s.permissionKeeper.AddGroupMember(s.ctx, groupID, member, nil))
	added, found := s.permissionKeeper.GetGroupMember(s.ctx, groupID, member)
	s.Require().True(found)
	s.Require().Nil(added.ExpirationTime)

	newExp := s.ctx.BlockTime().AddDate(0, 0, 2)
	s.permissionKeeper.UpdateGroupMember(s.ctx, groupID, member, added.Id, &newExp)

	updated, found := s.permissionKeeper.GetGroupMemberByID(s.ctx, added.Id)
	s.Require().True(found)
	s.Require().Equal(member.String(), updated.Member)
	s.Require().NotNil(updated.ExpirationTime)
	s.Require().True(newExp.Equal(*updated.ExpirationTime))
}

func (s *TestSuite) TestRemoveGroupMember() {
	groupID := math.NewUint(rand.Uint64()) //nolint: gosec
	member := sample.RandAccAddress()

	s.Require().NoError(s.permissionKeeper.AddGroupMember(s.ctx, groupID, member, nil))
	s.Require().NoError(s.permissionKeeper.RemoveGroupMember(s.ctx, groupID, member))

	_, found := s.permissionKeeper.GetGroupMember(s.ctx, groupID, member)
	s.Require().False(found)

	// removing an already-removed member must report the not-found error.
	err := s.permissionKeeper.RemoveGroupMember(s.ctx, groupID, member)
	s.Require().ErrorIs(err, storagetypes.ErrNoSuchGroupMember)
}

func (s *TestSuite) TestGetGroupMember_NotFound() {
	_, found := s.permissionKeeper.GetGroupMember(s.ctx, math.NewUint(rand.Uint64()), sample.RandAccAddress()) //nolint: gosec
	s.Require().False(found)
}

func (s *TestSuite) TestGetGroupMemberByID_NotFound() {
	_, found := s.permissionKeeper.GetGroupMemberByID(s.ctx, math.NewUint(rand.Uint64())) //nolint: gosec
	s.Require().False(found)
}

// TestPutPolicy_StatementsCapDoesNotBrickStoredPolicies_GroupPrincipal mirrors
// TestPutPolicy_StatementsCapDoesNotBrickStoredPolicies with a group principal instead of an
// account principal: only a group principal drives storedStatementsNum's
// PRINCIPAL_TYPE_GNFD_GROUP branch (resolved via GetPolicyForGroup), which every other test in
// this file leaves dead because they either use account principals or store zero statements.
func (s *TestSuite) TestPutPolicy_StatementsCapDoesNotBrickStoredPolicies_GroupPrincipal() {
	resourceID := math.NewUint(rand.Uint64())                               //nolint: gosec
	principal := types.NewPrincipalWithGroupID(math.NewUint(rand.Uint64())) //nolint: gosec
	overCap := int(types.DefaultMaxStatementsNum) + 3

	makeStatements := func(n int) []*types.Statement {
		stmts := make([]*types.Statement, n)
		for i := range stmts {
			stmts[i] = &types.Statement{
				Effect:  types.EFFECT_ALLOW,
				Actions: []types.ActionType{types.ACTION_GET_OBJECT},
			}
		}
		return stmts
	}

	// Store an over-cap group policy the way one could exist before the cap was enforced.
	loose := types.DefaultParams()
	loose.MaximumStatementsNum = uint64(overCap) //nolint: gosec
	s.Require().NoError(s.permissionKeeper.SetParams(s.ctx, loose))
	_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    principal,
		ResourceType: 1,
		ResourceId:   resourceID,
		Statements:   makeStatements(overCap),
	})
	s.Require().NoError(err)
	s.Require().NoError(s.permissionKeeper.SetParams(s.ctx, types.DefaultParams()))

	// Same count: allowed. This forces storedStatementsNum's group branch, which must report
	// the already-stored count so the write is treated as a same-size update, not growth.
	_, err = s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    principal,
		ResourceType: 1,
		ResourceId:   resourceID,
		Statements:   makeStatements(overCap),
	})
	s.Require().NoError(err, "rewriting a stored over-cap group policy without adding statements must be allowed")

	// More: still rejected, the cap must keep bounding growth.
	_, err = s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    principal,
		ResourceType: 1,
		ResourceId:   resourceID,
		Statements:   makeStatements(overCap + 1),
	})
	s.Require().Error(err, "growing an over-cap group policy further must still be rejected")
	s.Require().ErrorIs(err, types.ErrLimitExceeded)
}

func (s *TestSuite) TestGetPolicyGroupForResource() {
	resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
	_, found := s.permissionKeeper.GetPolicyGroupForResource(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET)
	s.Require().False(found)

	groupID := math.NewUint(rand.Uint64()) //nolint: gosec
	_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    types.NewPrincipalWithGroupID(groupID),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   resourceID,
	})
	s.Require().NoError(err)

	group, found := s.permissionKeeper.GetPolicyGroupForResource(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET)
	s.Require().True(found)
	s.Require().Len(group.Items, 1)
	s.Require().True(group.Items[0].GroupId.Equal(groupID))
}

func (s *TestSuite) TestGetPolicyForGroup_NotFound() {
	resourceID := math.NewUint(rand.Uint64())     //nolint: gosec
	otherGroupID := math.NewUint(rand.Uint64())   //nolint: gosec
	missingGroupID := math.NewUint(rand.Uint64()) //nolint: gosec

	// no PolicyGroup stored at all for this resource.
	_, found := s.permissionKeeper.GetPolicyForGroup(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET, missingGroupID)
	s.Require().False(found)

	// a PolicyGroup exists for the resource, but not for missingGroupID.
	_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    types.NewPrincipalWithGroupID(otherGroupID),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   resourceID,
	})
	s.Require().NoError(err)
	_, found = s.permissionKeeper.GetPolicyForGroup(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET, missingGroupID)
	s.Require().False(found)
}

func (s *TestSuite) TestDeletePolicy() {
	s.Run("account principal found", func() {
		addr := sample.RandAccAddress()
		resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
		exp := s.ctx.BlockTime().AddDate(0, 0, 1)

		putID, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
			Principal:      types.NewPrincipalWithAccount(addr),
			ResourceType:   resource.RESOURCE_TYPE_BUCKET,
			ResourceId:     resourceID,
			ExpirationTime: &exp,
		})
		s.Require().NoError(err)

		deletedID, err := s.permissionKeeper.DeletePolicy(s.ctx, types.NewPrincipalWithAccount(addr), resource.RESOURCE_TYPE_BUCKET, resourceID)
		s.Require().NoError(err)
		s.Require().True(putID.Equal(deletedID))

		_, found := s.permissionKeeper.GetPolicyForAccount(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET, addr)
		s.Require().False(found)
		_, found = s.permissionKeeper.GetPolicyByID(s.ctx, deletedID)
		s.Require().False(found)
	})

	s.Run("account principal not found", func() {
		addr := sample.RandAccAddress()
		resourceID := math.NewUint(rand.Uint64()) //nolint: gosec

		deletedID, err := s.permissionKeeper.DeletePolicy(s.ctx, types.NewPrincipalWithAccount(addr), resource.RESOURCE_TYPE_BUCKET, resourceID)
		s.Require().NoError(err)
		s.Require().True(deletedID.IsNil(), "no policy existed, so DeletePolicy must not report an id")
	})

	s.Run("group principal found, items remain", func() {
		resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
		groupID1 := math.NewUint(rand.Uint64())   //nolint: gosec
		groupID2 := math.NewUint(rand.Uint64())   //nolint: gosec

		_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
			Principal:    types.NewPrincipalWithGroupID(groupID1),
			ResourceType: resource.RESOURCE_TYPE_BUCKET,
			ResourceId:   resourceID,
		})
		s.Require().NoError(err)
		_, err = s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
			Principal:    types.NewPrincipalWithGroupID(groupID2),
			ResourceType: resource.RESOURCE_TYPE_BUCKET,
			ResourceId:   resourceID,
		})
		s.Require().NoError(err)

		deletedID, err := s.permissionKeeper.DeletePolicy(s.ctx, types.NewPrincipalWithGroupID(groupID1), resource.RESOURCE_TYPE_BUCKET, resourceID)
		s.Require().NoError(err)
		s.Require().False(deletedID.IsNil())

		_, found := s.permissionKeeper.GetPolicyForGroup(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET, groupID1)
		s.Require().False(found)
		group, found := s.permissionKeeper.GetPolicyGroupForResource(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET)
		s.Require().True(found)
		s.Require().Len(group.Items, 1)
		s.Require().True(group.Items[0].GroupId.Equal(groupID2))
	})

	s.Run("group principal found, becomes empty", func() {
		resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
		groupID := math.NewUint(rand.Uint64())    //nolint: gosec
		exp := s.ctx.BlockTime().AddDate(0, 0, 1)

		_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
			Principal:      types.NewPrincipalWithGroupID(groupID),
			ResourceType:   resource.RESOURCE_TYPE_BUCKET,
			ResourceId:     resourceID,
			ExpirationTime: &exp,
		})
		s.Require().NoError(err)

		deletedID, err := s.permissionKeeper.DeletePolicy(s.ctx, types.NewPrincipalWithGroupID(groupID), resource.RESOURCE_TYPE_BUCKET, resourceID)
		s.Require().NoError(err)
		s.Require().False(deletedID.IsNil())

		_, found := s.permissionKeeper.GetPolicyGroupForResource(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET)
		s.Require().False(found)
	})

	s.Run("group principal with unparsable group id", func() {
		badPrincipal := &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_GROUP, Value: "not-a-number"}
		_, err := s.permissionKeeper.DeletePolicy(s.ctx, badPrincipal, resource.RESOURCE_TYPE_BUCKET, math.NewUint(rand.Uint64())) //nolint: gosec
		s.Require().ErrorIs(err, types.ErrInvalidPrincipal)
	})

	s.Run("unknown principal type", func() {
		_, err := s.permissionKeeper.DeletePolicy(s.ctx, &types.Principal{Type: types.PRINCIPAL_TYPE_UNSPECIFIED},
			resource.RESOURCE_TYPE_BUCKET, math.NewUint(rand.Uint64())) //nolint: gosec
		s.Require().ErrorIs(err, types.ErrInvalidPrincipal)
	})
}

// TestPutPolicy_UnknownPrincipalType covers PutPolicy's default switch branch, the PRINCIPAL_TYPE_UNSPECIFIED case.
func (s *TestSuite) TestPutPolicy_UnknownPrincipalType() {
	_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    &types.Principal{Type: types.PRINCIPAL_TYPE_UNSPECIFIED},
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   math.NewUint(rand.Uint64()), //nolint: gosec
	})
	s.Require().ErrorIs(err, types.ErrInvalidPrincipal)
}

func (s *TestSuite) TestMustGetPolicyByID_PanicsWhenNotFound() {
	s.Require().Panics(func() {
		s.permissionKeeper.MustGetPolicyByID(s.ctx, math.NewUint(rand.Uint64())) //nolint: gosec
	})
}

// TestPutPolicy_UpdateExpirationTransitions exercises updatePolicy's expiry-transition branches
// that TestPruneAccountPolicies/TestPruneGroupPolicies never reach in practice (both reuse a
// mutable Policy across PutPolicy calls, so the policy read back to update always turns out to
// have had no expiration in the first place): shrinking an expiring policy back to non-expiring,
// and moving from one expiration time straight to a different one. Like those tests, updates are
// applied by mutating a policy fetched via GetPolicyByID (which carries its real Id) rather than
// a fresh literal, since PutPolicy's override-write path returns the caller's own Id field as-is.
func (s *TestSuite) TestPutPolicy_UpdateExpirationTransitions() {
	addr := sample.RandAccAddress()
	resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
	firstExp := s.ctx.BlockTime().AddDate(0, 0, 1)

	putID, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:      types.NewPrincipalWithAccount(addr),
		ResourceType:   resource.RESOURCE_TYPE_BUCKET,
		ResourceId:     resourceID,
		ExpirationTime: &firstExp,
	})
	s.Require().NoError(err)

	// expiry -> a different expiry.
	secondExp := firstExp.AddDate(0, 0, 1)
	stored, found := s.permissionKeeper.GetPolicyByID(s.ctx, putID)
	s.Require().True(found)
	stored.ExpirationTime = &secondExp
	_, err = s.permissionKeeper.PutPolicy(s.ctx, stored)
	s.Require().NoError(err)
	stored, found = s.permissionKeeper.GetPolicyByID(s.ctx, putID)
	s.Require().True(found)
	s.Require().True(secondExp.Equal(*stored.ExpirationTime))

	// expiry -> no expiry.
	stored.ExpirationTime = nil
	_, err = s.permissionKeeper.PutPolicy(s.ctx, stored)
	s.Require().NoError(err)
	stored, found = s.permissionKeeper.GetPolicyByID(s.ctx, putID)
	s.Require().True(found)
	s.Require().Nil(stored.ExpirationTime)
}

func (s *TestSuite) TestForceDeleteAccountPolicyForResource() {
	total, complete := s.permissionKeeper.ForceDeleteAccountPolicyForResource(s.ctx, 5, 3,
		resource.RESOURCE_TYPE_UNSPECIFIED, math.NewUint(rand.Uint64())) //nolint: gosec
	s.Require().True(complete)
	s.Require().Equal(uint64(3), total)

	resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
	a1, a2, a3, a4 := sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress()
	allAddrs := []sdk.AccAddress{a1, a2, a3, a4}
	exp := s.ctx.BlockTime().AddDate(0, 0, 1)
	for i, addr := range allAddrs {
		policy := &types.Policy{
			Principal:    types.NewPrincipalWithAccount(addr),
			ResourceType: resource.RESOURCE_TYPE_BUCKET,
			ResourceId:   resourceID,
		}
		if i == len(allAddrs)-1 {
			policy.ExpirationTime = &exp
		}
		_, err := s.permissionKeeper.PutPolicy(s.ctx, policy)
		s.Require().NoError(err)
	}

	existsFor := func(addr sdk.AccAddress) bool {
		_, found := s.permissionKeeper.GetPolicyForAccount(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET, addr)
		return found
	}

	// Partial: pause after two deletions, the rest must still be readable.
	total, complete = s.permissionKeeper.ForceDeleteAccountPolicyForResource(s.ctx, 2, 0, resource.RESOURCE_TYPE_BUCKET, resourceID)
	s.Require().False(complete)
	s.Require().Equal(uint64(2), total)
	remaining := 0
	for _, addr := range allAddrs {
		if existsFor(addr) {
			remaining++
		}
	}
	s.Require().Equal(2, remaining)
	s.Require().True(s.permissionKeeper.ExistAccountPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))

	// Resume and finish: raising maxDelete above the remaining count completes the GC.
	total, complete = s.permissionKeeper.ForceDeleteAccountPolicyForResource(s.ctx, 10, total, resource.RESOURCE_TYPE_BUCKET, resourceID)
	s.Require().True(complete)
	s.Require().Equal(uint64(4), total)
	s.Require().False(s.permissionKeeper.ExistAccountPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))
	for _, addr := range allAddrs {
		s.Require().False(existsFor(addr))
	}
}

func (s *TestSuite) TestForceDeleteGroupPolicyForResource() {
	total, complete := s.permissionKeeper.ForceDeleteGroupPolicyForResource(s.ctx, 5, 3,
		resource.RESOURCE_TYPE_UNSPECIFIED, math.NewUint(rand.Uint64())) //nolint: gosec
	s.Require().True(complete)
	s.Require().Equal(uint64(3), total)

	total, complete = s.permissionKeeper.ForceDeleteGroupPolicyForResource(s.ctx, 5, 3,
		resource.RESOURCE_TYPE_GROUP, math.NewUint(rand.Uint64())) //nolint: gosec
	s.Require().True(complete)
	s.Require().Equal(uint64(3), total)

	resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
	g1 := math.NewUint(rand.Uint64())         //nolint: gosec
	g2 := math.NewUint(rand.Uint64())         //nolint: gosec
	g3 := math.NewUint(rand.Uint64())         //nolint: gosec
	g4 := math.NewUint(rand.Uint64())         //nolint: gosec
	allGroups := []math.Uint{g1, g2, g3, g4}
	exp := s.ctx.BlockTime().AddDate(0, 0, 1)
	for i, groupID := range allGroups {
		policy := &types.Policy{
			Principal:    types.NewPrincipalWithGroupID(groupID),
			ResourceType: resource.RESOURCE_TYPE_BUCKET,
			ResourceId:   resourceID,
		}
		if i == 1 {
			// g2 is appended to an already-existing PolicyGroup (g1 created it); giving it an
			// expiration covers that append branch's own "has expiration" line, which is distinct
			// from the fresh-PolicyGroup creation path g1 takes.
			policy.ExpirationTime = &exp
		}
		_, err := s.permissionKeeper.PutPolicy(s.ctx, policy)
		s.Require().NoError(err)
	}

	// Partial: the persisted PolicyGroup must be rewritten with the untouched tail, not deleted.
	total, complete = s.permissionKeeper.ForceDeleteGroupPolicyForResource(s.ctx, 2, 0, resource.RESOURCE_TYPE_BUCKET, resourceID)
	s.Require().False(complete)
	s.Require().Equal(uint64(2), total)
	group, found := s.permissionKeeper.GetPolicyGroupForResource(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET)
	s.Require().True(found)
	s.Require().Len(group.Items, 2)
	s.Require().True(s.permissionKeeper.ExistGroupPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))

	// Resume and finish.
	total, complete = s.permissionKeeper.ForceDeleteGroupPolicyForResource(s.ctx, 10, total, resource.RESOURCE_TYPE_BUCKET, resourceID)
	s.Require().True(complete)
	s.Require().Equal(uint64(4), total)
	s.Require().False(s.permissionKeeper.ExistGroupPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))
	_, found = s.permissionKeeper.GetPolicyGroupForResource(s.ctx, resourceID, resource.RESOURCE_TYPE_BUCKET)
	s.Require().False(found)
}

func (s *TestSuite) TestForceDeleteGroupMembers() {
	groupID := math.NewUint(rand.Uint64()) //nolint: gosec
	m1, m2, m3, m4 := sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress()
	allMembers := []sdk.AccAddress{m1, m2, m3, m4}
	for _, member := range allMembers {
		s.Require().NoError(s.permissionKeeper.AddGroupMember(s.ctx, groupID, member, nil))
	}

	memberExists := func(member sdk.AccAddress) bool {
		_, found := s.permissionKeeper.GetGroupMember(s.ctx, groupID, member)
		return found
	}

	total, complete := s.permissionKeeper.ForceDeleteGroupMembers(s.ctx, 2, 0, groupID)
	s.Require().False(complete)
	s.Require().Equal(uint64(2), total)
	remaining := 0
	for _, member := range allMembers {
		if memberExists(member) {
			remaining++
		}
	}
	s.Require().Equal(2, remaining)
	s.Require().True(s.permissionKeeper.ExistGroupMemberForGroup(s.ctx, groupID))

	total, complete = s.permissionKeeper.ForceDeleteGroupMembers(s.ctx, 10, total, groupID)
	s.Require().True(complete)
	s.Require().Equal(uint64(4), total)
	s.Require().False(s.permissionKeeper.ExistGroupMemberForGroup(s.ctx, groupID))
	for _, member := range allMembers {
		s.Require().False(memberExists(member))
	}
}

func (s *TestSuite) TestExistAccountPolicyForResource() {
	s.Require().False(s.permissionKeeper.ExistAccountPolicyForResource(s.ctx, resource.RESOURCE_TYPE_UNSPECIFIED, math.NewUint(rand.Uint64()))) //nolint: gosec

	resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
	s.Require().False(s.permissionKeeper.ExistAccountPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))

	_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   resourceID,
	})
	s.Require().NoError(err)
	s.Require().True(s.permissionKeeper.ExistAccountPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))
}

func (s *TestSuite) TestExistGroupPolicyForResource() {
	resourceID := math.NewUint(rand.Uint64()) //nolint: gosec
	s.Require().False(s.permissionKeeper.ExistGroupPolicyForResource(s.ctx, resource.RESOURCE_TYPE_UNSPECIFIED, resourceID))
	s.Require().False(s.permissionKeeper.ExistGroupPolicyForResource(s.ctx, resource.RESOURCE_TYPE_GROUP, resourceID))
	s.Require().False(s.permissionKeeper.ExistGroupPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))

	_, err := s.permissionKeeper.PutPolicy(s.ctx, &types.Policy{
		Principal:    types.NewPrincipalWithGroupID(math.NewUint(rand.Uint64())), //nolint: gosec
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   resourceID,
	})
	s.Require().NoError(err)
	s.Require().True(s.permissionKeeper.ExistGroupPolicyForResource(s.ctx, resource.RESOURCE_TYPE_BUCKET, resourceID))
}

func (s *TestSuite) TestExistGroupMemberForGroup() {
	groupID := math.NewUint(rand.Uint64()) //nolint: gosec
	s.Require().False(s.permissionKeeper.ExistGroupMemberForGroup(s.ctx, groupID))

	s.Require().NoError(s.permissionKeeper.AddGroupMember(s.ctx, groupID, sample.RandAccAddress(), nil))
	s.Require().True(s.permissionKeeper.ExistGroupMemberForGroup(s.ctx, groupID))
}

// TestMigrateAccountPolicyForResources seeds the pre-migration (v1) account-policy key layout
// directly into the store and asserts MigrateAccountPolicyForResources relocates each entry's
// bytes unchanged to the v2 key layout and removes the v1 key, for all three resource types it
// handles.
func (s *TestSuite) TestMigrateAccountPolicyForResources() {
	store := s.ctx.KVStore(s.storeKey)

	type fixture struct {
		resourceType resource.ResourceType
		resourceID   math.Uint
		addr         sdk.AccAddress
		value        []byte
	}
	fixtures := []fixture{
		{resource.RESOURCE_TYPE_BUCKET, math.NewUint(rand.Uint64()), sample.RandAccAddress(), []byte("bucket-legacy-value")}, //nolint: gosec
		{resource.RESOURCE_TYPE_OBJECT, math.NewUint(rand.Uint64()), sample.RandAccAddress(), []byte("object-legacy-value")}, //nolint: gosec
		{resource.RESOURCE_TYPE_GROUP, math.NewUint(rand.Uint64()), sample.RandAccAddress(), []byte("group-legacy-value")},   //nolint: gosec
	}

	for _, f := range fixtures {
		v1Key := types.GetPolicyForAccountKey(f.resourceID, f.resourceType, f.addr, false)
		store.Set(v1Key, f.value)
	}

	s.permissionKeeper.MigrateAccountPolicyForResources(s.ctx)

	for _, f := range fixtures {
		v1Key := types.GetPolicyForAccountKey(f.resourceID, f.resourceType, f.addr, false)
		s.Require().Nil(store.Get(v1Key), "legacy v1 key must be removed after migration")

		v2Key := types.GetPolicyForAccountKey(f.resourceID, f.resourceType, f.addr, true)
		s.Require().Equal(f.value, store.Get(v2Key), "v2 key must hold the same bytes the v1 key held")
	}
}
