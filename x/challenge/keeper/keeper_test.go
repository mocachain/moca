package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	k, ctx := makeKeeper(t)
	require.NotNil(t, k.Logger(ctx))
}
