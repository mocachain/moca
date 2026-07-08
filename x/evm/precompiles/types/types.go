package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

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

// RegisterInterfaces forwards to cosmos/evm's vm types' registration so that
// moca's authz precompile (which builds an ad-hoc local interface registry to
// unpack inner authorizations) keeps registering the eth tx types under the
// upstream type URLs. This is a compatibility shim: pre-migration moca
// shipped its own MsgEthereumTx in x/evm/types and registered it here;
// post-migration the same type is owned by cosmos/evm.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	evmtypes.RegisterInterfaces(registry)
}
