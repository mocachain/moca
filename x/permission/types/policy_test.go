package types_test

import (
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/testutil/sample"
	"github.com/evmos/evmos/v12/testutil/storage"
	types2 "github.com/evmos/evmos/v12/types"
	"github.com/evmos/evmos/v12/types/common"
	"github.com/evmos/evmos/v12/types/resource"
	"github.com/evmos/evmos/v12/x/permission/types"
)

const (
	objInvoicePDF  = "invoice.pdf"
	objAlternation = "(draft|final)"
	objTestPrefix  = "test_*"
	escalationBkt  = "escalation-bucket"
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

// TestStatementValidateRuntime_EmptyResourcesSlice pins that an empty Resources
// slice is treated as absent on object- and group-scoped statements.
//
// The check tested nil rather than length, and the two are not the same thing
// once a statement has been through ABI decoding: go-ethereum decodes a
// zero-length dynamic array to an empty non-nil slice, so every object- and
// group-scoped PutPolicy submitted through the EVM storage precompile carried
// Resources == []string{} and was rejected. The native tx path was unaffected,
// because protobuf decodes an absent repeated field to nil.
//
// A populated Resources on a non-bucket statement must still be rejected --
// that is what the check is for.
func TestStatementValidateRuntime_EmptyResourcesSlice(t *testing.T) {
	for _, resType := range []resource.ResourceType{
		resource.RESOURCE_TYPE_OBJECT,
		resource.RESOURCE_TYPE_GROUP,
	} {
		t.Run(resType.String()+"/empty slice accepted", func(t *testing.T) {
			s := &types.Statement{
				Effect:    types.EFFECT_ALLOW,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{},
			}
			require.NoError(t, s.ValidateRuntime(sdk.Context{}, resType))
		})

		t.Run(resType.String()+"/nil accepted", func(t *testing.T) {
			s := &types.Statement{
				Effect:  types.EFFECT_ALLOW,
				Actions: []types.ActionType{types.ACTION_GET_OBJECT},
			}
			require.NoError(t, s.ValidateRuntime(sdk.Context{}, resType))
		})

		t.Run(resType.String()+"/populated still rejected", func(t *testing.T) {
			s := &types.Statement{
				Effect:    types.EFFECT_ALLOW,
				Actions:   []types.ActionType{types.ACTION_GET_OBJECT},
				Resources: []string{"grn:o::bucket/obj"},
			}
			require.ErrorIs(t, s.ValidateRuntime(sdk.Context{}, resType), types.ErrInvalidStatement)
		})
	}
}
