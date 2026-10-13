package types

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// storage params default values
const (
	DefaultMaxSegmentSize            uint64 = 16 * 1024 * 1024 // 16M
	DefaultRedundantDataChunkNum     uint32 = 4
	DefaultRedundantParityChunkNum   uint32 = 2
	DefaultMaxPayloadSize            uint64 = 64 * 1024 * 1024 * 1024
	DefaultMaxBucketsPerAccount      uint32 = 100
	DefaultMinChargeSize             uint64 = 1 * 1024 * 1024 // 1M
	DefaultDiscontinueCountingWindow uint64 = 10000
	// Finite, not MaxUint64: an unbounded queue fills faster than the ~100/block
	// drain removes, so it never clears and bloats state (slower blocks). 100k / 10k
	// per SP per counting window stay well under the window drain (100 x 10000 = 1M).
	DefaultDiscontinueObjectMax     uint64 = 100000
	DefaultDiscontinueBucketMax     uint64 = 10000
	DefaultDiscontinueConfirmPeriod int64  = 604800 // 7 days (in second)
	DefaultDiscontinueDeletionMax   uint64 = 100
	DefaultStalePolicyCleanupMax    uint64 = 200
	DefaultMinUpdateQuotaInterval   uint64 = 2592000 // 30 days (in second)

	// TODO
	DefaultMaxLocalVirtualGroupNumPerBucket uint32 = 10
)

var (
	KeyMaxSegmentSize                   = []byte("MaxSegmentSize")
	KeyRedundantDataChunkNum            = []byte("RedundantDataChunkNum")
	KeyRedundantParityChunkNum          = []byte("RedundantParityChunkNum")
	KeyMaxPayloadSize                   = []byte("MaxPayloadSize")
	KeyMinChargeSize                    = []byte("MinChargeSize")
	KeyMaxBucketsPerAccount             = []byte("MaxBucketsPerAccount")
	KeyDiscontinueCountingWindow        = []byte("DiscontinueCountingWindow")
	KeyDiscontinueObjectMax             = []byte("DiscontinueObjectMax")
	KeyDiscontinueBucketMax             = []byte("DiscontinueBucketMax")
	KeyDiscontinueConfirmPeriod         = []byte("DiscontinueConfirmPeriod")
	KeyDiscontinueDeletionMax           = []byte("DiscontinueDeletionMax")
	KeyStalePolicyCleanupMax            = []byte("StalePolicyCleanupMax")
	KeyMinUpdateQuotaInterval           = []byte("MinUpdateQuotaInterval")
	KeyMaxLocalVirtualGroupNumPerBucket = []byte("MaxLocalVirtualGroupNumPerBucket")
)

