package types_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/x/permission/types"
)

// TestResources_AnchoredDenyNoLongerReachesLongerNames pins the widening
// direction of the Resources semantics change, the one an operator has to know
// about before upgrading. An owner who wrote a Deny naming an exact object used
// to get every name that merely starts with it, because matching was
// unanchored. Anchored matching drops that reach, so a bucket-wide Allow in the
// same policy wins for those names: EFFECT_DENY becomes EFFECT_ALLOW.
func TestResources_AnchoredDenyNoLongerReachesLongerNames(t *testing.T) {
	bucket := "escalation-bucket"
	policy := types.Policy{
		Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   math.OneUint(),
		Statements: []*types.Statement{
			{
				Effect:    types.EFFECT_ALLOW,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{types2.NewObjectGRN(bucket, "*").String()},
			},
			{
				Effect:    types.EFFECT_DENY,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{types2.NewObjectGRN(bucket, "secret").String()},
			},
		},
	}

	// The exact name the Deny spells out is still denied under both semantics.
	got, _ := policy.Eval(types.ACTION_GET_OBJECT, time.Now(),
		&types.VerifyOptions{Resource: types2.NewObjectGRN(bucket, "secret").String()})
	require.Equal(t, types.EFFECT_DENY, got, "the exact denied name must stay denied")

	// A name the old unanchored match also reached: was EFFECT_DENY before this
	// change, is EFFECT_ALLOW after it. Owners relying on a bare name as a prefix
	// Deny must rewrite it as "secret*" before the upgrade.
	got, _ = policy.Eval(types.ACTION_GET_OBJECT, time.Now(),
		&types.VerifyOptions{Resource: types2.NewObjectGRN(bucket, "secret_v2").String()})
	require.Equal(t, types.EFFECT_ALLOW, got)
}

// TestResources_RegexpDenyBecomesLiteral pins the same direction for an owner
// who wrote an actual regular expression, which is what Eval compiled until now.
// The expression becomes a literal name, so it stops denying anything real.
func TestResources_RegexpDenyBecomesLiteral(t *testing.T) {
	bucket := "escalation-bucket"
	policy := types.Policy{
		Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   math.OneUint(),
		Statements: []*types.Statement{
			{
				Effect:    types.EFFECT_ALLOW,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{types2.NewObjectGRN(bucket, "*").String()},
			},
			{
				Effect:    types.EFFECT_DENY,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{types2.NewObjectGRN(bucket, "(secret|private).*").String()},
			},
		},
	}
	got, _ := policy.Eval(types.ACTION_GET_OBJECT, time.Now(),
		&types.VerifyOptions{Resource: types2.NewObjectGRN(bucket, "private-keys.txt").String()})
	require.Equal(t, types.EFFECT_ALLOW, got)
}

// TestResources_WildcardCharInAnObjectNameWidensAnAllow pins that an Allow
// naming an object whose real name contains '?' or '*' now also reaches the
// siblings that character spans, because the pattern syntax has no escape.
func TestResources_WildcardCharInAnObjectNameWidensAnAllow(t *testing.T) {
	bucket := "escalation-bucket"
	policy := types.Policy{
		Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   math.OneUint(),
		Statements: []*types.Statement{
			{
				Effect:  types.EFFECT_ALLOW,
				Actions: []types.ActionType{types.ACTION_GET_OBJECT},
				// The owner means the single object literally called "q?.csv".
				Resources: []string{types2.NewObjectGRN(bucket, "q?.csv").String()},
			},
		},
	}
	got, _ := policy.Eval(types.ACTION_GET_OBJECT, time.Now(),
		&types.VerifyOptions{Resource: types2.NewObjectGRN(bucket, "qX.csv").String()})
	require.Equal(t, types.EFFECT_ALLOW, got)
}

// TestResources_UncompilablePatternDoesNotMatch keeps the fail-safe in Eval
// covered. Escaping means almost every byte string compiles, so the one input
// that still cannot is a Resources entry holding invalid UTF-8 — which gogoproto
// does not reject on the wire. Such an entry must be skipped, not crash Eval.
func TestResources_UncompilablePatternDoesNotMatch(t *testing.T) {
	bucket := "utf8-bucket"
	policy := types.Policy{
		Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   math.OneUint(),
		Statements: []*types.Statement{
			{
				Effect:    types.EFFECT_ALLOW,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{"grn:o::" + bucket + "/obj\xff\xfe"},
			},
		},
	}
	require.NotPanics(t, func() {
		got, _ := policy.Eval(types.ACTION_GET_OBJECT, time.Now(),
			&types.VerifyOptions{Resource: types2.NewObjectGRN(bucket, "obj\xff\xfe").String()})
		require.Equal(t, types.EFFECT_UNSPECIFIED, got)
	})

	// The same entry must be refused at write time rather than stored inert.
	require.Error(t, policy.Statements[0].ValidateRuntime(sdk.Context{}, resource.RESOURCE_TYPE_BUCKET))
}
