package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/gensp/types"
)

func TestKeyPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"non-empty", "x"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, []byte(tc.prefix), types.KeyPrefix(tc.prefix))
		})
	}
}
