package types_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/testutil/storage"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/types/common"
	"github.com/mocachain/moca/v2/types/resource"
	"github.com/mocachain/moca/v2/x/permission/types"
)

func TestPolicy_BucketBasic(t *testing.T) {
	tests := []struct {
		name          string
		policyAction  types.ActionType
		policyEffect  types.Effect
		operateAction types.ActionType
		expectEffect  types.Effect
	}{
		{
			name:          "basic_update_bucket_info",
			policyAction:  types.ACTION_UPDATE_BUCKET_INFO,
			policyEffect:  types.EFFECT_ALLOW,
			operateAction: types.ACTION_UPDATE_BUCKET_INFO,
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "basic_delete_bucket",
			policyAction:  types.ACTION_DELETE_BUCKET,
			policyEffect:  types.EFFECT_ALLOW,
			operateAction: types.ACTION_DELETE_BUCKET,
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "basic_delete_bucket_deny",
			policyAction:  types.ACTION_DELETE_BUCKET,
			policyEffect:  types.EFFECT_DENY,
			operateAction: types.ACTION_DELETE_BUCKET,
			expectEffect:  types.EFFECT_DENY,
		},
		{
			name:          "basic_delete_bucket_pass",
			policyAction:  types.ACTION_UPDATE_BUCKET_INFO,
			policyEffect:  types.EFFECT_ALLOW,
			operateAction: types.ACTION_DELETE_BUCKET,
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "basic_create_object",
			policyAction:  types.ACTION_CREATE_OBJECT,
			policyEffect:  types.EFFECT_ALLOW,
			operateAction: types.ACTION_CREATE_OBJECT,
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "basic_create_object_deny",
			policyAction:  types.ACTION_CREATE_OBJECT,
			policyEffect:  types.EFFECT_DENY,
			operateAction: types.ACTION_CREATE_OBJECT,
			expectEffect:  types.EFFECT_DENY,
		},
		{
			name:          "basic_create_object_pass",
			policyAction:  types.ACTION_COPY_OBJECT,
			policyEffect:  types.EFFECT_ALLOW,
			operateAction: types.ACTION_CREATE_OBJECT,
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "basic_delete_object",
			policyAction:  types.ACTION_DELETE_OBJECT,
			policyEffect:  types.EFFECT_ALLOW,
			operateAction: types.ACTION_DELETE_OBJECT,
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "basic_delete_object_deny",
			policyAction:  types.ACTION_DELETE_OBJECT,
			policyEffect:  types.EFFECT_DENY,
			operateAction: types.ACTION_DELETE_OBJECT,
			expectEffect:  types.EFFECT_DENY,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := sample.RandAccAddress()
			policy := types.Policy{
				Principal:    types.NewPrincipalWithAccount(user),
				ResourceType: resource.RESOURCE_TYPE_BUCKET,
				ResourceId:   math.OneUint(),
				Statements: []*types.Statement{
					{
						Effect:  tt.policyEffect,
						Actions: []types.ActionType{tt.policyAction},
					},
				},
			}
			effect, _ := policy.Eval(tt.operateAction, time.Now(), nil)
			require.Equal(t, effect, tt.expectEffect)
		})
	}
}

func TestPolicy_BucketExpirationBasic(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                    string
		policyAction            types.ActionType
		policyEffect            types.Effect
		policyExpirationTime    *time.Time
		statementExpirationTime *time.Time
		operateAction           types.ActionType
		operateTime             time.Time
		expectEffect            types.Effect
	}{
		{
			name:                 "policy_expired",
			policyAction:         types.ACTION_UPDATE_BUCKET_INFO,
			policyEffect:         types.EFFECT_ALLOW,
			policyExpirationTime: &now,
			operateAction:        types.ACTION_UPDATE_BUCKET_INFO,
			expectEffect:         types.EFFECT_UNSPECIFIED,
			operateTime:          time.Now().Add(1 * time.Second),
		},
		{
			name:                 "policy_not_expired",
			policyAction:         types.ACTION_UPDATE_BUCKET_INFO,
			policyEffect:         types.EFFECT_ALLOW,
			policyExpirationTime: &now,
			operateAction:        types.ACTION_UPDATE_BUCKET_INFO,
			expectEffect:         types.EFFECT_ALLOW,
			operateTime:          time.Now().Add(-1 * time.Second),
		},
		{
			name:                    "statement_expired",
			policyAction:            types.ACTION_UPDATE_BUCKET_INFO,
			policyEffect:            types.EFFECT_ALLOW,
			statementExpirationTime: &now,
			operateAction:           types.ACTION_UPDATE_BUCKET_INFO,
			expectEffect:            types.EFFECT_UNSPECIFIED,
			operateTime:             time.Now().Add(1 * time.Second),
		},
		{
			name:                    "statement_not_expired",
			policyAction:            types.ACTION_UPDATE_BUCKET_INFO,
			policyEffect:            types.EFFECT_ALLOW,
			statementExpirationTime: &now,
			operateAction:           types.ACTION_UPDATE_BUCKET_INFO,
			expectEffect:            types.EFFECT_ALLOW,
			operateTime:             time.Now().Add(-1 * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := sample.RandAccAddress()
			policy := types.Policy{
				Principal:      types.NewPrincipalWithAccount(user),
				ResourceType:   resource.RESOURCE_TYPE_BUCKET,
				ResourceId:     math.OneUint(),
				ExpirationTime: tt.policyExpirationTime,
				Statements: []*types.Statement{
					{
						Effect:         tt.policyEffect,
						Actions:        []types.ActionType{tt.policyAction},
						ExpirationTime: tt.statementExpirationTime,
					},
				},
			}
			effect, _ := policy.Eval(tt.operateAction, tt.operateTime, nil)
			require.Equal(t, effect, tt.expectEffect)
		})
	}
}

