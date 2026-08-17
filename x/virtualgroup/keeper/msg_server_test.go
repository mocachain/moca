package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/evmos/evmos/v12/testutil/sample"
	sptypes "github.com/evmos/evmos/v12/x/sp/types"
	"github.com/evmos/evmos/v12/x/virtualgroup/keeper"
	"github.com/evmos/evmos/v12/x/virtualgroup/types"
)

// expectedSecondaries is what the storage params report for a request at the time it
// is made; every case below asks for a group of this size.
const expectedSecondaries = 6

// dupCheckFixture sets up a family holding one group with the given secondaries and
// returns a msgServer plus a function that requests a new group in that family.
type dupCheckFixture struct {
	create func(secondarySpIDs []uint32) error
	family *types.GlobalVirtualGroupFamily
}

func (s *TestSuite) newDupCheckFixture(stored []uint32) *dupCheckFixture {
	ctrl := gomock.NewController(s.T())
	storageKeeper := types.NewMockStorageKeeper(ctrl)
	s.virtualgroupKeeper.SetStorageKeeper(storageKeeper)
	storageKeeper.EXPECT().GetExpectSecondarySPNumForECObject(gomock.Any(), gomock.Any()).
		Return(uint32(expectedSecondaries)).AnyTimes()

	spOperator := sample.RandAccAddress()
	primarySP := &sptypes.StorageProvider{
		Id:              1,
		Status:          sptypes.STATUS_IN_SERVICE,
		OperatorAddress: spOperator.String(),
		FundingAddress:  sample.RandAccAddress().String(),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(primarySP, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, id uint32) (*sptypes.StorageProvider, bool) {
			return &sptypes.StorageProvider{Id: id, Status: sptypes.STATUS_IN_SERVICE}, true
		}).AnyTimes()
	s.bankKeeper.EXPECT().
		SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	family := &types.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: primarySP.Id}
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id:             1000,
		FamilyId:       family.Id,
		PrimarySpId:    primarySP.Id,
		SecondarySpIds: stored,
	})
	family.GlobalVirtualGroupIds = append(family.GlobalVirtualGroupIds, 1000)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, family)

	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	return &dupCheckFixture{
		family: family,
		create: func(secondarySpIDs []uint32) error {
			_, err := msgServer.CreateGlobalVirtualGroup(s.ctx, &types.MsgCreateGlobalVirtualGroup{
				StorageProvider: spOperator.String(),
				FamilyId:        family.Id,
				SecondarySpIds:  secondarySpIDs,
				Deposit:         sdk.NewCoin(s.virtualgroupKeeper.DepositDenomForGVG(s.ctx), math.NewInt(1)),
			})
			return err
		},
	}
}

// A stored group whose secondary list merely starts with the requested one is a
// different group, not a duplicate. The check matched only as far as the request
// reached, so a longer stored group blocked a legitimate creation.
func (s *TestSuite) TestCreateGlobalVirtualGroup_LongerStoredGroupIsNotADuplicate() {
	f := s.newDupCheckFixture([]uint32{2, 3, 4, 5, 6, 7, 8, 9})

	require.NoError(s.T(), f.create([]uint32{2, 3, 4, 5, 6, 7}),
		"a longer stored group is not a duplicate of the requested one")

	stored, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, f.family.Id)
	require.True(s.T(), found)
	require.Len(s.T(), stored.GlobalVirtualGroupIds, 2)
}

// An identical secondary list in the same order is still rejected.
func (s *TestSuite) TestCreateGlobalVirtualGroup_IdenticalGroupIsStillADuplicate() {
	f := s.newDupCheckFixture([]uint32{2, 3, 4, 5, 6, 7})

	require.ErrorIs(s.T(), f.create([]uint32{2, 3, 4, 5, 6, 7}), types.ErrDuplicateGVG)
}

