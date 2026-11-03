package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
)

// RegisterInterfaces registers the pre-v2 Ethereum transaction types with registry.
// The CLI and the node's query services register them; the app does not.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil), &MsgEthereumTx{})
	registry.RegisterInterface(
		"ethermint.evm.v1.TxData",
		(*TxData)(nil),
		&LegacyTx{},
		&AccessListTx{},
		&DynamicFeeTx{},
	)
	registry.RegisterImplementations((*tx.TxExtensionOptionI)(nil), &ExtensionOptionsEthereumTx{})
}
