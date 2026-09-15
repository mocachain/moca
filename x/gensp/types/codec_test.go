package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/gensp/types"
)

func TestRegisterCodec(t *testing.T) {
	require.NotPanics(t, func() {
		types.RegisterCodec(codec.NewLegacyAmino())
	})
}

func TestRegisterInterfaces(t *testing.T) {
	require.NotPanics(t, func() {
		types.RegisterInterfaces(cdctypes.NewInterfaceRegistry())
	})
}