// NewParams creates a new Params instance
func NewParams(
	maxSegmentSize uint64, redundantDataChunkNum uint32,
	redundantParityChunkNum uint32, maxPayloadSize uint64, maxBucketsPerAccount uint32,
	minChargeSize uint64,
	discontinueCountingWindow, discontinueObjectMax, discontinueBucketMax uint64,
	discontinueConfirmPeriod int64,
	discontinueDeletionMax uint64,
	stalePoliesCleanupMax uint64,
	minUpdateQuotaInterval uint64,
	maxLocalVirtualGroupNumPerBucket uint32,
) Params {
	return Params{
		VersionedParams: VersionedParams{
			MaxSegmentSize:          maxSegmentSize,
			RedundantDataChunkNum:   redundantDataChunkNum,
			RedundantParityChunkNum: redundantParityChunkNum,
			MinChargeSize:           minChargeSize,
		},
		MaxPayloadSize:                   maxPayloadSize,
		MaxBucketsPerAccount:             maxBucketsPerAccount,
		DiscontinueCountingWindow:        discontinueCountingWindow,
		DiscontinueObjectMax:             discontinueObjectMax,
		DiscontinueBucketMax:             discontinueBucketMax,
		DiscontinueConfirmPeriod:         discontinueConfirmPeriod,
		DiscontinueDeletionMax:           discontinueDeletionMax,
		StalePolicyCleanupMax:            stalePoliesCleanupMax,
		MinQuotaUpdateInterval:           minUpdateQuotaInterval,
		MaxLocalVirtualGroupNumPerBucket: maxLocalVirtualGroupNumPerBucket,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(
		DefaultMaxSegmentSize, DefaultRedundantDataChunkNum,
		DefaultRedundantParityChunkNum, DefaultMaxPayloadSize, DefaultMaxBucketsPerAccount,
		DefaultMinChargeSize,
		DefaultDiscontinueCountingWindow, DefaultDiscontinueObjectMax, DefaultDiscontinueBucketMax,
		DefaultDiscontinueConfirmPeriod, DefaultDiscontinueDeletionMax, DefaultStalePolicyCleanupMax,
		DefaultMinUpdateQuotaInterval, DefaultMaxLocalVirtualGroupNumPerBucket,
	)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateMaxSegmentSize(p.VersionedParams.MaxSegmentSize); err != nil {
		return err
	}
	if err := validateRedundantDataChunkNum(p.VersionedParams.RedundantDataChunkNum); err != nil {
		return err
	}
	if err := validateRedundantParityChunkNum(p.VersionedParams.RedundantParityChunkNum); err != nil {
		return err
	}
	if err := validateMinChargeSize(p.VersionedParams.MinChargeSize); err != nil {
		return err
	}
	if err := validateMaxPayloadSize(p.MaxPayloadSize); err != nil {
		return err
	}
	if err := validateMaxBucketsPerAccount(p.MaxBucketsPerAccount); err != nil {
		return err
	}
	if err := validateDiscontinueCountingWindow(p.DiscontinueCountingWindow); err != nil {
		return err
	}
	if err := validateDiscontinueObjectMax(p.DiscontinueObjectMax); err != nil {
		return err
	}
	if err := validateDiscontinueBucketMax(p.DiscontinueBucketMax); err != nil {
		return err
	}
	if err := validateDiscontinueConfirmPeriod(p.DiscontinueConfirmPeriod); err != nil {
		return err
	}
	if err := validateDiscontinueDeletionMax(p.DiscontinueDeletionMax); err != nil {
		return err
	}
	if err := validateStalePolicyCleanupMax(p.StalePolicyCleanupMax); err != nil {
		return err
	}
	if err := validateMinUpdateQuotaInterval(p.MinQuotaUpdateInterval); err != nil {
		return err
	}
	if err := validateMaxLocalVirtualGroupNumPerBucket(p.MaxLocalVirtualGroupNumPerBucket); err != nil {
		return err
	}
	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// String implements the Stringer interface.
func (p VersionedParams) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

func (p *Params) GetMaxSegmentSize() uint64 {
	if p != nil {
		return p.VersionedParams.MaxSegmentSize
	}
	return 0
}

func (p *Params) GetRedundantDataChunkNum() uint32 {
	if p != nil {
		return p.VersionedParams.RedundantDataChunkNum
	}
	return 0
}

func (p *Params) GetRedundantParityChunkNum() uint32 {
	if p != nil {
		return p.VersionedParams.RedundantParityChunkNum
	}
	return 0
}

func (p *Params) GetMinChargeSize() uint64 {
	if p != nil {
		return p.VersionedParams.MinChargeSize
	}
	return 0
}

func validateMaxSegmentSize(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("max segment size must be positive: %d", v)
	}

	return nil
}

func validateMaxPayloadSize(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("max payload size must be positive: %d", v)
	}

	return nil
}

func validateMaxBucketsPerAccount(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("max buckets per account must be positive: %d", v)
	}

	return nil
}

func validateMinChargeSize(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("min charge size must be positive: %d", v)
	}

	return nil
}

func validateRedundantDataChunkNum(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("redundant data chunk num must be positive: %d", v)
	}

	return nil
}

func validateRedundantParityChunkNum(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("redundant parity size chunk num must be positive: %d", v)
	}

	return nil
}

func validateDiscontinueCountingWindow(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("discontinue counting window must be positive: %d", v)
	}

	return nil
}

func validateDiscontinueObjectMax(i interface{}) error {
	_, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	return nil
}

func validateDiscontinueBucketMax(i interface{}) error {
	_, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	return nil
}

func validateDiscontinueConfirmPeriod(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v <= 0 {
		return fmt.Errorf("discontinue confirm period must be positive: %d", v)
	}
	return nil
}

func validateDiscontinueDeletionMax(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("discontinue deletion max must be positive: %d", v)
	}
	return nil
}

func validateStalePolicyCleanupMax(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf(" max stale policy to cleanup must be positive: %d", v)
	}
	return nil
}

func validateMinUpdateQuotaInterval(i interface{}) error {
	_, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	return nil
}

func validateMaxLocalVirtualGroupNumPerBucket(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("max LVG per bucket must be positive: %d", v)
	}

	return nil
}
