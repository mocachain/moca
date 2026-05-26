package types

// ModuleName names the precompile-supporting subset of moca's old x/evm module.
// The package now exists only to back moca's chain-specific precompiles
// (storage, virtualgroup, storageprovider, etc.) with their helpers and a
// distinct SDK error codespace; cosmos/evm v0.6.0's x/vm owns the actual EVM
// module under the "evm" codespace, so moca uses its own name here to avoid
// colliding with cosmos/evm at error registration.
const ModuleName = "mocaprecompiles"

// MethodArgs is implemented by precompile method-argument structs. ParseMethodArgs
// invokes Validate after copying the ABI-decoded inputs into the struct.
type MethodArgs interface {
	Validate() error
}
