package keeper_test

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/virtualgroup/keeper"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
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
