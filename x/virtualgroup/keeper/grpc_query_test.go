package keeper_test

import (
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mocachain/moca/v2/testutil/sample"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// TestGRPCQuery_NilRequest asserts every query RPC rejects a nil request with the
// same InvalidArgument code, before touching any keeper state.
func (s *TestSuite) TestGRPCQuery_NilRequest() {
	cases := []struct {
		name string
		call func() error
	}{
		{"Params", func() error { _, err := s.virtualgroupKeeper.Params(s.ctx, nil); return err }},
		{"GlobalVirtualGroup", func() error { _, err := s.virtualgroupKeeper.GlobalVirtualGroup(s.ctx, nil); return err }},
		{"GlobalVirtualGroupByFamilyID", func() error {
			_, err := s.virtualgroupKeeper.GlobalVirtualGroupByFamilyID(s.ctx, nil)
			return err
		}},
		{"GlobalVirtualGroupFamily", func() error {
			_, err := s.virtualgroupKeeper.GlobalVirtualGroupFamily(s.ctx, nil)
			return err
		}},
		{"GlobalVirtualGroupFamilies", func() error {
			_, err := s.virtualgroupKeeper.GlobalVirtualGroupFamilies(s.ctx, nil)
			return err
		}},
		{"AvailableGlobalVirtualGroupFamilies", func() error {
			_, err := s.virtualgroupKeeper.AvailableGlobalVirtualGroupFamilies(s.ctx, nil)
			return err
		}},
		{"SwapInInfo", func() error { _, err := s.virtualgroupKeeper.SwapInInfo(s.ctx, nil); return err }},
		{"GVGStatistics", func() error { _, err := s.virtualgroupKeeper.GVGStatistics(s.ctx, nil); return err }},
		{"QuerySpAvailableGlobalVirtualGroupFamilies", func() error {
			_, err := s.virtualgroupKeeper.QuerySpAvailableGlobalVirtualGroupFamilies(s.ctx, nil)
			return err
		}},
		{"QuerySpOptimalGlobalVirtualGroupFamily", func() error {
			_, err := s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, nil)
			return err
		}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Require().Equal(codes.InvalidArgument, status.Code(tc.call()))
		})
	}
}

func (s *TestSuite) TestParams() {
	resp, err := s.virtualgroupKeeper.Params(s.ctx, &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().Equal(types.DefaultParams(), resp.Params)
}

func (s *TestSuite) TestGlobalVirtualGroup() {
	_, err := s.virtualgroupKeeper.GlobalVirtualGroup(s.ctx, &types.QueryGlobalVirtualGroupRequest{GlobalVirtualGroupId: 999999})
	s.Require().ErrorIs(err, types.ErrGVGNotExist)

	gvg := &types.GlobalVirtualGroup{
		Id:                    1,
		FamilyId:              100,
		PrimarySpId:           10,
		SecondarySpIds:        []uint32{20, 30},
		StoredSize:            100,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
		TotalDeposit:          math.NewInt(3_200_000),
	}
	s.virtualgroupKeeper.SetGVG(s.ctx, gvg)

	resp, err := s.virtualgroupKeeper.GlobalVirtualGroup(s.ctx, &types.QueryGlobalVirtualGroupRequest{GlobalVirtualGroupId: 1})
	s.Require().NoError(err)
	s.Require().Equal(gvg, resp.GlobalVirtualGroup)
}

func (s *TestSuite) TestGlobalVirtualGroupByFamilyID() {
	_, err := s.virtualgroupKeeper.GlobalVirtualGroupByFamilyID(s.ctx,
		&types.QueryGlobalVirtualGroupByFamilyIDRequest{GlobalVirtualGroupFamilyId: 99999})
	s.Require().ErrorIs(err, types.ErrGVGFamilyNotExist)

	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 1, FamilyId: 100, VirtualPaymentAddress: sample.RandAccAddress().String(), TotalDeposit: math.ZeroInt(),
	})
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: 2, FamilyId: 100, VirtualPaymentAddress: sample.RandAccAddress().String(), TotalDeposit: math.ZeroInt(),
	})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 100, PrimarySpId: 10, GlobalVirtualGroupIds: []uint32{1, 2}})

	resp, err := s.virtualgroupKeeper.GlobalVirtualGroupByFamilyID(s.ctx,
		&types.QueryGlobalVirtualGroupByFamilyIDRequest{GlobalVirtualGroupFamilyId: 100})
	s.Require().NoError(err)
	s.Require().Len(resp.GlobalVirtualGroups, 2)

	// A family referencing a GVG id that was never stored violates the store's own
	// invariant; the handler panics rather than silently returning a partial list.
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: 300, PrimarySpId: 10, GlobalVirtualGroupIds: []uint32{999}})
	s.Require().Panics(func() {
		_, _ = s.virtualgroupKeeper.GlobalVirtualGroupByFamilyID(s.ctx,
			&types.QueryGlobalVirtualGroupByFamilyIDRequest{GlobalVirtualGroupFamilyId: 300})
	})
}

