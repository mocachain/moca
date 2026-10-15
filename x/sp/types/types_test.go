package types_test

import (
	"strings"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func TestNewStorageProvider(t *testing.T) {
	operator := sample.RandAccAddress()
	funding := sample.RandAccAddress()
	seal := sample.RandAccAddress()
	approval := sample.RandAccAddress()
	gc := sample.RandAccAddress()
	maintenance := sample.RandAccAddress()
	desc := types.NewDescription("m", "i", "w", "d")
	deposit := math.NewInt(100)

	t.Run("valid bls key", func(t *testing.T) {
		blsKeyHex := sample.RandBlsPubKeyHex()
		sp, err := types.NewStorageProvider(1, operator, funding, seal, approval, gc, maintenance,
			deposit, "http://127.0.0.1:9033", desc, blsKeyHex)
		require.NoError(t, err)
		require.Equal(t, uint32(1), sp.Id)
		require.Equal(t, operator.String(), sp.OperatorAddress)
		require.Equal(t, funding.String(), sp.FundingAddress)
		require.Equal(t, seal.String(), sp.SealAddress)
		require.Equal(t, approval.String(), sp.ApprovalAddress)
		require.Equal(t, gc.String(), sp.GcAddress)
		require.Equal(t, maintenance.String(), sp.MaintenanceAddress)
		require.Equal(t, deposit, sp.TotalDeposit)
		require.Equal(t, "http://127.0.0.1:9033", sp.Endpoint)
		require.Equal(t, desc, sp.Description)
	})

	t.Run("invalid hex bls key errors", func(t *testing.T) {
		_, err := types.NewStorageProvider(1, operator, funding, seal, approval, gc, maintenance,
			deposit, "http://127.0.0.1:9033", desc, "not-hex")
		require.Error(t, err)
	})
}

func TestStorageProvider_GetAccAddresses(t *testing.T) {
	operator := sample.RandAccAddress()
	funding := sample.RandAccAddress()
	seal := sample.RandAccAddress()
	approval := sample.RandAccAddress()
	gc := sample.RandAccAddress()
	maintenance := sample.RandAccAddress()

	sp := types.StorageProvider{
		OperatorAddress:    operator.String(),
		FundingAddress:     funding.String(),
		SealAddress:        seal.String(),
		ApprovalAddress:    approval.String(),
		GcAddress:          gc.String(),
		MaintenanceAddress: maintenance.String(),
	}

	require.Equal(t, operator, sp.GetOperatorAccAddress())
	require.Equal(t, funding, sp.GetFundingAccAddress())
	require.Equal(t, seal, sp.GetSealAccAddress())
	require.Equal(t, approval, sp.GetApprovalAccAddress())
	require.Equal(t, gc, sp.GetGcAccAddress())
	// GetTestAccAddress reads the MaintenanceAddress field despite its name.
	require.Equal(t, maintenance, sp.GetTestAccAddress())
}

func TestStorageProvider_GetAccAddresses_EmptyOperatorShortCircuits(t *testing.T) {
	empty := types.StorageProvider{}
	require.Equal(t, sdk.AccAddress{}, empty.GetFundingAccAddress())
	require.Equal(t, sdk.AccAddress{}, empty.GetSealAccAddress())
	require.Equal(t, sdk.AccAddress{}, empty.GetApprovalAccAddress())
	require.Equal(t, sdk.AccAddress{}, empty.GetGcAccAddress())
	require.Equal(t, sdk.AccAddress{}, empty.GetTestAccAddress())
}

func TestStorageProvider_StatusHelpers(t *testing.T) {
	inService := types.StorageProvider{Status: types.STATUS_IN_SERVICE}
	require.True(t, inService.IsInService())
	require.False(t, inService.IsInMaintenance())

	inMaintenance := types.StorageProvider{Status: types.STATUS_IN_MAINTENANCE}
	require.False(t, inMaintenance.IsInService())
	require.True(t, inMaintenance.IsInMaintenance())

	jailed := types.StorageProvider{Status: types.STATUS_IN_JAILED}
	require.False(t, jailed.IsInService())
	require.False(t, jailed.IsInMaintenance())
}

func TestStorageProvider_GetTotalDeposit(t *testing.T) {
	sp := types.StorageProvider{TotalDeposit: math.NewInt(42)}
	require.Equal(t, math.NewInt(42), sp.GetTotalDeposit())
}

func TestDescription_EnsureLength(t *testing.T) {
	longStr := func(n int) string { return strings.Repeat("a", n) }

	tests := []struct {
		name    string
		desc    types.Description
		wantErr bool
	}{
		{"valid", types.NewDescription("m", "i", "w", "d"), false},
		{"moniker too long", types.Description{Moniker: longStr(types.MaxMonikerLength + 1)}, true},
		{"identity too long", types.Description{Identity: longStr(types.MaxIdentityLength + 1)}, true},
		{"website too long", types.Description{Website: longStr(types.MaxWebsiteLength + 1)}, true},
		{"details too long", types.Description{Details: longStr(types.MaxDetailsLength + 1)}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.desc
			err := d.EnsureLength()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDescription_UpdateDescription(t *testing.T) {
	base := types.NewDescription("m", "i", "w", "d")

	t.Run("do-not-modify sentinels keep the original values", func(t *testing.T) {
		update := types.NewDescription(types.DoNotModifyDesc, types.DoNotModifyDesc, types.DoNotModifyDesc, types.DoNotModifyDesc)
		got, err := base.UpdateDescription(&update)
		require.NoError(t, err)
		require.Equal(t, base, *got)
	})

	t.Run("new values are applied", func(t *testing.T) {
		update := types.NewDescription("m2", "i2", "w2", "d2")
		got, err := base.UpdateDescription(&update)
		require.NoError(t, err)
		require.Equal(t, "m2", got.Moniker)
		require.Equal(t, "i2", got.Identity)
		require.Equal(t, "w2", got.Website)
		require.Equal(t, "d2", got.Details)
	})

	t.Run("length validation failure propagates", func(t *testing.T) {
		update := types.NewDescription(strings.Repeat("a", types.MaxMonikerLength+1), "i2", "w2", "d2")
		_, err := base.UpdateDescription(&update)
		require.Error(t, err)
	})
}
