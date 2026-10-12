package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// caseValid labels the happy-path row in the table-driven tests in this file
// and in params_test.go, which share this package.
const caseValid = "valid"

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
			name:    caseValid,
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
