package keeper_test

import (
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestStorageProviderExitable asserts the exit precondition: an SP that is still the
// primary of a GVG family must not be exitable -- including the case where its GVG
// counts are already zero but a residual (empty) family still references it. That
// zero-count-with-residual-family case is the gap this guards against.
func (s *TestSuite) TestStorageProviderExitable() {
	const spID = uint32(7)

	// No GVG stats and no families -> exitable.
	s.Require().NoError(s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID))

	// GVG counts are zero, but a family still references the SP -> NOT exitable.
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{
		StorageProviderId: spID,
		PrimaryCount:      0,
		SecondaryCount:    0,
	})
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId:                        spID,
		GlobalVirtualGroupFamilyIds: []uint32{1},
	})
	s.Require().ErrorIs(
		s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID),
		types.ErrSPCanNotExit,
	)

	// Once the family is swapped out (empty list) -> exitable again.
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId:                        spID,
		GlobalVirtualGroupFamilyIds: []uint32{},
	})
	s.Require().NoError(s.virtualgroupKeeper.StorageProviderExitable(s.ctx, spID))
}