func TestPolicy_CreateObjectLimitSize(t *testing.T) {
	tests := []struct {
		name         string
		limitSize    uint64
		wantedSize   uint64
		expectEffect types.Effect
	}{
		{
			name:         "limit_size_not_exceed",
			limitSize:    2 * 1024,
			wantedSize:   1 * 1024,
			expectEffect: types.EFFECT_ALLOW,
		},
		{
			name:         "limit_size_exceed",
			limitSize:    2 * 1024,
			wantedSize:   3 * 1024,
			expectEffect: types.EFFECT_DENY,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := sample.RandAccAddress()
			policy := types.Policy{
				Principal:    types.NewPrincipalWithAccount(user),
				ResourceType: resource.RESOURCE_TYPE_BUCKET,
				ResourceId:   math.OneUint(),
				Statements: []*types.Statement{
					{
						Effect:    types.EFFECT_ALLOW,
						Actions:   []types.ActionType{types.ACTION_CREATE_OBJECT},
						LimitSize: &common.UInt64Value{Value: tt.limitSize},
					},
				},
			}
			wantedSize := tt.wantedSize
			effect, p := policy.Eval(types.ACTION_CREATE_OBJECT, time.Now(), &types.VerifyOptions{WantedSize: &wantedSize})
			if effect == types.EFFECT_ALLOW && tt.limitSize > tt.wantedSize {
				require.Equal(t, p.Statements[0].LimitSize.GetValue(), tt.limitSize-tt.wantedSize)
			}
			require.Equal(t, effect, tt.expectEffect)
		})
	}
}

func TestPolicy_SubResource(t *testing.T) {
	bucketName := storage.GenRandomBucketName()
	tests := []struct {
		name            string
		policyAction    types.ActionType
		policyEffect    types.Effect
		policyResource  string
		operateAction   types.ActionType
		operateResource string
		expectEffect    types.Effect
	}{
		{
			name:            "policy_resource_matched_allow",
			policyAction:    types.ACTION_GET_OBJECT,
			policyEffect:    types.EFFECT_ALLOW,
			policyResource:  types2.NewObjectGRN(bucketName, "*").String(),
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: types2.NewObjectGRN(bucketName, "xxxx").String(),
			expectEffect:    types.EFFECT_ALLOW,
		},
		{
			name:            "policy_resource_matched_deny",
			policyAction:    types.ACTION_GET_OBJECT,
			policyEffect:    types.EFFECT_DENY,
			policyResource:  types2.NewObjectGRN(bucketName, "*").String(),
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: types2.NewObjectGRN(bucketName, "xxxx").String(),
			expectEffect:    types.EFFECT_DENY,
		},
		{
			name:            "policy_resource_not_matched",
			policyAction:    types.ACTION_GET_OBJECT,
			policyEffect:    types.EFFECT_ALLOW,
			policyResource:  types2.NewObjectGRN(bucketName, "xxx").String(),
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: types2.NewObjectGRN(bucketName, "1111").String(),
			expectEffect:    types.EFFECT_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := sample.RandAccAddress()
			policy := types.Policy{
				Principal:    types.NewPrincipalWithAccount(user),
				ResourceType: resource.RESOURCE_TYPE_BUCKET,
				ResourceId:   math.OneUint(),
				Statements: []*types.Statement{
					{
						Effect:    tt.policyEffect,
						Actions:   []types.ActionType{tt.policyAction},
						Resources: []string{tt.policyResource},
					},
				},
			}
			effect, _ := policy.Eval(tt.operateAction, time.Now(), &types.VerifyOptions{Resource: tt.operateResource})
			require.Equal(t, effect, tt.expectEffect)
		})
	}
}

