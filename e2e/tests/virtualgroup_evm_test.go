package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/mocachain/moca/v2/precompiles/virtualgroup"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestVirtualGroupDepositEvmFlow drives x/virtualgroup's per-GVG deposit
// through the virtualgroup precompile's deposit method, exercising the
// deposit half of the retired suite's TestBasic. (There's no withdraw
// method on this precompile today -- MsgWithdraw from the retired suite has
// no EVM-facing equivalent yet -- so only the deposit side is covered here.)
func TestVirtualGroupDepositEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	vgClient := virtualgroupmoduletypes.NewQueryClient(conn)
	precompile := virtualgroup.Precompile{}

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	gvgsResp, err := vgClient.GlobalVirtualGroupByFamilyID(ctx, &virtualgroupmoduletypes.QueryGlobalVirtualGroupByFamilyIDRequest{
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, gvgsResp.GlobalVirtualGroups, "expected at least one GVG in the freshly created family")
	gvg := gvgsResp.GlobalVirtualGroups[0]
	depositBefore := gvg.TotalDeposit

	depositMethod, err := virtualgroup.GetMethod(virtualgroup.DepositMethodName)
	require.NoError(t, err)
	depositAmount := mustBigInt(t, oneMocaInAmoca) // 1 MOCA
	depositArgs, err := depositMethod.Inputs.Pack(
		gvg.Id,
		virtualgroup.Coin{Denom: "amoca", Amount: depositAmount},
	)
	require.NoError(t, err)
	sendPrecompileTx(t, ctx, client, chainID, sp.OperatorKey, precompile.Address(),
		append(append([]byte{}, depositMethod.ID...), depositArgs...))

	afterResp, err := vgClient.GlobalVirtualGroup(ctx, &virtualgroupmoduletypes.QueryGlobalVirtualGroupRequest{
		GlobalVirtualGroupId: gvg.Id,
	})
	require.NoError(t, err)
	wantDeposit := depositBefore.Add(sdkmath.NewIntFromBigInt(depositAmount))
	require.Equal(t, wantDeposit.String(), afterResp.GlobalVirtualGroup.TotalDeposit.String())
}
