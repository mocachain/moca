package types_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func TestNewDepositAuthorization(t *testing.T) {
	spAddr := sample.RandAccAddress()
	coin := sdk.NewInt64Coin(types.DefaultDepositDenom, 100)

	a := types.NewDepositAuthorization(spAddr, &coin)
	require.Equal(t, spAddr.String(), a.SpAddress)
	require.Equal(t, &coin, a.MaxDeposit)
}

func TestDepositAuthorization_MsgTypeURL(t *testing.T) {
	a := types.DepositAuthorization{}
	require.Equal(t, sdk.MsgTypeURL(&types.MsgDeposit{}), a.MsgTypeURL())
}

func TestDepositAuthorization_ValidateBasic(t *testing.T) {
	posCoin := sdk.NewInt64Coin(types.DefaultDepositDenom, 100)
	negCoin := sdk.Coin{Denom: types.DefaultDepositDenom, Amount: math.NewInt(-1)}

	tests := []struct {
		name       string
		maxDeposit *sdk.Coin
		wantErr    bool
	}{
		{"nil max deposit is unlimited and valid", nil, false},
		{"positive max deposit", &posCoin, false},
		{"negative max deposit", &negCoin, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := types.DepositAuthorization{MaxDeposit: tc.maxDeposit}
			err := a.ValidateBasic()
			if tc.wantErr {
				require.ErrorIs(t, err, authz.ErrNegativeMaxTokens)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDepositAuthorization_Accept(t *testing.T) {
	spAddr := sample.RandAccAddress()
	otherAddr := sample.RandAccAddress()

	t.Run("wrong msg type is rejected", func(t *testing.T) {
		a := types.DepositAuthorization{SpAddress: spAddr.String()}
		_, err := a.Accept(context.Background(), &types.MsgCreateStorageProvider{})
		require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	})

	t.Run("sp address mismatch is rejected", func(t *testing.T) {
		a := types.DepositAuthorization{SpAddress: spAddr.String()}
		msg := &types.MsgDeposit{SpAddress: otherAddr.String()}
		_, err := a.Accept(context.Background(), msg)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	})

	t.Run("nil max deposit always accepts and stays unlimited", func(t *testing.T) {
		a := types.DepositAuthorization{SpAddress: spAddr.String()}
		msg := &types.MsgDeposit{SpAddress: spAddr.String(), Deposit: sdk.NewInt64Coin(types.DefaultDepositDenom, 5)}
		resp, err := a.Accept(context.Background(), msg)
		require.NoError(t, err)
		require.True(t, resp.Accept)
		require.False(t, resp.Delete)
		require.Equal(t, &types.DepositAuthorization{SpAddress: spAddr.String()}, resp.Updated)
	})

	t.Run("exact exhaustion deletes the grant", func(t *testing.T) {
		limit := sdk.NewInt64Coin(types.DefaultDepositDenom, 100)
		a := types.DepositAuthorization{SpAddress: spAddr.String(), MaxDeposit: &limit}
		msg := &types.MsgDeposit{SpAddress: spAddr.String(), Deposit: sdk.NewInt64Coin(types.DefaultDepositDenom, 100)}
		resp, err := a.Accept(context.Background(), msg)
		require.NoError(t, err)
		require.True(t, resp.Accept)
		require.True(t, resp.Delete)
		require.Nil(t, resp.Updated)
	})

	t.Run("partial deposit reduces the remaining limit", func(t *testing.T) {
		limit := sdk.NewInt64Coin(types.DefaultDepositDenom, 100)
		a := types.DepositAuthorization{SpAddress: spAddr.String(), MaxDeposit: &limit}
		msg := &types.MsgDeposit{SpAddress: spAddr.String(), Deposit: sdk.NewInt64Coin(types.DefaultDepositDenom, 40)}
		resp, err := a.Accept(context.Background(), msg)
		require.NoError(t, err)
		require.True(t, resp.Accept)
		require.False(t, resp.Delete)
		updated, ok := resp.Updated.(*types.DepositAuthorization)
		require.True(t, ok)
		require.Equal(t, sdk.NewInt64Coin(types.DefaultDepositDenom, 60), *updated.MaxDeposit)
	})

	t.Run("deposit exceeding the remaining limit errors", func(t *testing.T) {
		limit := sdk.NewInt64Coin(types.DefaultDepositDenom, 100)
		a := types.DepositAuthorization{SpAddress: spAddr.String(), MaxDeposit: &limit}
		msg := &types.MsgDeposit{SpAddress: spAddr.String(), Deposit: sdk.NewInt64Coin(types.DefaultDepositDenom, 200)}
		_, err := a.Accept(context.Background(), msg)
		require.Error(t, err)
	})
}