func (s *TestSuite) TestGlobalVirtualGroupFamily() {
	_, err := s.virtualgroupKeeper.GlobalVirtualGroupFamily(s.ctx, &types.QueryGlobalVirtualGroupFamilyRequest{FamilyId: 99999})
	s.Require().ErrorIs(err, types.ErrGVGFamilyNotExist)

	family := &types.GlobalVirtualGroupFamily{Id: 100, PrimarySpId: 10, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, family)

	resp, err := s.virtualgroupKeeper.GlobalVirtualGroupFamily(s.ctx, &types.QueryGlobalVirtualGroupFamilyRequest{FamilyId: 100})
	s.Require().NoError(err)
	s.Require().Equal(family, resp.GlobalVirtualGroupFamily)
}

func (s *TestSuite) TestGlobalVirtualGroupFamilies() {
	for i := uint32(1); i <= 3; i++ {
		s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: i, PrimarySpId: 10, VirtualPaymentAddress: sample.RandAccAddress().String()})
	}

	resp, err := s.virtualgroupKeeper.GlobalVirtualGroupFamilies(s.ctx, &types.QueryGlobalVirtualGroupFamiliesRequest{})
	s.Require().NoError(err)
	s.Require().Len(resp.GvgFamilies, 3)

	_, err = s.virtualgroupKeeper.GlobalVirtualGroupFamilies(s.ctx, &types.QueryGlobalVirtualGroupFamiliesRequest{
		Pagination: &query.PageRequest{Key: []byte{0x01}, Offset: 1}, // offset + key together is invalid
	})
	s.Require().Error(err)
}

// seedAvailabilityFixtures sets up three GVG families sharing the same primary SP:
//   - 100 has spare capacity (available)
//   - 200 is fully utilized (not available)
//   - 300 references a GVG id that was never stored, to drive the not-found error
//     path shared by the "available families" handlers below.
func (s *TestSuite) seedAvailabilityFixtures() {
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id:                    1,
		FamilyId:              100,
		StoredSize:            100,
		TotalDeposit:          math.NewInt(3_200_000), // 200 bytes staked, 100 used -> spare capacity
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 100, PrimarySpId: 10, GlobalVirtualGroupIds: []uint32{1}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})

	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id:                    2,
		FamilyId:              200,
		StoredSize:            500,
		TotalDeposit:          math.NewInt(8_000_000), // 500 bytes staked, 500 used -> no spare capacity
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 200, PrimarySpId: 10, GlobalVirtualGroupIds: []uint32{2}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})

	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: 300, PrimarySpId: 10, GlobalVirtualGroupIds: []uint32{999}, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
}

