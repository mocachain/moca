package keeper_test

import (
	"math/rand"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/permission/types"
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

// TestPutPolicy_MaximumStatementsNum is a regression test for MOCA-965
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

	max := s.permissionKeeper.MaximumStatementsNum(s.ctx)
	s.Require().Equal(types.DefaultMaxStatementsNum, max, "test assumes the default cap is in effect")

	// Exactly at the cap must still be accepted.
	atCap := types.Policy{
		Principal: &types.Principal{
			Type:  types.PRINCIPAL_TYPE_GNFD_ACCOUNT,
			Value: sample.RandAccAddressHex(),
		},
		ResourceType: 1,
		ResourceId:   math.NewUint(rand.Uint64()), //nolint: gosec
		Statements:   makeStatements(int(max)),
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
		Statements:   makeStatements(int(max) + 1),
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