func TestPolicy_SubResourceWildcardMatching(t *testing.T) {
	bucketName := storage.GenRandomBucketName()
	tests := []struct {
		name          string
		policyEffect  types.Effect
		policyObject  string
		operateObject string
		expectEffect  types.Effect
	}{
		{
			name:          "exact_name_does_not_reach_longer_sibling",
			policyObject:  "report",
			operateObject: "report2",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "exact_name_does_not_reach_nested_path",
			policyObject:  "report",
			operateObject: "report/q4-internal.pdf",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "dot_is_literal_not_any_character",
			policyObject:  "invoice.pdf",
			operateObject: "invoiceXpdf",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "dot_still_matches_itself",
			policyObject:  "invoice.pdf",
			operateObject: "invoice.pdf",
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "alternation_is_literal",
			policyObject:  "(draft|final)",
			operateObject: "draft",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "alternation_still_matches_itself",
			policyObject:  "(draft|final)",
			operateObject: "(draft|final)",
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "character_class_is_literal",
			policyObject:  "[a-z]+",
			operateObject: "zzz",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "prefix_wildcard_matches_documented_prefix",
			policyObject:  "test_*",
			operateObject: "test_abc",
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "prefix_wildcard_does_not_drop_its_last_literal",
			policyObject:  "test_*",
			operateObject: "testify.pdf",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "prefix_wildcard_does_not_match_other_prefix",
			policyObject:  "test_*",
			operateObject: "prod_abc",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "bucket_wide_wildcard_spans_nested_paths",
			policyObject:  "*",
			operateObject: "deep/nested/file.txt",
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "bucket_wide_wildcard_deny_spans_newline_in_name",
			policyEffect:  types.EFFECT_DENY,
			policyObject:  "*",
			operateObject: "note\nsecret",
			expectEffect:  types.EFFECT_DENY,
		},
		{
			name:          "question_mark_matches_exactly_one_character",
			policyObject:  "a?c",
			operateObject: "abc",
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "question_mark_does_not_match_two_characters",
			policyObject:  "a?c",
			operateObject: "abbc",
			expectEffect:  types.EFFECT_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect := tt.policyEffect
			if effect == types.EFFECT_UNSPECIFIED {
				effect = types.EFFECT_ALLOW
			}
			policy := types.Policy{
				Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
				ResourceType: resource.RESOURCE_TYPE_BUCKET,
				ResourceId:   math.OneUint(),
				Statements: []*types.Statement{
					{
						Effect:    effect,
						Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
						Resources: []string{types2.NewObjectGRN(bucketName, tt.policyObject).String()},
					},
				},
			}
			got, _ := policy.Eval(types.ACTION_GET_OBJECT, time.Now(),
				&types.VerifyOptions{Resource: types2.NewObjectGRN(bucketName, tt.operateObject).String()})
			require.Equal(t, tt.expectEffect, got)
		})
	}
}

// An object name only has to be valid UTF-8, so it can carry regexp metacharacters that do not
// form a valid expression. Such a resource must be matched literally rather than compiled.
func TestPolicy_SubResourceUncompilableName(t *testing.T) {
	bucketName := storage.GenRandomBucketName()
	objectName := "quarter(1"
	policy := types.Policy{
		Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   math.OneUint(),
		Statements: []*types.Statement{
			{
				Effect:    types.EFFECT_ALLOW,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{types2.NewObjectGRN(bucketName, objectName).String()},
			},
		},
	}

	require.NotPanics(t, func() {
		effect, _ := policy.Eval(types.ACTION_GET_OBJECT, time.Now(),
			&types.VerifyOptions{Resource: types2.NewObjectGRN(bucketName, objectName).String()})
		require.Equal(t, types.EFFECT_ALLOW, effect)
	})

	// The statement is storable today, so validation must not reject the name either.
	require.NoError(t, policy.Statements[0].ValidateBasic(resource.RESOURCE_TYPE_BUCKET))
	require.NoError(t, policy.Statements[0].ValidateRuntime(sdk.Context{}, resource.RESOURCE_TYPE_BUCKET))
}