func (s *TestSuite) TestAvailableGlobalVirtualGroupFamilies() {
	_, err := s.virtualgroupKeeper.AvailableGlobalVirtualGroupFamilies(s.ctx, &types.AvailableGlobalVirtualGroupFamiliesRequest{
		GlobalVirtualGroupFamilyIds: []uint32{99999},
	})
	s.Require().ErrorIs(err, types.ErrGVGFamilyNotExist)

	s.seedAvailabilityFixtures()

	_, err = s.virtualgroupKeeper.AvailableGlobalVirtualGroupFamilies(s.ctx, &types.AvailableGlobalVirtualGroupFamiliesRequest{
		GlobalVirtualGroupFamilyIds: []uint32{300},
	})
	s.Require().ErrorIs(err, types.ErrGVGNotExist)

	resp, err := s.virtualgroupKeeper.AvailableGlobalVirtualGroupFamilies(s.ctx, &types.AvailableGlobalVirtualGroupFamiliesRequest{
		GlobalVirtualGroupFamilyIds: []uint32{100, 200},
	})
	s.Require().NoError(err)
	s.Require().Equal([]uint32{100}, resp.GlobalVirtualGroupFamilyIds)
}

func (s *TestSuite) TestSwapInInfo() {
	_, err := s.virtualgroupKeeper.SwapInInfo(s.ctx, &types.QuerySwapInInfoRequest{GlobalVirtualGroupFamilyId: 100})
	s.Require().ErrorIs(err, types.ErrSwapInInfoNotExist)

	// Family-scoped swap (globalVirtualGroupFamilyID specified).
	family := &types.GlobalVirtualGroupFamily{Id: 100, PrimarySpId: 55, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, family)
	familyTargetSP := &sptypes.StorageProvider{Id: 55, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.Require().NoError(s.virtualgroupKeeper.SwapIn(s.ctx, 100, 0, 66, familyTargetSP, 12345))

	resp, err := s.virtualgroupKeeper.SwapInInfo(s.ctx, &types.QuerySwapInInfoRequest{GlobalVirtualGroupFamilyId: 100})
	s.Require().NoError(err)
	s.Require().Equal(uint32(66), resp.SwapInInfo.SuccessorSpId)

	// GVG-scoped swap (globalVirtualGroupFamilyID left unspecified).
	gvg := &types.GlobalVirtualGroup{
		Id: 5, PrimarySpId: 1, SecondarySpIds: []uint32{2, 3},
		TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualgroupKeeper.SetGVG(s.ctx, gvg)
	gvgTargetSP := &sptypes.StorageProvider{Id: 2, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.Require().NoError(s.virtualgroupKeeper.SwapIn(s.ctx, types.NoSpecifiedFamilyID, 5, 9, gvgTargetSP, 999))

	resp, err = s.virtualgroupKeeper.SwapInInfo(s.ctx, &types.QuerySwapInInfoRequest{GlobalVirtualGroupId: 5})
	s.Require().NoError(err)
	s.Require().Equal(uint32(9), resp.SwapInInfo.SuccessorSpId)
}

func (s *TestSuite) TestGVGStatistics() {
	_, err := s.virtualgroupKeeper.GVGStatistics(s.ctx, &types.QuerySPGVGStatisticsRequest{SpId: 42})
	s.Require().ErrorIs(err, types.ErrGVGStatisticsNotExist)

	stats := &types.GVGStatisticsWithinSP{StorageProviderId: 42, PrimaryCount: 2, SecondaryCount: 3}
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, stats)

	resp, err := s.virtualgroupKeeper.GVGStatistics(s.ctx, &types.QuerySPGVGStatisticsRequest{SpId: 42})
	s.Require().NoError(err)
	s.Require().Equal(stats, resp.GvgStats)
}

func (s *TestSuite) TestQuerySpAvailableGlobalVirtualGroupFamilies() {
	const spID = uint32(10)

	_, err := s.virtualgroupKeeper.QuerySpAvailableGlobalVirtualGroupFamilies(s.ctx,
		&types.QuerySPAvailableGlobalVirtualGroupFamiliesRequest{SpId: spID})
	s.Require().ErrorIs(err, types.ErrGVGFamilyStatisticsNotExist)

	s.seedAvailabilityFixtures()

	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: spID, GlobalVirtualGroupFamilyIds: []uint32{300},
	})
	_, err = s.virtualgroupKeeper.QuerySpAvailableGlobalVirtualGroupFamilies(s.ctx,
		&types.QuerySPAvailableGlobalVirtualGroupFamiliesRequest{SpId: spID})
	s.Require().ErrorIs(err, types.ErrGVGNotExist)

	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: spID, GlobalVirtualGroupFamilyIds: []uint32{99999},
	})
	_, err = s.virtualgroupKeeper.QuerySpAvailableGlobalVirtualGroupFamilies(s.ctx,
		&types.QuerySPAvailableGlobalVirtualGroupFamiliesRequest{SpId: spID})
	s.Require().ErrorIs(err, types.ErrGVGFamilyNotExist)

	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: spID, GlobalVirtualGroupFamilyIds: []uint32{100, 200},
	})
	resp, err := s.virtualgroupKeeper.QuerySpAvailableGlobalVirtualGroupFamilies(s.ctx,
		&types.QuerySPAvailableGlobalVirtualGroupFamiliesRequest{SpId: spID})
	s.Require().NoError(err)
	s.Require().Equal([]uint32{100}, resp.GlobalVirtualGroupFamilyIds)
}

