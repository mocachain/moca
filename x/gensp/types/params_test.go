package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/mocachain/moca/v2/x/gensp/types"
)

func TestNewParams(t *testing.T) {
	require.Equal(t, types.Params{}, types.NewParams())
}

func TestDefaultParams(t *testing.T) {
	require.Equal(t, types.NewParams(), types.DefaultParams())
}

func TestParams_Validate(t *testing.T) {
	require.NoError(t, types.DefaultParams().Validate())
}

func TestParams_String(t *testing.T) {
	p := types.DefaultParams()
	out, err := yaml.Marshal(p)
	require.NoError(t, err)
	require.Equal(t, string(out), p.String())
}
