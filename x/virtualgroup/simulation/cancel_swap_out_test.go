package simulation_test

import (
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	"github.com/mocachain/moca/v2/x/virtualgroup/simulation"
)

func TestSimulateMsgCancelSwapOut(t *testing.T) {
	op := simulation.SimulateMsgCancelSwapOut(nil, nil, keeper.Keeper{})
	require.NotNil(t, op)

	r := rand.New(rand.NewSource(1)) //nolint:gosec
	accs := simtypes.RandomAccounts(r, 1)

	opMsg, futureOps, err := op(r, nil, sdk.Context{}, accs, "")
	require.NoError(t, err)
	require.Nil(t, futureOps)
	require.Contains(t, opMsg.Comment, "not implemented")
}
