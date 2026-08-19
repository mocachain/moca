package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/payment/types"
)

// TestValidateWithdrawTimeLockThreshold_RejectsUnset covers the threshold being
// unset. Withdraw dereferences the parameter on every call, so leaving it out of
// a parameter change does not remove the limit -- it makes every withdrawal fail.
func TestValidateWithdrawTimeLockThreshold_RejectsUnset(t *testing.T) {
	params := types.DefaultParams()
	params.WithdrawTimeLockThreshold = nil

	err := params.Validate()
	require.Error(t, err, "an unset threshold must be rejected, not stored")
	require.Contains(t, err.Error(), "withdraw time lock threshold must be set")
}

// TestValidateWithdrawTimeLockThreshold_AcceptsSet keeps the rejection scoped to
// the unset case: the default and an explicit positive value are still valid.
func TestValidateWithdrawTimeLockThreshold_AcceptsSet(t *testing.T) {
	require.NoError(t, types.DefaultParams().Validate())

	positive := math.NewInt(1)
	params := types.DefaultParams()
	params.WithdrawTimeLockThreshold = &positive
	require.NoError(t, params.Validate())

	// A non-positive value is still rejected by the existing check.
	zero := math.ZeroInt()
	params.WithdrawTimeLockThreshold = &zero
	require.ErrorContains(t, params.Validate(), "should be positive")
}
