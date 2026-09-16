package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  Params
		wantErr bool
	}{
		{
			name:   "defaults",
			params: DefaultParams(),
		},
		{
			name:    "zero maximum statements num",
			params:  NewParams(0, DefaultMaxPolicyGroupNum, DefaultMaximumRemoveExpiredPoliciesIteration),
			wantErr: true,
		},
		{
			name:    "zero maximum group num",
			params:  NewParams(DefaultMaxStatementsNum, 0, DefaultMaximumRemoveExpiredPoliciesIteration),
			wantErr: true,
		},
		{
			name:    "zero maximum remove expired policies iteration",
			params:  NewParams(DefaultMaxStatementsNum, DefaultMaxPolicyGroupNum, 0),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidateMaximumStatementsNum covers validateMaximumStatementsNum directly:
// Params.Validate calls validateMaximumGroupNum for this field instead (pre-existing,
// functionally harmless since both apply the same v==0 check), which leaves this
// function otherwise uncalled anywhere in the module.
func TestValidateMaximumStatementsNum(t *testing.T) {
	require.NoError(t, validateMaximumStatementsNum(uint64(1)))
	require.Error(t, validateMaximumStatementsNum(uint64(0)))
	require.Error(t, validateMaximumStatementsNum("not-a-uint64"))
}

func TestValidateMaximumGroupNum(t *testing.T) {
	require.NoError(t, validateMaximumGroupNum(uint64(1)))
	require.Error(t, validateMaximumGroupNum(uint64(0)))
	require.Error(t, validateMaximumGroupNum("not-a-uint64"))
}

func TestValidateMaximumRemoveExpiredPoliciesIteration(t *testing.T) {
	require.NoError(t, validateMaximumRemoveExpiredPoliciesIteration(uint64(1)))
	require.Error(t, validateMaximumRemoveExpiredPoliciesIteration(uint64(0)))
	require.Error(t, validateMaximumRemoveExpiredPoliciesIteration("not-a-uint64"))
}
