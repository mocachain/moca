package cli_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
	permissiontypes "github.com/mocachain/moca/v2/x/permission/types"
	"github.com/mocachain/moca/v2/x/storage/client/cli"
	"github.com/mocachain/moca/v2/x/storage/types"
)

func TestGetVisibilityType(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    types.VisibilityType
		wantErr error
	}{
		{
			name: "valid visibility type",
			arg:  "VISIBILITY_TYPE_PUBLIC_READ",
			want: types.VISIBILITY_TYPE_PUBLIC_READ,
		},
		{
			name:    "invalid visibility type falls back to private",
			arg:     "bogus",
			want:    types.VISIBILITY_TYPE_PRIVATE,
			wantErr: gnfderrors.ErrInvalidVisibilityType,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.GetVisibilityType(tt.arg)
			require.Equal(t, tt.want, got)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetActionType(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    permissiontypes.ActionType
		wantErr error
	}{
		{
			name: "valid action type",
			arg:  "ACTION_TYPE_ALL",
			want: permissiontypes.ACTION_TYPE_ALL,
		},
		{
			name:    "invalid action type falls back to unspecified",
			arg:     "bogus",
			want:    permissiontypes.ACTION_UNSPECIFIED,
			wantErr: gnfderrors.ErrInvalidActionType,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.GetActionType(tt.arg)
			require.Equal(t, tt.want, got)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetPrincipalType(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    permissiontypes.PrincipalType
		wantErr error
	}{
		{
			name: "valid principal type",
			arg:  "PRINCIPAL_TYPE_GNFD_ACCOUNT",
			want: permissiontypes.PRINCIPAL_TYPE_GNFD_ACCOUNT,
		},
		{
			name:    "invalid principal type falls back to unspecified",
			arg:     "bogus",
			want:    permissiontypes.PRINCIPAL_TYPE_UNSPECIFIED,
			wantErr: gnfderrors.ErrInvalidPrincipalType,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.GetPrincipalType(tt.arg)
			require.Equal(t, tt.want, got)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetPrincipal(t *testing.T) {
	hexAddr := sample.RandAccAddressHex()

	tests := []struct {
		name string
		arg  string
		want permissiontypes.Principal
	}{
		{
			name: "hex address resolves to an account principal",
			arg:  hexAddr,
			want: permissiontypes.Principal{
				Type:  permissiontypes.PRINCIPAL_TYPE_GNFD_ACCOUNT,
				Value: hexAddr,
			},
		},
		{
			name: "non-address string resolves to a group principal",
			arg:  "groupName",
			want: permissiontypes.Principal{
				Type:  permissiontypes.PRINCIPAL_TYPE_GNFD_GROUP,
				Value: "groupName",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.GetPrincipal(tt.arg)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// newKeyringWithAccount builds an in-memory keyring with a single funded key,
// mirroring the fixture tx_test.go's CLITestSuite.SetupSuite uses.
func newKeyringWithAccount(t *testing.T) (keyring.Keyring, testutil.TestAccount) {
	encCfg := encoding.MakeConfig()
	kr := keyring.NewInMemory(encCfg.Codec)
	accounts := testutil.CreateKeyringAccounts(t, kr, 1)
	return kr, accounts[0]
}

func TestGetPrimarySPField(t *testing.T) {
	kr, account := newKeyringWithAccount(t)

	tests := []struct {
		name     string
		arg      string
		wantAddr sdk.AccAddress
		wantName string
		wantType keyring.KeyType
		wantErr  bool
	}{
		{
			name: "empty string returns zero values",
			arg:  "",
		},
		{
			name:     "resolves by address",
			arg:      account.Address.String(),
			wantAddr: account.Address,
			wantName: account.Name,
			wantType: keyring.TypeLocal,
		},
		{
			name:     "resolves by key name",
			arg:      account.Name,
			wantAddr: account.Address,
			wantName: account.Name,
			wantType: keyring.TypeLocal,
		},
		{
			name:    "hex address not found in keyring",
			arg:     sample.RandAccAddressHex(),
			wantErr: true,
		},
		{
			name:    "key name not found in keyring",
			arg:     "no-such-key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			addr, name, kt, err := cli.GetPrimarySPField(kr, tt.arg)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, addr)
				require.Empty(t, name)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantAddr, addr)
			require.Equal(t, tt.wantName, name)
			require.Equal(t, tt.wantType, kt)
		})
	}
}

func TestGetPaymentAccountField(t *testing.T) {
	kr, account := newKeyringWithAccount(t)

	tests := []struct {
		name     string
		arg      string
		wantAddr sdk.AccAddress
		wantName string
		wantType keyring.KeyType
		wantErr  bool
	}{
		{
			name: "empty string returns zero values",
			arg:  "",
		},
		{
			name:     "resolves by address",
			arg:      account.Address.String(),
			wantAddr: account.Address,
			wantName: account.Name,
			wantType: keyring.TypeLocal,
		},
		{
			name:     "resolves by key name",
			arg:      account.Name,
			wantAddr: account.Address,
			wantName: account.Name,
			wantType: keyring.TypeLocal,
		},
		{
			name:    "hex address not found in keyring",
			arg:     sample.RandAccAddressHex(),
			wantErr: true,
		},
		{
			name:    "key name not found in keyring",
			arg:     "no-such-key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			addr, name, kt, err := cli.GetPaymentAccountField(kr, tt.arg)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, addr)
				require.Empty(t, name)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantAddr, addr)
			require.Equal(t, tt.wantName, name)
			require.Equal(t, tt.wantType, kt)
		})
	}
}

func TestGetTags(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want *types.ResourceTags
	}{
		{
			name: "empty string returns nil",
			arg:  "",
			want: nil,
		},
		{
			name: "empty braces returns nil",
			arg:  "{}",
			want: nil,
		},
		{
			name: "braces are stripped",
			arg:  "{alpha=one,beta=two}",
			want: &types.ResourceTags{
				Tags: []types.ResourceTags_Tag{
					{Key: "alpha", Value: "one"},
					{Key: "beta", Value: "two"},
				},
			},
		},
		{
			name: "no braces still parses",
			arg:  "gamma=three",
			want: &types.ResourceTags{
				Tags: []types.ResourceTags_Tag{
					{Key: "gamma", Value: "three"},
				},
			},
		},
		{
			name: "malformed pair without an equals sign is skipped",
			arg:  "delta=four,badpair",
			want: &types.ResourceTags{
				Tags: []types.ResourceTags_Tag{
					{Key: "delta", Value: "four"},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, cli.GetTags(tt.arg))
		})
	}
}