// Order is part of a group's identity: the same SPs in a different order is a
// different group and stays allowed. Without this, sorting either list before the
// comparison would still satisfy the two tests above.
func (s *TestSuite) TestCreateGlobalVirtualGroup_PermutationIsNotADuplicate() {
	f := s.newDupCheckFixture([]uint32{2, 3, 4, 5, 6, 7})

	require.NoError(s.T(), f.create([]uint32{7, 6, 5, 4, 3, 2}),
		"the same SPs in a different order are a different group")

	stored, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, f.family.Id)
	require.True(s.T(), found)
	require.Len(s.T(), stored.GlobalVirtualGroupIds, 2)
}

// The complement of the longer-stored case: a stored group shorter than the request,
// whose entries all match, is also a different group. This holds on both sides of the
// change, and pins the direction a prefix-style comparison would get wrong.
func (s *TestSuite) TestCreateGlobalVirtualGroup_ShorterStoredGroupIsNotADuplicate() {
	f := s.newDupCheckFixture([]uint32{2, 3})

	require.NoError(s.T(), f.create([]uint32{2, 3, 4, 5, 6, 7}),
		"a shorter stored group is not a duplicate of the requested one")

	stored, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, f.family.Id)
	require.True(s.T(), found)
	require.Len(s.T(), stored.GlobalVirtualGroupIds, 2)
}

// MOCA-1207: a family ID is caller-supplied, so without an ownership check any
// in-service SP could attach a new GVG to a rival's family -- planting groups that
// count against the rival's MaxGlobalVirtualGroupNumPerFamily cap and land under a
// PrimarySpId the rival never agreed to serve alongside. A rival SP must be
// rejected; the real owner must still be able to create in its own family.
func (s *TestSuite) TestCreateGlobalVirtualGroup_RejectsForeignFamily() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := types.NewMockStorageKeeper(ctrl)
	s.virtualgroupKeeper.SetStorageKeeper(storageKeeper)
	storageKeeper.EXPECT().GetExpectSecondarySPNumForECObject(gomock.Any(), gomock.Any()).
		Return(uint32(expectedSecondaries)).AnyTimes()

	owner := sample.RandAccAddress()
	rival := sample.RandAccAddress()
	ownerSP := &sptypes.StorageProvider{
		Id: 1, Status: sptypes.STATUS_IN_SERVICE,
		OperatorAddress: owner.String(), FundingAddress: sample.RandAccAddress().String(),
	}
	rivalSP := &sptypes.StorageProvider{
		Id: 2, Status: sptypes.STATUS_IN_SERVICE,
		OperatorAddress: rival.String(), FundingAddress: sample.RandAccAddress().String(),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, addr sdk.AccAddress) (*sptypes.StorageProvider, bool) {
			switch addr.String() {
			case owner.String():
				return ownerSP, true
			case rival.String():
				return rivalSP, true
			default:
				return nil, false
			}
		}).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, id uint32) (*sptypes.StorageProvider, bool) {
			return &sptypes.StorageProvider{Id: id, Status: sptypes.STATUS_IN_SERVICE}, true
		}).AnyTimes()
	s.bankKeeper.EXPECT().
		SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	// family 7 belongs to the owner (mirrors the confirmed repro's family 7/SP 1/SP 2).
	family := &types.GlobalVirtualGroupFamily{Id: 7, PrimarySpId: ownerSP.Id}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, family)

	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	secondarySpIDs := []uint32{11, 12, 13, 14, 15, 16}
	deposit := sdk.NewCoin(s.virtualgroupKeeper.DepositDenomForGVG(s.ctx), math.NewInt(1))

	_, err := msgServer.CreateGlobalVirtualGroup(s.ctx, &types.MsgCreateGlobalVirtualGroup{
		StorageProvider: rival.String(),
		FamilyId:        family.Id,
		SecondarySpIds:  secondarySpIDs,
		Deposit:         deposit,
	})
	require.ErrorIs(s.T(), err, types.ErrGVGFamilyNotOwned, "a rival SP must not plant a group in someone else's family")

	// the rejected attempt must not have mutated the family at all.
	stored, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, family.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), ownerSP.Id, stored.PrimarySpId)
	require.Empty(s.T(), stored.GlobalVirtualGroupIds)

	_, err = msgServer.CreateGlobalVirtualGroup(s.ctx, &types.MsgCreateGlobalVirtualGroup{
		StorageProvider: owner.String(),
		FamilyId:        family.Id,
		SecondarySpIds:  secondarySpIDs,
		Deposit:         deposit,
	})
	require.NoError(s.T(), err, "the real owner must still be able to create in its own family")

	stored, found = s.virtualgroupKeeper.GetGVGFamily(s.ctx, family.Id)
	require.True(s.T(), found)
	require.Len(s.T(), stored.GlobalVirtualGroupIds, 1)

	gvg, found := s.virtualgroupKeeper.GetGVG(s.ctx, stored.GlobalVirtualGroupIds[0])
	require.True(s.T(), found)
	require.Equal(s.T(), ownerSP.Id, gvg.PrimarySpId)
	require.Equal(s.T(), family.Id, gvg.FamilyId)
}

