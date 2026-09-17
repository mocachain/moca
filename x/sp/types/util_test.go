package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/sp/types"
)

func TestValidateEndpointURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty", "", true},
		{"unparseable", "http://\n", true},
		{"fully qualified path disallowed", "http://127.0.0.1:9033/foo", true},
		{"root path ok", "http://127.0.0.1:9033/", false},
		{"no path ok", "http://127.0.0.1:9033", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateEndpointURL(tc.url)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
