package types_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/sp/types"
)

func TestRegisterCodec(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	require.NotPanics(t, func() { types.RegisterCodec(cdc) })
}

func TestRegisterInterfaces(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	require.NotPanics(t, func() { types.RegisterInterfaces(registry) })

	for _, msg := range []sdk.Msg{
		&types.MsgCreateStorageProvider{},
		&types.MsgDeposit{},
		&types.MsgEditStorageProvider{},
		&types.MsgUpdateSpStoragePrice{},
		&types.MsgUpdateStorageProviderStatus{},
		&types.MsgUpdateParams{},
	} {
		_, err := registry.Resolve(sdk.MsgTypeURL(msg))
		require.NoError(t, err)
	}

	_, err := registry.Resolve(sdk.MsgTypeURL(&types.DepositAuthorization{}))
	require.NoError(t, err)
}