// MOCA-1207 guardrail: SwapIn is a flow that must legitimately load a family the
// caller does not own -- a successor SP reserves a *target* SP's family while that
// SP is exiting. The fix in GetOrCreateEmptyGVGFamily must not touch this path
// (SwapIn reads via the separate, unchecked GetGVGFamily and does its own explicit
// family.PrimarySpId == targetSP.Id check), so this must keep succeeding.
func (s *TestSuite) TestSwapIn_SuccessorReservesFamilyItDoesNotOwn() {
	const (
		targetSPID    = uint32(1) // owns the family, currently exiting
		successorSPID = uint32(2) // does not own it -- taking it over
		familyID      = uint32(9)
	)
	family := &types.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: targetSPID}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, family)

	targetSP := &sptypes.StorageProvider{Id: targetSPID, Status: sptypes.STATUS_GRACEFUL_EXITING}
	expiration := s.ctx.BlockTime().Unix() + 100

	err := s.virtualgroupKeeper.SwapIn(s.ctx, familyID, 0, successorSPID, targetSP, expiration)
	require.NoError(s.T(), err, "a successor must still be able to reserve a family it does not own during SP exit")

	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, familyID, 0)
	require.True(s.T(), found)
	require.Equal(s.T(), successorSPID, info.SuccessorSpId)
	require.Equal(s.T(), targetSPID, info.TargetSpId)
}

// A primary SP that lists itself as one of its own secondaries used to be accepted.
// Its statistics record was then fetched twice within the same request -- once to
// bump PrimaryCount, once more inside the secondary loop, since nothing had been
// persisted to the store yet -- and the trailing batch write is last-write-wins, so
// the PrimaryCount increment was silently dropped even though a real GVG now existed
// with this SP as primary. See MOCA-1072.
func (s *TestSuite) TestCreateGlobalVirtualGroup_PrimaryCanNotBeItsOwnSecondary() {
	f := s.newDupCheckFixture([]uint32{2, 3, 4, 5, 6, 7})

	// primarySP inside the fixture is SP id 1; list it as one of the 6 secondaries.
	err := f.create([]uint32{10, 11, 12, 13, 14, 1})
	require.ErrorIs(s.T(), err, types.ErrDuplicateSecondarySP)

	// Nothing should have been persisted: no new GVG in the family, and no
	// statistics record was created or mutated for the primary SP.
	stored, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, f.family.Id)
	require.True(s.T(), found)
	require.Len(s.T(), stored.GlobalVirtualGroupIds, 1,
		"only the pre-seeded GVG remains; the self-secondary request must not create a new one")

	_, found = s.virtualgroupKeeper.GetGVGStatisticsWithinSP(s.ctx, 1)
	require.False(s.T(), found,
		"the rejected request must not create a statistics record for the primary SP")
}