func TestPolicy_StatementWithoutResources(t *testing.T) {
	bucketName := storage.GenRandomBucketName()
	object := types2.NewObjectGRN(bucketName, "xxx").String()
	tests := []struct {
		name            string
		policyAction    types.ActionType
		policyEffect    types.Effect
		policyResources []string
		operateAction   types.ActionType
		operateResource string
		expectEffect    types.Effect
	}{
		{
			name:            "deny_all_reaches_object",
			policyAction:    types.ACTION_TYPE_ALL,
			policyEffect:    types.EFFECT_DENY,
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: object,
			expectEffect:    types.EFFECT_DENY,
		},
		{
			name:            "deny_object_action_reaches_object",
			policyAction:    types.ACTION_GET_OBJECT,
			policyEffect:    types.EFFECT_DENY,
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: object,
			expectEffect:    types.EFFECT_DENY,
		},
		{
			name:            "allow_all_does_not_reach_object",
			policyAction:    types.ACTION_TYPE_ALL,
			policyEffect:    types.EFFECT_ALLOW,
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: object,
			expectEffect:    types.EFFECT_UNSPECIFIED,
		},
		{
			name:            "allow_object_action_does_not_reach_object",
			policyAction:    types.ACTION_GET_OBJECT,
			policyEffect:    types.EFFECT_ALLOW,
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: object,
			expectEffect:    types.EFFECT_UNSPECIFIED,
		},
		{
			name:          "bucket_scoped_allow_unchanged",
			policyAction:  types.ACTION_TYPE_ALL,
			policyEffect:  types.EFFECT_ALLOW,
			operateAction: types.ACTION_UPDATE_BUCKET_INFO,
			expectEffect:  types.EFFECT_ALLOW,
		},
		{
			name:          "bucket_scoped_deny_unchanged",
			policyAction:  types.ACTION_TYPE_ALL,
			policyEffect:  types.EFFECT_DENY,
			operateAction: types.ACTION_UPDATE_BUCKET_INFO,
			expectEffect:  types.EFFECT_DENY,
		},
		{
			name:            "empty_resources_deny_reaches_object",
			policyAction:    types.ACTION_TYPE_ALL,
			policyEffect:    types.EFFECT_DENY,
			policyResources: []string{},
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: object,
			expectEffect:    types.EFFECT_DENY,
		},
		{
			name:            "empty_resources_allow_does_not_reach_object",
			policyAction:    types.ACTION_TYPE_ALL,
			policyEffect:    types.EFFECT_ALLOW,
			policyResources: []string{},
			operateAction:   types.ACTION_GET_OBJECT,
			operateResource: object,
			expectEffect:    types.EFFECT_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := types.Policy{
				Principal:    types.NewPrincipalWithAccount(sample.RandAccAddress()),
				ResourceType: resource.RESOURCE_TYPE_BUCKET,
				ResourceId:   math.OneUint(),
				Statements: []*types.Statement{
					{
						Effect:    tt.policyEffect,
						Actions:   []types.ActionType{tt.policyAction},
						Resources: tt.policyResources,
					},
				},
			}
			effect, _ := policy.Eval(tt.operateAction, time.Now(), &types.VerifyOptions{Resource: tt.operateResource})
			require.Equal(t, tt.expectEffect, effect)
		})
	}
}

// TestPolicy_EvalDoesNotPanicOnUnparsableResourceRegex is a regression test
// Write-time validation (MsgPutPolicy.ValidateRuntime,
// now invoked by msg_server.PutPolicy) stops *new* policies from being stored
// with a Resources entry that isn't a valid regexp, but it does nothing for
// policies that were already written to state before that fix shipped. Eval
// used regexp.MustCompile on the stored value directly, which panics on an
// invalid pattern instead of returning an error; the `if reg == nil` guard
// right after it is unreachable because MustCompile never returns nil, it
// panics. This is defence in depth for that already-stored state: Eval must
// treat an unparsable pattern as "does not match" rather than crash the
// caller (a message handler or the unauthenticated VerifyPermission query).
func TestPolicy_EvalDoesNotPanicOnUnparsableResourceRegex(t *testing.T) {
	user := sample.RandAccAddress()
	policy := types.Policy{
		Principal:    types.NewPrincipalWithAccount(user),
		ResourceType: resource.RESOURCE_TYPE_BUCKET,
		ResourceId:   math.OneUint(),
		Statements: []*types.Statement{
			{
				Effect:  types.EFFECT_ALLOW,
				Actions: []types.ActionType{types.ACTION_GET_OBJECT},
				// Unterminated character class: not a valid regexp, but this is
				// exactly the kind of value pre-fix state could contain (see
				// TestMsgPutPolicy_ValidateRuntime in message_test.go for a
				// value that clears ValidateBasic yet fails regexp.Compile).
				Resources: []string{"grn:o::somebucket/obj["},
			},
		},
	}

	var effect types.Effect
	require.NotPanics(t, func() {
		effect, _ = policy.Eval(types.ACTION_GET_OBJECT, time.Now(), &types.VerifyOptions{
			Resource: types2.NewObjectGRN("somebucket", "obj").String(),
		})
	})
	require.Equal(t, types.EFFECT_UNSPECIFIED, effect, "an unparsable pattern must not match, not panic")
}

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