func (s *TestSuite) TestQuerySpOptimalGlobalVirtualGroupFamily() {
	const spID = uint32(11)

	_, err := s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx,
		&types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{SpId: spID})
	s.Require().ErrorIs(err, types.ErrGVGFamilyStatisticsNotExist)

	s.seedAvailabilityFixtures()
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: spID, GlobalVirtualGroupFamilyIds: []uint32{100, 200},
	})

	resp, err := s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Maximize_Free_Store_Size,
	})
	s.Require().NoError(err)
	s.Require().Equal(uint32(100), resp.GlobalVirtualGroupFamilyId) // 100 has the only free space

	resp, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Minimal_Free_Store_Size,
	})
	s.Require().NoError(err)
	s.Require().Equal(uint32(200), resp.GlobalVirtualGroupFamilyId) // 200 has zero free space

	resp, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Oldest_Create_Time,
	})
	s.Require().NoError(err)
	s.Require().Equal(uint32(100), resp.GlobalVirtualGroupFamilyId)

	resp, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Recentest_Create_Time,
	})
	s.Require().NoError(err)
	s.Require().Equal(uint32(200), resp.GlobalVirtualGroupFamilyId)

	_, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.PickVGFStrategy(99),
	})
	s.Require().Error(err)

	// A family id in the sp's statistics that was never stored as a GVGFamily hits
	// the not-found branch inside both the Maximize and Minimal loops.
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: spID, GlobalVirtualGroupFamilyIds: []uint32{99999},
	})
	_, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Maximize_Free_Store_Size,
	})
	s.Require().ErrorIs(err, types.ErrGVGFamilyNotExist)
	_, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Minimal_Free_Store_Size,
	})
	s.Require().ErrorIs(err, types.ErrGVGFamilyNotExist)

	// A family referencing a GVG id that was never stored surfaces the
	// GetGlobalVirtualFamilyTotalStakingAndStoredSize error from both loops too.
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: spID, GlobalVirtualGroupFamilyIds: []uint32{300},
	})
	_, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Maximize_Free_Store_Size,
	})
	s.Require().ErrorIs(err, types.ErrGVGNotExist)
	_, err = s.virtualgroupKeeper.QuerySpOptimalGlobalVirtualGroupFamily(s.ctx, &types.QuerySpOptimalGlobalVirtualGroupFamilyRequest{
		SpId: spID, PickVgfStrategy: types.Strategy_Minimal_Free_Store_Size,
	})
	s.Require().ErrorIs(err, types.ErrGVGNotExist)
}
