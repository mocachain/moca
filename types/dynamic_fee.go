package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	evmantetypes "github.com/cosmos/evm/ante/types"
)

// HasDynamicFeeExtensionOption returns true if the tx implements the `ExtensionOptionDynamicFeeTx` extension option.
func HasDynamicFeeExtensionOption(any *codectypes.Any) bool {
	_, ok := any.GetCachedValue().(*evmantetypes.ExtensionOptionDynamicFeeTx)
	return ok
}
