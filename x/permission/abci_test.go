package permission_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/x/permission"
	"github.com/mocachain/moca/v2/x/permission/types"
)

// TestEndBlocker_RemovesExpiredPolicy exercises permission.EndBlocker end to
// end: a policy whose expiration time has already passed must be pruned by
// the time the block ends.
func TestEndBlocker_RemovesExpiredPolicy(t *testing.T) {
	k, ctx := makeKeeper(t)
	// Without params, MaximumRemoveExpiredPoliciesIteration is 0 and RemoveExpiredPolicies
	// breaks out of its loop before ever visiting an entry.
	require.NoError(t, k.SetParams(ctx, types.DefaultParams()))

	now := time.Now()
	ctx = ctx.WithBlockTime(now)
	past := now.Add(-time.Hour)

	policy := types.Policy{
		Principal: &types.Principal{
			Type:  types.PRINCIPAL_TYPE_GNFD_ACCOUNT,
			Value: sample.RandAccAddressHex(),
		},
		ResourceType:   resource.RESOURCE_TYPE_BUCKET,
		ResourceId:     math.NewUint(1),
		ExpirationTime: &past,
	}
	policyID, err := k.PutPolicy(ctx, &policy)
	require.NoError(t, err)

	_, found := k.GetPolicyByID(ctx, policyID)
	require.True(t, found, "policy must be stored before EndBlocker runs")

	require.NoError(t, permission.EndBlocker(ctx, *k))

	_, found = k.GetPolicyByID(ctx, policyID)
	require.False(t, found, "EndBlocker must prune the expired policy")
}
