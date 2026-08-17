package types_test

import (
	stdmath "math"
	"testing"

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
