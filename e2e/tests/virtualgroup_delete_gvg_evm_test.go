package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestVirtualGroupDeleteGvgEvmFlow was meant to drive x/virtualgroup's GVG
// deletion through the virtualgroup precompile's deleteGlobalVirtualGroup
// method (create a fresh, empty GVG via setupPrimarySP, delete it
// immediately, assert the deposit refunds to the owning SP).
//
// BLOCKED: a freshly-created GVG is not actually "empty" in the sense
// deleteGlobalVirtualGroup requires. Verified live: right after creation,
// the GVG's virtual payment account already carries a positive
// netflow_rate (e.g. 2516582400), not zero, even with StoredSize still 0 --
// so DeleteGlobalVirtualGroup's k.paymentKeeper.IsEmptyNetFlow check
// rejects it with "the store size of gvg is not zero" (a reused, somewhat
// misleading error registered originally for the StoredSize check, also
// wrapped onto the netflow-rate check). This rate appears to represent an
// ongoing commitment to the family's secondary SPs from the moment of
// creation, not something tied to actual stored objects.
//
// Unwinding it back to zero isn't a single precompile call: swapOut
// requires negotiating a specific successor SP to take over each GVG
// (msgServer.SwapOut resolves the caller by operator/funding address and
// expects a target family + successor, not a simple self-service exit), and
// the fuller SP-exit lifecycle (spExit -> completeSPExit) changes the SP's
// own registration status too. Actually reaching a deletable GVG needs that
// multi-step, multi-SP flow modeled end-to-end -- out of scope for this
// pass; worth its own investigation.
func TestVirtualGroupDeleteGvgEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	vgClient := virtualgroupmoduletypes.NewQueryClient(conn)
	paymentClient := paymenttypes.NewQueryClient(conn)

	sp, familyID := setupPrimarySP(t, ctx, client, conn, chainID)

	familyResp, err := vgClient.GlobalVirtualGroupFamily(ctx, &virtualgroupmoduletypes.QueryGlobalVirtualGroupFamilyRequest{FamilyId: familyID})
	require.NoError(t, err)
	gvgID := familyResp.GlobalVirtualGroupFamily.GlobalVirtualGroupIds[0]

	gvgResp, err := vgClient.GlobalVirtualGroup(ctx, &virtualgroupmoduletypes.QueryGlobalVirtualGroupRequest{GlobalVirtualGroupId: gvgID})
	require.NoError(t, err)

	streamResp, err := paymentClient.StreamRecord(ctx, &paymenttypes.QueryGetStreamRecordRequest{Account: gvgResp.GlobalVirtualGroup.VirtualPaymentAddress})
	require.NoError(t, err)
	require.False(t, streamResp.StreamRecord.NetflowRate.IsZero(),
		"reproduces the blocker: a freshly-created GVG already has a non-zero netflow rate, sp %d", sp.SPID)

	t.Skip("BLOCKED: deleteGlobalVirtualGroup requires the GVG's netflow rate at zero, " +
		"which a freshly-created GVG never has -- reaching that state needs the full " +
		"swapOut/spExit lifecycle modeled end-to-end, see comment above")
}
