package simulation_test

import (
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/storage/keeper"
	"github.com/mocachain/moca/v2/x/storage/simulation"
	"github.com/mocachain/moca/v2/x/storage/types"
)

func TestSimulateMsgCancelMigrateBucket(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	accs := simtypes.RandomAccounts(r, 1)

	// The three dependencies are never touched by the returned operation (it
	// is an unimplemented stub), so nil/zero-value stand-ins are enough.
	op := simulation.SimulateMsgCancelMigrateBucket(nil, nil, keeper.Keeper{})

	opMsg, futureOps, err := op(r, nil, sdk.Context{}, accs, "")

	require.NoError(t, err)
	require.Empty(t, futureOps)
	require.False(t, opMsg.OK)
	require.Equal(t, types.ModuleName, opMsg.Route)
	require.Equal(t, types.TypeMsgCancelMigrateBucket, opMsg.Name)
	require.Contains(t, opMsg.Comment, "not implemented")
}
