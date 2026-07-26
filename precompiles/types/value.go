package types

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/vm"
)

// ErrValueNotAccepted is returned when a precompile call carries native value.
var ErrValueNotAccepted = errors.New("precompile does not accept value")

// RejectValue returns an error if the call carries native value.
func RejectValue(contract *vm.Contract) error {
	if v := contract.Value(); v != nil && v.Sign() != 0 {
		return ErrValueNotAccepted
	}
	return nil
}
