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

// TestParams_Validate_FieldErrors covers the remaining reachable branches of
// Params.Validate(): one invalid value per field (all still correctly typed --
// the type-assertion branches are only reachable from inside the package, see
// params_internal_test.go), plus the cross-field reserve/forced-settle check.
func TestParams_Validate_FieldErrors(t *testing.T) {
	for _, tc := range []struct {
		desc      string
		mutate    func(*types.Params)
		errSubstr string
	}{
		{
			desc:   "valid default",
			mutate: func(*types.Params) {},
		},
		{
			desc:      "zero reserve time",
			mutate:    func(p *types.Params) { p.VersionedParams.ReserveTime = 0 },
			errSubstr: "reserve time must be positive",
		},
		{
			desc:      "negative validator tax rate",
			mutate:    func(p *types.Params) { p.VersionedParams.ValidatorTaxRate = math.LegacyNewDec(-1) },
			errSubstr: "validator tax ratio should be between 0 and 1",
		},
		{
			desc:      "validator tax rate above one",
			mutate:    func(p *types.Params) { p.VersionedParams.ValidatorTaxRate = math.LegacyNewDec(2) },
			errSubstr: "validator tax ratio should be between 0 and 1",
		},
		{
			desc:      "zero forced settle time",
			mutate:    func(p *types.Params) { p.ForcedSettleTime = 0 },
			errSubstr: "forced settle time must be positive",
		},
		{
			desc:      "zero payment account count limit",
			mutate:    func(p *types.Params) { p.PaymentAccountCountLimit = 0 },
			errSubstr: "payment account count limit must be positive",
		},
		{
			desc:      "zero max auto settle flow count",
			mutate:    func(p *types.Params) { p.MaxAutoSettleFlowCount = 0 },
			errSubstr: "max force settle flow count must be positive",
		},
		{
			desc:      "zero max auto resume flow count",
			mutate:    func(p *types.Params) { p.MaxAutoResumeFlowCount = 0 },
			errSubstr: "max auto resume flow count must be positive",
		},
		{
			desc:      "blank fee denom",
			mutate:    func(p *types.Params) { p.FeeDenom = "   " },
			errSubstr: "fee denom cannt be blank",
		},
		{
			desc: "reserve time not greater than forced settle time",
			mutate: func(p *types.Params) {
				p.VersionedParams.ReserveTime = p.ForcedSettleTime
			},
			errSubstr: "reserve time must be greater than force settle time",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			params := types.DefaultParams()
			tc.mutate(&params)

			err := params.Validate()
			if tc.errSubstr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.errSubstr)
		})
	}
}
