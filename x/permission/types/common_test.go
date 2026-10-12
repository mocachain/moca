package types_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	"github.com/mocachain/moca/v2/x/permission/types"
)

func TestNewPrincipalWithGroupID(t *testing.T) {
	groupID := sdkmath.NewUint(42)
	p := types.NewPrincipalWithGroupID(groupID)
	require.Equal(t, types.PRINCIPAL_TYPE_GNFD_GROUP, p.Type)
	require.Equal(t, groupID.String(), p.Value)
}

func TestNewPrincipalWithGroupInfo(t *testing.T) {
	owner := sample.RandAccAddress()
	p := types.NewPrincipalWithGroupInfo(owner, "mygroup")
	require.Equal(t, types.PRINCIPAL_TYPE_GNFD_GROUP, p.Type)
	require.Equal(t, types2.NewGroupGRN(owner, "mygroup").String(), p.Value)
}

func TestPrincipal_ValidateBasic(t *testing.T) {
	validAddr := sample.RandAccAddress().String()
	tests := []struct {
		name    string
		p       *types.Principal
		wantErr bool
	}{
		{"unspecified", &types.Principal{Type: types.PRINCIPAL_TYPE_UNSPECIFIED}, true},
		{"account valid", &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_ACCOUNT, Value: validAddr}, false},
		{"account invalid", &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_ACCOUNT, Value: "not-an-address"}, true},
		{"group any value", &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_GROUP, Value: "unchecked"}, false},
		{"unknown type", &types.Principal{Type: types.PrincipalType(99)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.ValidateBasic()
			if tt.wantErr {
				require.ErrorIs(t, err, types.ErrInvalidPrincipal)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPrincipal_GetAccountAddress(t *testing.T) {
	addr := sample.RandAccAddress()
	p := types.NewPrincipalWithAccount(addr)
	got, err := p.GetAccountAddress()
	require.NoError(t, err)
	require.Equal(t, addr, got)

	badValue := &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_ACCOUNT, Value: "not-hex"}
	_, err = badValue.GetAccountAddress()
	require.ErrorIs(t, err, types.ErrInvalidPrincipal)

	mismatched := types.NewPrincipalWithGroupID(sdkmath.NewUint(1))
	require.Panics(t, func() {
		_, _ = mismatched.GetAccountAddress()
	})
}

func TestPrincipal_GetGroupID(t *testing.T) {
	groupID := sdkmath.NewUint(7)
	p := types.NewPrincipalWithGroupID(groupID)
	got, err := p.GetGroupID()
	require.NoError(t, err)
	require.True(t, groupID.Equal(got))

	badValue := &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_GROUP, Value: "not-a-number"}
	_, err = badValue.GetGroupID()
	require.ErrorIs(t, err, types.ErrInvalidPrincipal)

	mismatched := types.NewPrincipalWithAccount(sample.RandAccAddress())
	require.Panics(t, func() {
		_, _ = mismatched.GetGroupID()
	})
}

func TestPrincipal_MustGetAccountAddress(t *testing.T) {
	addr := sample.RandAccAddress()
	p := types.NewPrincipalWithAccount(addr)
	require.Equal(t, addr, p.MustGetAccountAddress())

	badValue := &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_ACCOUNT, Value: "not-hex"}
	require.Panics(t, func() {
		badValue.MustGetAccountAddress()
	})
}

func TestPrincipal_MustGetGroupID(t *testing.T) {
	groupID := sdkmath.NewUint(9)
	p := types.NewPrincipalWithGroupID(groupID)
	require.True(t, groupID.Equal(p.MustGetGroupID()))

	badValue := &types.Principal{Type: types.PRINCIPAL_TYPE_GNFD_GROUP, Value: "not-a-number"}
	require.Panics(t, func() {
		badValue.MustGetGroupID()
	})
}
