package keeper_test

import (
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// Settle takes a caller-supplied list of GVG ids and errors on the first one it
// cannot find. If the ids are visited in a random order, a request naming both a
// settleable and an unknown GVG settles a varying number of them before failing,
// so the same message consumes different gas on different nodes. GasUsed feeds
// LastResultsHash, so that difference diverges the block hash across validators.
// Drive one such request repeatedly and require the gas to be identical every
// time; ranging the dedup map directly yields more than one total.
func (s *TestSuite) TestSettle_GasIsIndependentOfGVGIDOrder() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)

	existingGVGID := uint32(1)
	missingGVGID := uint32(999999)
	secondarySpID := uint32(10)

	gvg := &types.GlobalVirtualGroup{
		Id:                    existingGVGID,
		SecondarySpIds:        []uint32{secondarySpID},
		VirtualPaymentAddress: sample.RandAccAddress().String(),
		TotalDeposit:          math.ZeroInt(),
	}
	s.virtualgroupKeeper.SetGVG(s.ctx, gvg)

	sp := &sptypes.StorageProvider{Id: secondarySpID, FundingAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), secondarySpID).Return(sp, true).AnyTimes()
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.NewInt(100), nil).AnyTimes()
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	req := &types.MsgSettle{
		StorageProvider:       sample.RandAccAddress().String(),
		GlobalVirtualGroupIds: []uint32{existingGVGID, missingGVGID},
	}

	baseCtx := s.ctx
	gasReadings := make(map[uint64]int)
	const iterations = 200

	for i := 0; i < iterations; i++ {
		callCtx := baseCtx.WithGasMeter(storetypes.NewInfiniteGasMeter())
		_, err := msgServer.Settle(callCtx, req)
		s.Require().ErrorIs(err, types.ErrGVGNotExist)
		gasReadings[callCtx.GasMeter().GasConsumed()]++
	}

	s.Require().Lenf(gasReadings, 1,
		"the same MsgSettle must consume the same gas on every node; saw %d different totals over %d calls: %v",
		len(gasReadings), iterations, gasReadings)
}
