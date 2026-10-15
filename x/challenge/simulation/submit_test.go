package simulation_test

import (
	"math/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/challenge/keeper"
	"github.com/mocachain/moca/v2/x/challenge/simulation"
)

// SimulateMsgSubmit's body is a `// TODO` stub: it never dereferences its
// account/bank/challenge keeper arguments or the operation closure's
// BaseApp/Context arguments, so nil/zero-value stand-ins are safe here. This
// only asserts the documented no-op shape, not any real simulation logic.
func TestSimulateMsgSubmit(t *testing.T) {
	r := rand.New(rand.NewSource(1)) //nolint:gosec
	accs := simtypes.RandomAccounts(r, 3)

	op := simulation.SimulateMsgSubmit(nil, nil, keeper.Keeper{})
	opMsg, futureOps, err := op(r, nil, sdk.Context{}, accs, "")

	require.NoError(t, err)
	require.Nil(t, futureOps)
	require.False(t, opMsg.OK)
	require.Contains(t, opMsg.Comment, "not implemented")
}
