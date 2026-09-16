package types

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestDepositDenom(t *testing.T) {
	tests := []struct {
		name  string
		denom interface{}
		err   string
	}{
		{
			name:  caseValid,
			denom: "denom",
		},
		{
			name:  "invalid type",
			denom: 1,
			err:   "invalid parameter type",
		},
		{
			name:  "empty",
			denom: " ",
			err:   "deposit denom cannot be blank",
		},
		{
			name:  "invalid denom",
			denom: "%",
			err:   "invalid denom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDepositDenom(tt.denom)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGVGStakingPerBytes(t *testing.T) {
	var nilInt math.Int
	tests := []struct {
		name  string
		ratio interface{}
		err   string
	}{
		{
			name:  caseValid,
			ratio: math.NewInt(1),
		},
		{
			name:  "invalid type",
			ratio: 1,
			err:   "invalid parameter type",
		},
		{
			name:  "invalid value",
			ratio: nilInt,
			err:   "invalid value for GVG staking per bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGVGStakingPerBytes(tt.ratio)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMaxGlobalVirtualGroupNumPerFamily(t *testing.T) {
	tests := []struct {
		name   string
		number interface{}
		err    string
	}{
		{
			name:   caseValid,
			number: uint32(1),
		},
		{
			name:   "invalid type",
			number: 1,
			err:    "invalid parameter type",
		},
		{
			name:   "invalid size",
			number: uint32(0),
			err:    "max GVG per family must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxGlobalVirtualGroupNumPerFamily(tt.number)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMaxStoreSizePerFamily(t *testing.T) {
	tests := []struct {
		name string
		size interface{}
		err  string
	}{
		{
			name: caseValid,
			size: uint64(1),
		},
		{
			name: "invalid type",
			size: 1,
			err:  "invalid parameter type",
		},
		{
			name: "invalid size",
			size: uint64(0),
			err:  "max store size per GVG family must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxStoreSizePerFamily(tt.size)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateParams(t *testing.T) {
	tests := []struct {
		name    string
		params  Params
		wantErr string
	}{
		{
			name:   caseValid,
			params: DefaultParams(),
		},
		{
			name: "invalid deposit denom",
			params: NewParams("%", DefaultGVGStakingPerBytes, DefaultMaxGlobalVirtualGroupNumPerFamily,
				DefaultMaxStoreSizePerFamily, DefaultSwapInValidityPeriod, DefaultSPConcurrentExitNum),
			wantErr: "invalid denom",
		},
		{
			name: "invalid gvg staking per bytes",
			params: NewParams(DefaultDepositDenom, math.NewInt(0), DefaultMaxGlobalVirtualGroupNumPerFamily,
				DefaultMaxStoreSizePerFamily, DefaultSwapInValidityPeriod, DefaultSPConcurrentExitNum),
			wantErr: "invalid value for GVG staking per bytes",
		},
		{
			name: "invalid max gvg per family",
			params: NewParams(DefaultDepositDenom, DefaultGVGStakingPerBytes, 0,
				DefaultMaxStoreSizePerFamily, DefaultSwapInValidityPeriod, DefaultSPConcurrentExitNum),
			wantErr: "max GVG per family must be positive",
		},
		{
			name: "invalid max store size per family",
			params: NewParams(DefaultDepositDenom, DefaultGVGStakingPerBytes, DefaultMaxGlobalVirtualGroupNumPerFamily,
				0, DefaultSwapInValidityPeriod, DefaultSPConcurrentExitNum),
			wantErr: "max store size per GVG family must be positive",
		},
		{
			name: "invalid swap in validity period",
			params: NewParams(DefaultDepositDenom, DefaultGVGStakingPerBytes, DefaultMaxGlobalVirtualGroupNumPerFamily,
				DefaultMaxStoreSizePerFamily, math.NewInt(-1), DefaultSPConcurrentExitNum),
			wantErr: "swapIn info validity period must be positive",
		},
		{
			name: "invalid sp concurrent exit num",
			params: NewParams(DefaultDepositDenom, DefaultGVGStakingPerBytes, DefaultMaxGlobalVirtualGroupNumPerFamily,
				DefaultMaxStoreSizePerFamily, DefaultSwapInValidityPeriod, math.NewInt(-1)),
			wantErr: "number of sp concurrent exit must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParamsString(t *testing.T) {
	out := DefaultParams().String()
	require.Contains(t, out, DefaultDepositDenom)
}

// TestSPConcurrentExitNumBound covers the bound on the concurrent-exit count.
// The parameter is read back as a uint32 and compared against a uint32 count, so
// a value past that range wraps -- at a multiple of 2^32 it wraps to zero, and
// the comparison then refuses every storage-provider exit rather than allowing
// the large number the parameter names.
func TestSPConcurrentExitNumBound(t *testing.T) {
	maxUint32 := math.NewIntFromUint64(4294967295)
	tests := []struct {
		name    string
		value   math.Int
		wantErr bool
	}{
		{"default", math.NewInt(1), false},
		{"largest representable as uint32", maxUint32, false},
		{"one past uint32", maxUint32.AddRaw(1), true},
		{"wraps to zero", math.NewIntFromUint64(1 << 32), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.value
			params := DefaultParams()
			params.SpConcurrentExitNum = &v

			err := params.Validate()
			if tc.wantErr {
				require.Error(t, err, "a count that cannot be held as a uint32 must be rejected")
				require.Contains(t, err.Error(), "number of sp concurrent exit too large")
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestSwapInValidityPeriod calls the unexported validator directly (rather
// than through Params.Validate) because the "invalid parameter type" branch
// can only be reached with a non-*math.Int value, which the struct field
// never holds in practice.
func TestSwapInValidityPeriod(t *testing.T) {
	var zeroInt math.Int
	valid := math.NewInt(1)
	invalid := math.NewInt(-1)

	tests := []struct {
		name   string
		period interface{}
		err    string
	}{
		{
			name:   caseValid,
			period: &valid,
		},
		{
			name:   "invalid type",
			period: 1,
			err:    "invalid parameter type",
		},
		{
			name:   "nil pointer is allowed",
			period: (*math.Int)(nil),
		},
		{
			name:   "zero-value Int is allowed",
			period: &zeroInt,
		},
		{
			name:   "negative",
			period: &invalid,
			err:    "swapIn info validity period must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSwapInValidityPeriod(tt.period)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestSPConcurrentExitNum covers the same type-assertion and nil-skip
// branches as TestSwapInValidityPeriod; the too-large-for-uint32 branch is
// covered separately by TestSPConcurrentExitNumBound via Params.Validate.
func TestSPConcurrentExitNum(t *testing.T) {
	var zeroInt math.Int
	valid := math.NewInt(1)
	invalid := math.NewInt(-1)

	tests := []struct {
		name   string
		number interface{}
		err    string
	}{
		{
			name:   caseValid,
			number: &valid,
		},
		{
			name:   "invalid type",
			number: 1,
			err:    "invalid parameter type",
		},
		{
			name:   "nil pointer is allowed",
			number: (*math.Int)(nil),
		},
		{
			name:   "zero-value Int is allowed",
			number: &zeroInt,
		},
		{
			name:   "negative",
			number: &invalid,
			err:    "number of sp concurrent exit must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSPConcurrentExitNum(tt.number)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}
