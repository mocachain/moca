package types

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"

	mocatypes "github.com/mocachain/moca/v2/types"
)

var (
	ParamStoreKeyEnableErc20                = []byte("EnableErc20")
	ParamStoreKeyDynamicPrecompiles         = []byte("DynamicPrecompiles")
	ParamStoreKeyNativePrecompiles          = []byte("NativePrecompiles")
	ParamStoreKeyPermissionlessRegistration = []byte("PermissionlessRegistration")
)

var (
	DefaultNativePrecompiles  []string
	DefaultDynamicPrecompiles []string
)

func NewParams(
	enableErc20 bool,
	nativePrecompiles []string,
	dynamicPrecompiles []string,
	permissionlessRegistration bool,
) Params {
	slices.Sort(nativePrecompiles)
	slices.Sort(dynamicPrecompiles)
	return Params{
		EnableErc20:                enableErc20,
		NativePrecompiles:          nativePrecompiles,
		DynamicPrecompiles:         dynamicPrecompiles,
		PermissionlessRegistration: permissionlessRegistration,
	}
}

func DefaultParams() Params {
	return Params{
		EnableErc20:                true,
		NativePrecompiles:          DefaultNativePrecompiles,
		DynamicPrecompiles:         DefaultDynamicPrecompiles,
		PermissionlessRegistration: true,
	}
}

func (p Params) Validate() error {
	npAddrs, err := ValidatePrecompiles(p.NativePrecompiles)
	if err != nil {
		return err
	}

	dpAddrs, err := ValidatePrecompiles(p.DynamicPrecompiles)
	if err != nil {
		return err
	}

	combined := dpAddrs
	combined = append(combined, npAddrs...)
	return validatePrecompilesUniqueness(combined)
}

func ValidatePrecompiles(precompiles []string) ([]common.Address, error) {
	precAddrs := make([]common.Address, 0, len(precompiles))
	for _, precompile := range precompiles {
		if err := mocatypes.ValidateAddress(precompile); err != nil {
			return nil, fmt.Errorf("invalid precompile %s", precompile)
		}
		precAddrs = append(precAddrs, common.HexToAddress(precompile))
	}

	if !slices.IsSorted(precompiles) {
		return nil, fmt.Errorf("precompiles need to be sorted: %s", precompiles)
	}
	return precAddrs, nil
}

func validatePrecompilesUniqueness(precompiles []common.Address) error {
	seenPrecompiles := make(map[string]struct{})
	for _, precompile := range precompiles {
		if _, ok := seenPrecompiles[precompile.Hex()]; ok {
			return fmt.Errorf("duplicate precompile %s", precompile)
		}
		seenPrecompiles[precompile.Hex()] = struct{}{}
	}
	return nil
}

func (p Params) IsNativePrecompile(addr common.Address) bool {
	return isAddrIncluded(addr, p.NativePrecompiles)
}

func (p Params) IsDynamicPrecompile(addr common.Address) bool {
	return isAddrIncluded(addr, p.DynamicPrecompiles)
}

func isAddrIncluded(addr common.Address, strAddrs []string) bool {
	for _, sa := range strAddrs {
		cmnAddr := common.HexToAddress(sa)
		if bytes.Equal(addr.Bytes(), cmnAddr.Bytes()) {
			return true
		}
	}
	return false
}
