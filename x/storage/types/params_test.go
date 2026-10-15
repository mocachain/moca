package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const wantInvalidParamType = "invalid parameter type"

func TestValidateMaxSegmentSize(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(16 * 1024 * 1024), ""},
		{"invalid type", 1, wantInvalidParamType},
		{"zero", uint64(0), "max segment size must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxSegmentSize(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateRedundantDataChunkNum(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint32(4), ""},
		{"invalid type", 4, wantInvalidParamType},
		{"zero", uint32(0), "redundant data chunk num must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRedundantDataChunkNum(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateRedundantParityChunkNum(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint32(2), ""},
		{"invalid type", 2, wantInvalidParamType},
		{"zero", uint32(0), "redundant parity size chunk num must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRedundantParityChunkNum(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateMinChargeSize(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(1024), ""},
		{"invalid type", 1024, wantInvalidParamType},
		{"zero", uint64(0), "min charge size must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMinChargeSize(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateMaxPayloadSize(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(1024), ""},
		{"invalid type", 1024, wantInvalidParamType},
		{"zero", uint64(0), "max payload size must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxPayloadSize(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateMaxBucketsPerAccount(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint32(100), ""},
		{"invalid type", 100, wantInvalidParamType},
		{"zero", uint32(0), "max buckets per account must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxBucketsPerAccount(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateDiscontinueCountingWindow(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(10000), ""},
		{"invalid type", 10000, wantInvalidParamType},
		{"zero", uint64(0), "discontinue counting window must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiscontinueCountingWindow(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidateDiscontinueObjectMax documents that, unlike the other
// discontinue-family validators, this one only type-checks -- a zero value
// passes today.
func TestValidateDiscontinueObjectMax(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(100000), ""},
		{"zero is accepted", uint64(0), ""},
		{"invalid type", 100000, wantInvalidParamType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiscontinueObjectMax(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidateDiscontinueBucketMax: same "type check only" contract as
// TestValidateDiscontinueObjectMax above.
func TestValidateDiscontinueBucketMax(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(10000), ""},
		{"zero is accepted", uint64(0), ""},
		{"invalid type", 10000, wantInvalidParamType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiscontinueBucketMax(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateDiscontinueConfirmPeriod(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", int64(604800), ""},
		{"invalid type", 604800, wantInvalidParamType},
		{"zero", int64(0), "discontinue confirm period must be positive"},
		{"negative", int64(-1), "discontinue confirm period must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiscontinueConfirmPeriod(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateDiscontinueDeletionMax(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(100), ""},
		{"invalid type", 100, wantInvalidParamType},
		{"zero", uint64(0), "discontinue deletion max must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiscontinueDeletionMax(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateStalePolicyCleanupMax(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(200), ""},
		{"invalid type", 200, wantInvalidParamType},
		{"zero", uint64(0), "max stale policy to cleanup must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStalePolicyCleanupMax(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidateMinUpdateQuotaInterval documents that, like the discontinue
// object/bucket max validators, this one only type-checks.
func TestValidateMinUpdateQuotaInterval(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint64(2592000), ""},
		{"zero is accepted", uint64(0), ""},
		{"invalid type", 2592000, wantInvalidParamType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMinUpdateQuotaInterval(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateMaxLocalVirtualGroupNumPerBucket(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		err  string
	}{
		{"valid", uint32(10), ""},
		{"invalid type", 10, wantInvalidParamType},
		{"zero", uint32(0), "max LVG per bucket must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxLocalVirtualGroupNumPerBucket(tt.val)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParams_Validate(t *testing.T) {
	require.NoError(t, DefaultParams().Validate(), "the default params must be valid")

	tests := []struct {
		name        string
		mutate      func(p *Params)
		errContains string
	}{
		{"max segment size zero", func(p *Params) { p.VersionedParams.MaxSegmentSize = 0 }, "max segment size must be positive"},
		{"redundant data chunk num zero", func(p *Params) { p.VersionedParams.RedundantDataChunkNum = 0 }, "redundant data chunk num must be positive"},
		{"redundant parity chunk num zero", func(p *Params) { p.VersionedParams.RedundantParityChunkNum = 0 }, "redundant parity size chunk num must be positive"},
		{"min charge size zero", func(p *Params) { p.VersionedParams.MinChargeSize = 0 }, "min charge size must be positive"},
		{"max payload size zero", func(p *Params) { p.MaxPayloadSize = 0 }, "max payload size must be positive"},
		{"max buckets per account zero", func(p *Params) { p.MaxBucketsPerAccount = 0 }, "max buckets per account must be positive"},
		{"discontinue counting window zero", func(p *Params) { p.DiscontinueCountingWindow = 0 }, "discontinue counting window must be positive"},
		{"discontinue confirm period zero", func(p *Params) { p.DiscontinueConfirmPeriod = 0 }, "discontinue confirm period must be positive"},
		{"discontinue deletion max zero", func(p *Params) { p.DiscontinueDeletionMax = 0 }, "discontinue deletion max must be positive"},
		{"stale policy cleanup max zero", func(p *Params) { p.StalePolicyCleanupMax = 0 }, "max stale policy to cleanup must be positive"},
		{"max lvg per bucket zero", func(p *Params) { p.MaxLocalVirtualGroupNumPerBucket = 0 }, "max LVG per bucket must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := DefaultParams()
			tt.mutate(&p)
			err := p.Validate()
			require.ErrorContains(t, err, tt.errContains)
		})
	}
}

func TestDefaultParams(t *testing.T) {
	p := DefaultParams()
	require.NoError(t, p.Validate())
	require.Equal(t, DefaultMaxSegmentSize, p.VersionedParams.MaxSegmentSize)
	require.Equal(t, DefaultRedundantDataChunkNum, p.VersionedParams.RedundantDataChunkNum)
	require.Equal(t, DefaultRedundantParityChunkNum, p.VersionedParams.RedundantParityChunkNum)
	require.Equal(t, DefaultMinChargeSize, p.VersionedParams.MinChargeSize)
	require.Equal(t, DefaultMaxPayloadSize, p.MaxPayloadSize)
	require.Equal(t, DefaultMaxBucketsPerAccount, p.MaxBucketsPerAccount)
	require.Equal(t, DefaultDiscontinueCountingWindow, p.DiscontinueCountingWindow)
	require.Equal(t, DefaultMaxLocalVirtualGroupNumPerBucket, p.MaxLocalVirtualGroupNumPerBucket)
}

func TestParams_String(t *testing.T) {
	p := DefaultParams()
	p.MaxBucketsPerAccount = 424242
	out := p.String()
	require.NotEmpty(t, out)
	require.Contains(t, out, "424242")
}

func TestVersionedParams_String(t *testing.T) {
	vp := VersionedParams{MaxSegmentSize: 987654321}
	out := vp.String()
	require.NotEmpty(t, out)
	require.Contains(t, out, "987654321")
}

func TestParams_Getters(t *testing.T) {
	var nilParams *Params
	require.Equal(t, uint64(0), nilParams.GetMaxSegmentSize())
	require.Equal(t, uint32(0), nilParams.GetRedundantDataChunkNum())
	require.Equal(t, uint32(0), nilParams.GetRedundantParityChunkNum())
	require.Equal(t, uint64(0), nilParams.GetMinChargeSize())

	p := &Params{VersionedParams: VersionedParams{
		MaxSegmentSize:          11,
		RedundantDataChunkNum:   22,
		RedundantParityChunkNum: 33,
		MinChargeSize:           44,
	}}
	require.Equal(t, uint64(11), p.GetMaxSegmentSize())
	require.Equal(t, uint32(22), p.GetRedundantDataChunkNum())
	require.Equal(t, uint32(33), p.GetRedundantParityChunkNum())
	require.Equal(t, uint64(44), p.GetMinChargeSize())
}
