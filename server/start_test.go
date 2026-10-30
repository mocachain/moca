package server

import (
	"testing"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/server"
)

// TestStartCmdQueryGasLimitFlag checks that start accepts --query-gas-limit and
// binds it to the key the app creator reads, defaulting to 0 (unbounded).
func TestStartCmdQueryGasLimitFlag(t *testing.T) {
	cmd := StartCmd(NewDefaultStartOptions(nil, t.TempDir()))
	v := viper.New()
	require.NoError(t, v.BindPFlags(cmd.Flags()))
	require.Zero(t, cast.ToUint64(v.Get(server.FlagQueryGasLimit)))

	require.NoError(t, cmd.ParseFlags([]string{"--" + server.FlagQueryGasLimit, "50000"}))
	require.Equal(t, uint64(50000), cast.ToUint64(v.Get(server.FlagQueryGasLimit)))
}
