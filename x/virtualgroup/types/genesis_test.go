package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultGenesis(t *testing.T) {
	gs := DefaultGenesis()
	require.NotNil(t, gs)
	require.Equal(t, DefaultParams(), gs.Params)
}

func TestGenesisStateValidate(t *testing.T) {
	tests := []struct {
		name    string
		genesis *GenesisState
		wantErr bool
	}{
		{
			name:    "valid",
			genesis: DefaultGenesis(),
		},
		{
			name:    "invalid params",
			genesis: &GenesisState{Params: Params{}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.genesis.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
