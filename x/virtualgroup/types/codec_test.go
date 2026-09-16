package types

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

// aminoMsgWrapper gives an sdk.Msg an interface-typed field to marshal
// through, the same way it sits inside a signed legacy-amino transaction.
// Marshaling the concrete struct directly would skip amino's interface
// resolution entirely, so it would pass whether or not RegisterCodec ever
// ran, and it would not have proven that the registered names are correct.
type aminoMsgWrapper struct {
	Msg sdk.Msg
}

func TestRegisterCodec(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	// Production wiring always registers the sdk.Msg interface itself (via
	// each module's app.go/std codec setup) before any module's concrete
	// types; do the same here so marshaling through the interface below
	// actually exercises RegisterCodec's registrations instead of failing
	// on the interface being unknown.
	sdk.RegisterLegacyAminoCodec(cdc)
	RegisterCodec(cdc)

	tests := []struct {
		name     string
		msg      sdk.Msg
		wantName string
	}{
		{"storage provider exit", &MsgStorageProviderExit{StorageProvider: sample.RandAccAddressHex()}, "virtualgroup/StorageProviderExit"},
		{"complete storage provider exit", &MsgCompleteStorageProviderExit{StorageProvider: sample.RandAccAddressHex()}, "virtualgroup/CompleteStorageProviderExit"},
		{"complete swap out", &MsgCompleteSwapOut{StorageProvider: sample.RandAccAddressHex()}, "virtualgroup/CompleteSwapOut"},
		{"cancel swap out", &MsgCancelSwapOut{StorageProvider: sample.RandAccAddressHex()}, "virtualgroup/CancelSwapOut"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bz, err := cdc.MarshalJSON(aminoMsgWrapper{Msg: tt.msg})
			require.NoError(t, err)
			require.Contains(t, string(bz), tt.wantName)
		})
	}
}

func TestRegisterInterfaces(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()

	// Before registration the concrete types are unknown to the registry.
	_, err := registry.Resolve("/moca.virtualgroup.MsgCreateGlobalVirtualGroup")
	require.Error(t, err)

	RegisterInterfaces(registry)

	typeURLs := []string{
		"/moca.virtualgroup.MsgCreateGlobalVirtualGroup",
		"/moca.virtualgroup.MsgDeleteGlobalVirtualGroup",
		"/moca.virtualgroup.MsgStorageProviderExit",
		"/moca.virtualgroup.MsgCompleteStorageProviderExit",
		"/moca.virtualgroup.MsgSwapOut",
		"/moca.virtualgroup.MsgDeposit",
		"/moca.virtualgroup.MsgWithdraw",
		"/moca.virtualgroup.MsgSettle",
		"/moca.virtualgroup.MsgUpdateParams",
		"/moca.virtualgroup.MsgCompleteSwapOut",
		"/moca.virtualgroup.MsgCancelSwapOut",
	}
	for _, url := range typeURLs {
		t.Run(url, func(t *testing.T) {
			msg, err := registry.Resolve(url)
			require.NoError(t, err)
			require.NotNil(t, msg)
		})
	}
}
