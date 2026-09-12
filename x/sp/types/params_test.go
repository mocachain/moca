package types_test

import (
	stdmath "math"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/sp/types"
)

// TestValidateParams_UpdateGlobalPriceIntervalBound covers the bound on the
// interval. x/sp/abci.go compares an elapsed duration in seconds against this
// parameter as an int64, so a value past that range wraps negative and makes the
// comparison trivially true, updating the global price on every block instead of
// on the configured interval.
func TestValidateParams_UpdateGlobalPriceIntervalBound(t *testing.T) {
	tests := []struct {
		name     string
		interval uint64
		wantErr  bool
	}{
		{"default", types.DefaultUpdateGlobalPriceInterval, false},
		{"a week", 7 * 24 * 60 * 60, false},
		{"largest representable as int64", stdmath.MaxInt64, false},
		{"one past int64", uint64(stdmath.MaxInt64) + 1, true},
		{"max uint64", stdmath.MaxUint64, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			params.UpdateGlobalPriceInterval = tc.interval

			err := params.Validate()
			if tc.wantErr {
				require.Error(t, err, "an interval that cannot be compared as an int64 must be rejected")
				require.Contains(t, err.Error(), "update global price interval too large")
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestParams_Validate covers the value-based validation branches reachable
// through Params.Validate (each validateXxx helper's happy path plus its one
// reachable failure mode). The interface{} type-assertion guard inside each
// validateXxx helper cannot be triggered from here: Validate always calls the
// helpers with the statically-typed struct field, so the "!ok" branch (and,
// for validateUpdatePriceDisallowedDays specifically, its only possible error
// return) is unreachable dead code left over from the old x/params-subspace
// signature — see the PR body for the accounting.
func TestParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*types.Params)
		wantErr string
	}{
		{"default params are valid", func(p *types.Params) {}, ""},
		{"blank deposit denom", func(p *types.Params) { p.DepositDenom = "" }, "deposit denom cannot be blank"},
		{"malformed deposit denom", func(p *types.Params) { p.DepositDenom = "1abc" }, "invalid denom"},
		{"nil min deposit", func(p *types.Params) { p.MinDeposit = math.Int{} }, "minimum deposit amount cannot be nil"},
		{"negative min deposit", func(p *types.Params) { p.MinDeposit = math.NewInt(-1) }, "cannot be lower than 0"},
		{"zero secondary sp store price ratio", func(p *types.Params) { p.SecondarySpStorePriceRatio = math.LegacyZeroDec() }, "invalid secondary sp store price ratio"},
		{"secondary sp store price ratio above one", func(p *types.Params) { p.SecondarySpStorePriceRatio = math.LegacyNewDec(2) }, "invalid secondary sp store price ratio"},
		{"zero historical blocks for maintenance records", func(p *types.Params) { p.NumOfHistoricalBlocksForMaintenanceRecords = 0 }, "HistoricalBlocksForMaintenanceRecords cannot be zero"},
		{"zero maintenance duration quota", func(p *types.Params) { p.MaintenanceDurationQuota = 0 }, "MaintenanceDurationQuota cannot be zero"},
		{"zero lockup blocks for maintenance", func(p *types.Params) { p.NumOfLockupBlocksForMaintenance = 0 }, "LockUpBlocksForMaintenance cannot be zero"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			tc.mutate(&params)

			err := params.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestParams_String(t *testing.T) {
	out := types.DefaultParams().String()
	require.Contains(t, out, types.DefaultDepositDenom)
}
