package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidators_RejectWrongType exercises the defensive "invalid parameter
// type" branch of each params.go validator. Params.Validate() -- the only
// production caller -- always passes correctly-typed Go values, so these
// branches are unreachable from the exported API; the only way to close them
// is to call the unexported validators directly with a deliberately
// wrong-typed value. That unexported access is why this table lives in an
// in-package _internal_test.go file instead of alongside the exported-API
// tests in params_test.go.
func TestValidators_RejectWrongType(t *testing.T) {
	tests := []struct {
		name string
		fn   func(interface{}) error
		bad  interface{}
	}{
		{name: "validateReserveTime", fn: validateReserveTime, bad: "not-a-uint64"},
		{name: "validateForcedSettleTime", fn: validateForcedSettleTime, bad: "not-a-uint64"},
		{name: "validatePaymentAccountCountLimit", fn: validatePaymentAccountCountLimit, bad: "not-a-uint64"},
		{name: "validateMaxAutoSettleFlowCount", fn: validateMaxAutoSettleFlowCount, bad: "not-a-uint64"},
		{name: "validateMaxAutoResumeFlowCount", fn: validateMaxAutoResumeFlowCount, bad: "not-a-uint64"},
		{name: "validateFeeDenom", fn: validateFeeDenom, bad: uint64(1)},
		{name: "validateValidatorTaxRate", fn: validateValidatorTaxRate, bad: "not-a-dec"},
		{name: "validateWithdrawTimeLockThreshold", fn: validateWithdrawTimeLockThreshold, bad: "not-an-int-pointer"},
		{name: "validateWithdrawTimeLockDuration", fn: validateWithdrawTimeLockDuration, bad: "not-a-uint64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn(tt.bad)
			require.EqualError(t, err, fmt.Sprintf("invalid parameter type: %T", tt.bad))
		})
	}
}
