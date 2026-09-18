package keeper_test

import (
	"errors"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/types/common"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
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

// spWithOperator returns an in-service storage provider whose operator address is addr.
func spWithOperator(id uint32, addr sdk.AccAddress) *sptypes.StorageProvider {
	return &sptypes.StorageProvider{
		Id:              id,
		Status:          sptypes.STATUS_IN_SERVICE,
		OperatorAddress: addr.String(),
		FundingAddress:  sample.RandAccAddress().String(),
	}
}

// UpdateParams rejects a request from anyone but the module's own authority (x/gov).
func (s *TestSuite) TestUpdateParams_RejectsWrongAuthority() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	_, err := msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: sample.RandAccAddress().String(),
		Params:    types.DefaultParams(),
	})
	require.ErrorIs(s.T(), err, govtypes.ErrInvalidSigner)
}

// GvgStakingPerBytes is fixed at genesis and can never be changed by governance.
func (s *TestSuite) TestUpdateParams_RejectsGvgStakingPerBytesChange() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	newParams := types.DefaultParams()
	newParams.GvgStakingPerBytes = newParams.GvgStakingPerBytes.AddRaw(1)

	_, err := msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: s.virtualgroupKeeper.GetAuthority(),
		Params:    newParams,
	})
	require.ErrorIs(s.T(), err, gnfderrors.ErrInvalidParameter)

	got := s.virtualgroupKeeper.GetParams(s.ctx)
	require.True(s.T(), got.GvgStakingPerBytes.Equal(types.DefaultGVGStakingPerBytes), "rejected update must not mutate params")
}

// DepositDenom is likewise fixed and cannot be changed by governance.
func (s *TestSuite) TestUpdateParams_RejectsDepositDenomChange() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	newParams := types.DefaultParams()
	newParams.DepositDenom = "someotherdenom"

	_, err := msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: s.virtualgroupKeeper.GetAuthority(),
		Params:    newParams,
	})
	require.ErrorIs(s.T(), err, gnfderrors.ErrInvalidParameter)
}

// A syntactically-allowed but semantically invalid new param set (zero max-GVG-per-family)
// must be rejected by SetParams' own validation, not silently stored.
func (s *TestSuite) TestUpdateParams_RejectsInvalidNewParams() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	newParams := types.DefaultParams()
	newParams.MaxGlobalVirtualGroupNumPerFamily = 0

	_, err := msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: s.virtualgroupKeeper.GetAuthority(),
		Params:    newParams,
	})
	require.ErrorContains(s.T(), err, "max GVG per family")

	got := s.virtualgroupKeeper.GetParams(s.ctx)
	require.Equal(s.T(), types.DefaultMaxGlobalVirtualGroupNumPerFamily, got.MaxGlobalVirtualGroupNumPerFamily)
}

// A valid update from the real authority, touching only mutable fields, is persisted.
func (s *TestSuite) TestUpdateParams_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	newParams := types.DefaultParams()
	newParams.MaxGlobalVirtualGroupNumPerFamily = 42
	newParams.MaxStoreSizePerFamily = 123456

	_, err := msgServer.UpdateParams(s.ctx, &types.MsgUpdateParams{
		Authority: s.virtualgroupKeeper.GetAuthority(),
		Params:    newParams,
	})
	require.NoError(s.T(), err)

	got := s.virtualgroupKeeper.GetParams(s.ctx)
	require.Equal(s.T(), uint32(42), got.MaxGlobalVirtualGroupNumPerFamily)
	require.Equal(s.T(), uint64(123456), got.MaxStoreSizePerFamily)
}

func (s *TestSuite) TestDeleteGlobalVirtualGroup_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.DeleteGlobalVirtualGroup(s.ctx, &types.MsgDeleteGlobalVirtualGroup{
		StorageProvider:      sample.RandAccAddress().String(),
		GlobalVirtualGroupId: 1,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// A keeper-level failure (an unknown GVG id) is passed straight through.
func (s *TestSuite) TestDeleteGlobalVirtualGroup_KeeperErrorPropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.DeleteGlobalVirtualGroup(s.ctx, &types.MsgDeleteGlobalVirtualGroup{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: 999,
	})
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

// The success path deletes the GVG once its own primary SP requests it.
func (s *TestSuite) TestDeleteGlobalVirtualGroup_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const (
		gvgID    = uint32(50)
		familyID = uint32(5)
		spID     = uint32(1)
	)
	sp := spWithOperator(spID, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id:                    gvgID,
		FamilyId:              familyID,
		PrimarySpId:           spID,
		StoredSize:            0,
		TotalDeposit:          math.ZeroInt(),
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id:                    familyID,
		PrimarySpId:           spID,
		GlobalVirtualGroupIds: []uint32{gvgID},
	})
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: spID, PrimaryCount: 1})

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.paymentKeeper.EXPECT().IsEmptyNetFlow(gomock.Any(), gomock.Any()).Return(true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil).AnyTimes()

	_, err := msgServer.DeleteGlobalVirtualGroup(s.ctx, &types.MsgDeleteGlobalVirtualGroup{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
	})
	require.NoError(s.T(), err)

	_, found := s.virtualgroupKeeper.GetGVG(s.ctx, gvgID)
	require.False(s.T(), found)
}

// Neither the operator nor the funding address resolves to a known SP.
func (s *TestSuite) TestDeposit_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)
	s.spKeeper.EXPECT().GetStorageProviderByFundingAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.Deposit(s.ctx, &types.MsgDeposit{
		StorageProvider:      sample.RandAccAddress().String(),
		GlobalVirtualGroupId: 1,
		Deposit:              sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// A funding address that is not also the operator address still resolves via the fallback lookup.
func (s *TestSuite) TestDeposit_ResolvesViaFundingAddressFallback() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	fundingAddr := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddress().String(), FundingAddress: fundingAddr.String()}
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: gvgID, TotalDeposit: math.ZeroInt(), VirtualPaymentAddress: sample.RandAccAddress().String()})

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)
	s.spKeeper.EXPECT().GetStorageProviderByFundingAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	_, err := msgServer.Deposit(s.ctx, &types.MsgDeposit{
		StorageProvider:      fundingAddr.String(),
		GlobalVirtualGroupId: gvgID,
		Deposit:              sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(10)),
	})
	require.NoError(s.T(), err)

	got, found := s.virtualgroupKeeper.GetGVG(s.ctx, gvgID)
	require.True(s.T(), found)
	require.True(s.T(), got.TotalDeposit.Equal(math.NewInt(10)))
}

func (s *TestSuite) TestDeposit_GVGNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.Deposit(s.ctx, &types.MsgDeposit{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: 999,
		Deposit:              sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestDeposit_WrongDenom() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: gvgID, TotalDeposit: math.ZeroInt()})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.Deposit(s.ctx, &types.MsgDeposit{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Deposit:              sdk.NewCoin("wrongdenom", math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, types.ErrInvalidDenom)
}

// A bank-module failure (e.g. a blocked module account) is passed through unwrapped.
func (s *TestSuite) TestDeposit_BankTransferErrorPropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: gvgID, TotalDeposit: math.ZeroInt()})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	bankErr := errors.New("insufficient funds")
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(bankErr)

	_, err := msgServer.Deposit(s.ctx, &types.MsgDeposit{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Deposit:              sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, bankErr)
}

// The happy path adds to the GVG's existing balance and draws from the SP's funding address.
func (s *TestSuite) TestDeposit_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, TotalDeposit: math.NewInt(5), VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	var gotSender sdk.AccAddress
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), types.ModuleName, gomock.Any()).
		DoAndReturn(func(_ sdk.Context, sender sdk.AccAddress, _ string, coins sdk.Coins) error {
			gotSender = sender
			require.True(s.T(), coins.AmountOf(types.DefaultDepositDenom).Equal(math.NewInt(7)))
			return nil
		})

	_, err := msgServer.Deposit(s.ctx, &types.MsgDeposit{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Deposit:              sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(7)),
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sdk.MustAccAddressFromHex(sp.FundingAddress).Bytes(), gotSender.Bytes())

	got, found := s.virtualgroupKeeper.GetGVG(s.ctx, gvgID)
	require.True(s.T(), found)
	require.True(s.T(), got.TotalDeposit.Equal(math.NewInt(12)))
}

func (s *TestSuite) TestWithdraw_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)
	s.spKeeper.EXPECT().GetStorageProviderByFundingAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sample.RandAccAddress().String(),
		GlobalVirtualGroupId: 1,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// A funding address that is not also the operator address still resolves via the fallback lookup.
func (s *TestSuite) TestWithdraw_ResolvesViaFundingAddressFallback() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	fundingAddr := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddress().String(), FundingAddress: fundingAddr.String()}
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: gvgID, PrimarySpId: sp.Id, TotalDeposit: math.NewInt(10)})

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)
	s.spKeeper.EXPECT().GetStorageProviderByFundingAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), types.ModuleName, gomock.Any(), gomock.Any()).Return(nil)

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      fundingAddr.String(),
		GlobalVirtualGroupId: gvgID,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(5)),
	})
	require.NoError(s.T(), err)
}

func (s *TestSuite) TestWithdraw_GVGNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: 999,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, types.ErrGVGNotExist)
}

func (s *TestSuite) TestWithdraw_NotPrimarySP() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: gvgID, PrimarySpId: 2, TotalDeposit: math.ZeroInt()})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, types.ErrWithdrawFailed)
}

func (s *TestSuite) TestWithdraw_WrongDenom() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{Id: gvgID, PrimarySpId: sp.Id, TotalDeposit: math.ZeroInt()})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Withdraw:             sdk.NewCoin("wrongdenom", math.NewInt(1)),
	})
	require.ErrorIs(s.T(), err, types.ErrInvalidDenom)
}

// A GVG storing more than its deposit can stake for is an inconsistent state that should
// never occur; the defensive panic must fire rather than allow a negative withdrawal.
func (s *TestSuite) TestWithdraw_PanicsOnNegativeAvailableTokens() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	// default staking price is 16000/byte; 1 byte stored needs 16000 staked, but only 1 is deposited.
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, PrimarySpId: sp.Id, StoredSize: 1, TotalDeposit: math.NewInt(1),
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	require.Panics(s.T(), func() {
		_, _ = msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
			StorageProvider:      sp.OperatorAddress,
			GlobalVirtualGroupId: gvgID,
			Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)),
		})
	})
}

func (s *TestSuite) TestWithdraw_AmountTooLarge() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, PrimarySpId: sp.Id, StoredSize: 0, TotalDeposit: math.NewInt(100),
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(101)),
	})
	require.ErrorIs(s.T(), err, types.ErrWithdrawAmountTooLarge)
}

// Amount == 0 means "withdraw everything currently available".
func (s *TestSuite) TestWithdraw_ZeroAmountWithdrawsAllAvailable() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, PrimarySpId: sp.Id, StoredSize: 0, TotalDeposit: math.NewInt(100),
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	var gotAmount math.Int
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), types.ModuleName, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _ string, _ sdk.AccAddress, coins sdk.Coins) error {
			gotAmount = coins.AmountOf(types.DefaultDepositDenom)
			return nil
		})

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.ZeroInt()),
	})
	require.NoError(s.T(), err)
	require.True(s.T(), gotAmount.Equal(math.NewInt(100)))

	got, found := s.virtualgroupKeeper.GetGVG(s.ctx, gvgID)
	require.True(s.T(), found)
	require.True(s.T(), got.TotalDeposit.IsZero())
}

// A positive amount within the available balance withdraws exactly that much.
func (s *TestSuite) TestWithdraw_PartialAmountSucceeds() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, PrimarySpId: sp.Id, StoredSize: 0, TotalDeposit: math.NewInt(100),
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	var gotRecipient sdk.AccAddress
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), types.ModuleName, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _ string, recipient sdk.AccAddress, coins sdk.Coins) error {
			gotRecipient = recipient
			require.True(s.T(), coins.AmountOf(types.DefaultDepositDenom).Equal(math.NewInt(30)))
			return nil
		})

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(30)),
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sdk.MustAccAddressFromHex(sp.FundingAddress).Bytes(), gotRecipient.Bytes())

	got, found := s.virtualgroupKeeper.GetGVG(s.ctx, gvgID)
	require.True(s.T(), found)
	require.True(s.T(), got.TotalDeposit.Equal(math.NewInt(70)))
}

func (s *TestSuite) TestWithdraw_BankTransferErrorPropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, PrimarySpId: sp.Id, StoredSize: 0, TotalDeposit: math.NewInt(100),
	})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	bankErr := errors.New("module account underfunded")
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), types.ModuleName, gomock.Any(), gomock.Any()).Return(bankErr)

	_, err := msgServer.Withdraw(s.ctx, &types.MsgWithdraw{
		StorageProvider:      sp.OperatorAddress,
		GlobalVirtualGroupId: gvgID,
		Withdraw:             sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(10)),
	})
	require.ErrorIs(s.T(), err, bankErr)
}

func (s *TestSuite) TestSwapOut_OperatorSPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.SwapOut(s.ctx, &types.MsgSwapOut{
		StorageProvider:       sample.RandAccAddress().String(),
		GlobalVirtualGroupIds: []uint32{1},
		SuccessorSpId:         2,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestSwapOut_SuccessorSPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.SwapOut(s.ctx, &types.MsgSwapOut{
		StorageProvider:       sp.OperatorAddress,
		GlobalVirtualGroupIds: []uint32{1},
		SuccessorSpId:         2,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestSwapOut_SuccessorNotInService() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	successor := &sptypes.StorageProvider{Id: 2, Status: sptypes.STATUS_IN_MAINTENANCE}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(successor, true)

	_, err := msgServer.SwapOut(s.ctx, &types.MsgSwapOut{
		StorageProvider:       sp.OperatorAddress,
		GlobalVirtualGroupIds: []uint32{1},
		SuccessorSpId:         2,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotInService)
}

// A signature from a key other than the successor's own approval key must be rejected.
func (s *TestSuite) TestSwapOut_InvalidApprovalSignature() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	realKey, err := gethcrypto.GenerateKey()
	require.NoError(s.T(), err)
	wrongKey, err := gethcrypto.GenerateKey()
	require.NoError(s.T(), err)
	successor := &sptypes.StorageProvider{
		Id: 2, Status: sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: gethcrypto.PubkeyToAddress(realKey.PublicKey).Hex(),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(successor, true)

	msg := &types.MsgSwapOut{
		StorageProvider:            sp.OperatorAddress,
		GlobalVirtualGroupFamilyId: 1,
		SuccessorSpId:              successor.Id,
		SuccessorSpApproval:        &common.Approval{},
	}
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(msg.GetApprovalBytes()), wrongKey)
	require.NoError(s.T(), err)
	msg.SuccessorSpApproval.Sig = sig

	_, err = msgServer.SwapOut(s.ctx, msg)
	require.ErrorIs(s.T(), err, sdkerrors.ErrInvalidPubKey)
}

// A duplicate SetSwapOutInfo call for the same family is rejected by the keeper.
func (s *TestSuite) TestSwapOut_DuplicateSwapOutInfoPropagatesError() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 7, nil, sp.Id, 2))

	privKey, err := gethcrypto.GenerateKey()
	require.NoError(s.T(), err)
	successor := &sptypes.StorageProvider{
		Id: 2, Status: sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex(),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(successor, true)

	msg := &types.MsgSwapOut{
		StorageProvider:            sp.OperatorAddress,
		GlobalVirtualGroupFamilyId: 7,
		SuccessorSpId:              successor.Id,
		SuccessorSpApproval:        &common.Approval{},
	}
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(msg.GetApprovalBytes()), privKey)
	require.NoError(s.T(), err)
	msg.SuccessorSpApproval.Sig = sig

	_, err = msgServer.SwapOut(s.ctx, msg)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

// The full happy path: a valid signed approval registers the swap-out.
func (s *TestSuite) TestSwapOut_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	privKey, err := gethcrypto.GenerateKey()
	require.NoError(s.T(), err)
	successor := &sptypes.StorageProvider{
		Id: 2, Status: sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex(),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).Times(2)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(successor, true).Times(2)

	msg := &types.MsgSwapOut{
		StorageProvider:            sp.OperatorAddress,
		GlobalVirtualGroupFamilyId: 9,
		SuccessorSpId:              successor.Id,
		SuccessorSpApproval:        &common.Approval{},
	}
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(msg.GetApprovalBytes()), privKey)
	require.NoError(s.T(), err)
	msg.SuccessorSpApproval.Sig = sig

	_, err = msgServer.SwapOut(s.ctx, msg)
	require.NoError(s.T(), err)

	// Re-registering the same family must now fail -- confirms the info was persisted.
	_, err = msgServer.SwapOut(s.ctx, msg)
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

func (s *TestSuite) TestCancelSwapOut_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.CancelSwapOut(s.ctx, &types.MsgCancelSwapOut{
		StorageProvider:            sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: 1,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// Canceling a swap-out that was never registered is rejected by the keeper.
func (s *TestSuite) TestCancelSwapOut_NotRegisteredPropagatesError() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(3, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.CancelSwapOut(s.ctx, &types.MsgCancelSwapOut{
		StorageProvider:            sp.OperatorAddress,
		GlobalVirtualGroupFamilyId: 1,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// The success path clears a previously-registered swap-out.
func (s *TestSuite) TestCancelSwapOut_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(3, sample.RandAccAddress())
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, 1, nil, sp.Id, 4))
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).Times(2)

	_, err := msgServer.CancelSwapOut(s.ctx, &types.MsgCancelSwapOut{
		StorageProvider:            sp.OperatorAddress,
		GlobalVirtualGroupFamilyId: 1,
	})
	require.NoError(s.T(), err)

	// Canceling again now fails -- the info is really gone.
	_, err = msgServer.CancelSwapOut(s.ctx, &types.MsgCancelSwapOut{
		StorageProvider:            sp.OperatorAddress,
		GlobalVirtualGroupFamilyId: 1,
	})
	require.Error(s.T(), err)
}

func (s *TestSuite) TestCompleteSwapOut_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.CompleteSwapOut(s.ctx, &types.MsgCompleteSwapOut{
		StorageProvider:            sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: 1,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCompleteSwapOut_NotRegisteredPropagatesError() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	successor := spWithOperator(2, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)

	_, err := msgServer.CompleteSwapOut(s.ctx, &types.MsgCompleteSwapOut{
		StorageProvider:            successor.OperatorAddress,
		GlobalVirtualGroupFamilyId: 1,
	})
	require.ErrorIs(s.T(), err, types.ErrSwapOutFailed)
}

// The full happy path completes a family-level swap-out, handing the family to the successor.
func (s *TestSuite) TestCompleteSwapOut_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const (
		familyID  = uint32(11)
		originID  = uint32(1)
		successID = uint32(2)
	)
	origin := spWithOperator(originID, sample.RandAccAddress())
	successor := spWithOperator(successID, sample.RandAccAddress())

	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: familyID, PrimarySpId: originID, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: originID, PrimaryCount: 1})
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: originID, GlobalVirtualGroupFamilyIds: []uint32{familyID},
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SetSwapOutInfo(s.ctx, familyID, nil, originID, successID))

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), originID).Return(origin, true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil)

	_, err := msgServer.CompleteSwapOut(s.ctx, &types.MsgCompleteSwapOut{
		StorageProvider:            successor.OperatorAddress,
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.NoError(s.T(), err)

	got, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, familyID)
	require.True(s.T(), found)
	require.Equal(s.T(), successID, got.PrimarySpId)
}

func (s *TestSuite) TestSettle_FamilyNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	_, err := msgServer.Settle(s.ctx, &types.MsgSettle{
		StorageProvider:            sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: 123,
	})
	require.ErrorIs(s.T(), err, types.ErrGVGFamilyNotExist)
}

func (s *TestSuite) TestSettle_FamilyPrimarySPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const familyID = uint32(1)
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: 99})
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.Settle(s.ctx, &types.MsgSettle{
		StorageProvider:            sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// A settlement failure on the family path is surfaced as the same sentinel used on the per-GVG path.
func (s *TestSuite) TestSettle_FamilySettleFailurePropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const familyID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: familyID, PrimarySpId: sp.Id, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), errors.New("query failed"))

	_, err := msgServer.Settle(s.ctx, &types.MsgSettle{
		StorageProvider:            sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.ErrorIs(s.T(), err, types.ErrSettleFailed)
}

// The family-level success path distributes a positive balance to the primary SP's funding address.
func (s *TestSuite) TestSettle_FamilySuccess() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const familyID = uint32(1)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: familyID, PrimarySpId: sp.Id, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.NewInt(500), nil)
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	_, err := msgServer.Settle(s.ctx, &types.MsgSettle{
		StorageProvider:            sample.RandAccAddress().String(),
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.NoError(s.T(), err)
}

// The per-GVG path's own settlement failure is likewise surfaced as ErrSettleFailed.
func (s *TestSuite) TestSettle_PerGVGSettleFailurePropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.NewInt(-1), nil)

	_, err := msgServer.Settle(s.ctx, &types.MsgSettle{
		StorageProvider:       sample.RandAccAddress().String(),
		GlobalVirtualGroupIds: []uint32{gvgID},
	})
	require.ErrorIs(s.T(), err, types.ErrSettleFailed)
}

func (s *TestSuite) TestStorageProviderExit_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.StorageProviderExit(s.ctx, &types.MsgStorageProviderExit{StorageProvider: sample.RandAccAddress().String()})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestStorageProviderExit_NotInService() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_IN_MAINTENANCE, OperatorAddress: sample.RandAccAddress().String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.StorageProviderExit(s.ctx, &types.MsgStorageProviderExit{StorageProvider: sp.OperatorAddress})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderExitFailed)
}

// An SP that still breaks the redundancy requirement on one of its GVGs cannot exit.
func (s *TestSuite) TestStorageProviderExit_BreaksRedundancyRequirement() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: sp.Id, BreakRedundancyReqmtGvgCount: 1})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.StorageProviderExit(s.ctx, &types.MsgStorageProviderExit{StorageProvider: sp.OperatorAddress})
	require.ErrorIs(s.T(), err, types.ErrSPCanNotExit)
}

// The default concurrent-exit limit is 1: one SP already exiting fills the only slot.
func (s *TestSuite) TestStorageProviderExit_ConcurrentLimitReached() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	alreadyExiting := sptypes.StorageProvider{Id: 2, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetAllStorageProviders(gomock.Any()).Return([]sptypes.StorageProvider{alreadyExiting})

	_, err := msgServer.StorageProviderExit(s.ctx, &types.MsgStorageProviderExit{StorageProvider: sp.OperatorAddress})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderExitFailed)
}

// The happy path moves the SP into graceful-exiting status.
func (s *TestSuite) TestStorageProviderExit_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetAllStorageProviders(gomock.Any()).Return(nil)

	var stored *sptypes.StorageProvider
	s.spKeeper.EXPECT().SetStorageProvider(gomock.Any(), gomock.Any()).
		Do(func(_ sdk.Context, sp *sptypes.StorageProvider) { stored = sp })

	_, err := msgServer.StorageProviderExit(s.ctx, &types.MsgStorageProviderExit{StorageProvider: sp.OperatorAddress})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sptypes.STATUS_GRACEFUL_EXITING, stored.Status)
}

func (s *TestSuite) TestCompleteStorageProviderExit_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.CompleteStorageProviderExit(s.ctx, &types.MsgCompleteStorageProviderExit{
		StorageProvider: sample.RandAccAddress().String(),
		Operator:        sample.RandAccAddress().String(),
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCompleteStorageProviderExit_NotExiting() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress()) // defaults to STATUS_IN_SERVICE
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.CompleteStorageProviderExit(s.ctx, &types.MsgCompleteStorageProviderExit{
		StorageProvider: sp.OperatorAddress,
		Operator:        sample.RandAccAddress().String(),
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderExitFailed)
}

// StorageProviderExitable's own precondition failure (still primary of a family) propagates.
func (s *TestSuite) TestCompleteStorageProviderExit_NotExitablePropagatesError() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := &sptypes.StorageProvider{Id: 1, Status: sptypes.STATUS_GRACEFUL_EXITING, OperatorAddress: sample.RandAccAddress().String()}
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: sp.Id, PrimaryCount: 1})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.CompleteStorageProviderExit(s.ctx, &types.MsgCompleteStorageProviderExit{
		StorageProvider: sp.OperatorAddress,
		Operator:        sample.RandAccAddress().String(),
	})
	require.ErrorIs(s.T(), err, types.ErrSPCanNotExit)
}

// A graceful exit refunds the total deposit to the SP's own funding address.
func (s *TestSuite) TestCompleteStorageProviderExit_GracefulSuccess() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := &sptypes.StorageProvider{
		Id: 1, Status: sptypes.STATUS_GRACEFUL_EXITING,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
		TotalDeposit:    math.NewInt(1000),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return(sptypes.DefaultDepositDenom)

	var gotRecipient sdk.AccAddress
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), sptypes.ModuleName, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _ string, recipient sdk.AccAddress, coins sdk.Coins) error {
			gotRecipient = recipient
			require.True(s.T(), coins.AmountOf(sptypes.DefaultDepositDenom).Equal(math.NewInt(1000)))
			return nil
		})
	s.spKeeper.EXPECT().Exit(gomock.Any(), gomock.Any()).Return(nil)

	_, err := msgServer.CompleteStorageProviderExit(s.ctx, &types.MsgCompleteStorageProviderExit{
		StorageProvider: sp.OperatorAddress,
		Operator:        sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sdk.MustAccAddressFromHex(sp.FundingAddress).Bytes(), gotRecipient.Bytes())
}

// A forced exit instead sweeps the deposit to the payment module's governance address.
func (s *TestSuite) TestCompleteStorageProviderExit_ForcedSuccess() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := &sptypes.StorageProvider{
		Id: 1, Status: sptypes.STATUS_FORCED_EXITING,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
		TotalDeposit:    math.NewInt(2000),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return(sptypes.DefaultDepositDenom)

	var gotRecipient sdk.AccAddress
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), sptypes.ModuleName, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ sdk.Context, _ string, recipient sdk.AccAddress, _ sdk.Coins) error {
			gotRecipient = recipient
			return nil
		})
	s.spKeeper.EXPECT().Exit(gomock.Any(), gomock.Any()).Return(nil)

	_, err := msgServer.CompleteStorageProviderExit(s.ctx, &types.MsgCompleteStorageProviderExit{
		StorageProvider: sp.OperatorAddress,
		Operator:        sample.RandAccAddress().String(),
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), paymenttypes.GovernanceAddress.Bytes(), gotRecipient.Bytes())
}

func (s *TestSuite) TestCompleteStorageProviderExit_BankTransferErrorPropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := &sptypes.StorageProvider{
		Id: 1, Status: sptypes.STATUS_GRACEFUL_EXITING,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
		TotalDeposit:    math.NewInt(1000),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return(sptypes.DefaultDepositDenom)
	bankErr := errors.New("module account underfunded")
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(bankErr)

	_, err := msgServer.CompleteStorageProviderExit(s.ctx, &types.MsgCompleteStorageProviderExit{
		StorageProvider: sp.OperatorAddress,
		Operator:        sample.RandAccAddress().String(),
	})
	require.ErrorIs(s.T(), err, bankErr)
}

// spKeeper.Exit failing (e.g. an accounting invariant) is passed straight through.
func (s *TestSuite) TestCompleteStorageProviderExit_ExitErrorPropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := &sptypes.StorageProvider{
		Id: 1, Status: sptypes.STATUS_GRACEFUL_EXITING,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
		TotalDeposit:    math.ZeroInt(),
	}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return(sptypes.DefaultDepositDenom)
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	exitErr := errors.New("exit accounting invariant violated")
	s.spKeeper.EXPECT().Exit(gomock.Any(), gomock.Any()).Return(exitErr)

	_, err := msgServer.CompleteStorageProviderExit(s.ctx, &types.MsgCompleteStorageProviderExit{
		StorageProvider: sp.OperatorAddress,
		Operator:        sample.RandAccAddress().String(),
	})
	require.ErrorIs(s.T(), err, exitErr)
}

func (s *TestSuite) TestReserveSwapIn_SuccessorSPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.ReserveSwapIn(s.ctx, &types.MsgReserveSwapIn{StorageProvider: sample.RandAccAddress().String(), TargetSpId: 2})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// An SP cannot swap into its own slot.
func (s *TestSuite) TestReserveSwapIn_CannotSwapSelf() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	successor := spWithOperator(5, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)

	_, err := msgServer.ReserveSwapIn(s.ctx, &types.MsgReserveSwapIn{StorageProvider: successor.OperatorAddress, TargetSpId: successor.Id})
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

// An SP that is itself exiting must not be able to reserve a swap-in: it would take
// over a family it is in the middle of handing off, and owning one keeps its own exit
// from ever completing. MsgSwapOut already requires its successor to be in service.
func (s *TestSuite) TestReserveSwapIn_SuccessorNotInService() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const familyID = uint32(30)
	successor := spWithOperator(5, sample.RandAccAddress())
	successor.Status = sptypes.STATUS_FORCED_EXITING
	target := &sptypes.StorageProvider{Id: 9, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: target.Id})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(target, true).AnyTimes()

	_, err := msgServer.ReserveSwapIn(s.ctx, &types.MsgReserveSwapIn{
		StorageProvider:            successor.OperatorAddress,
		TargetSpId:                 target.Id,
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotInService)

	_, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, familyID, 0)
	require.False(s.T(), found, "no reservation may be recorded for a successor that is not in service")
}

// Same for an SP in maintenance, which cannot serve the slot it is reserving.
func (s *TestSuite) TestReserveSwapIn_SuccessorInMaintenance() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const familyID = uint32(31)
	successor := spWithOperator(5, sample.RandAccAddress())
	successor.Status = sptypes.STATUS_IN_MAINTENANCE
	target := &sptypes.StorageProvider{Id: 9, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: target.Id})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(target, true).AnyTimes()

	_, err := msgServer.ReserveSwapIn(s.ctx, &types.MsgReserveSwapIn{
		StorageProvider:            successor.OperatorAddress,
		TargetSpId:                 target.Id,
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotInService)

	_, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, familyID, 0)
	require.False(s.T(), found)
}

func (s *TestSuite) TestReserveSwapIn_TargetSPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	successor := spWithOperator(5, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.ReserveSwapIn(s.ctx, &types.MsgReserveSwapIn{StorageProvider: successor.OperatorAddress, TargetSpId: 9})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// The keeper's own SwapIn validation failure (the target is not exiting) propagates.
func (s *TestSuite) TestReserveSwapIn_KeeperErrorPropagates() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	successor := spWithOperator(5, sample.RandAccAddress())
	target := &sptypes.StorageProvider{Id: 9, Status: sptypes.STATUS_IN_SERVICE}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(target, true)

	_, err := msgServer.ReserveSwapIn(s.ctx, &types.MsgReserveSwapIn{
		StorageProvider:            successor.OperatorAddress,
		TargetSpId:                 target.Id,
		GlobalVirtualGroupFamilyId: 3,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderWrongStatus)
}

// The happy path reserves the swap and derives the expiration deterministically from the
// block time and the configured validity period.
func (s *TestSuite) TestReserveSwapIn_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const familyID = uint32(3)
	successor := spWithOperator(5, sample.RandAccAddress())
	target := &sptypes.StorageProvider{Id: 9, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: target.Id})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(target, true)

	blockTime := time.Unix(1_800_000_000, 0).UTC()
	s.ctx = s.ctx.WithBlockTime(blockTime)

	_, err := msgServer.ReserveSwapIn(s.ctx, &types.MsgReserveSwapIn{
		StorageProvider:            successor.OperatorAddress,
		TargetSpId:                 target.Id,
		GlobalVirtualGroupFamilyId: familyID,
	})
	require.NoError(s.T(), err)

	info, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, familyID, 0)
	require.True(s.T(), found)
	require.Equal(s.T(), successor.Id, info.SuccessorSpId)
	wantExpiration := uint64(blockTime.Unix()) + s.virtualgroupKeeper.SwapInValidityPeriod(s.ctx) //nolint:gosec // G115
	require.Equal(s.T(), wantExpiration, info.ExpirationTime)
}

func (s *TestSuite) TestCancelSwapIn_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.CancelSwapIn(s.ctx, &types.MsgCancelSwapIn{StorageProvider: sample.RandAccAddress().String()})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCancelSwapIn_NotRegisteredPropagatesError() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(5, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.CancelSwapIn(s.ctx, &types.MsgCancelSwapIn{StorageProvider: sp.OperatorAddress, GlobalVirtualGroupFamilyId: 1})
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

func (s *TestSuite) TestCancelSwapIn_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const familyID = uint32(4)
	sp := spWithOperator(5, sample.RandAccAddress())
	target := &sptypes.StorageProvider{Id: 9, Status: sptypes.STATUS_GRACEFUL_EXITING}
	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{Id: familyID, PrimarySpId: target.Id})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, familyID, 0, sp.Id, target, s.ctx.BlockTime().Unix()+1000))
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.CancelSwapIn(s.ctx, &types.MsgCancelSwapIn{StorageProvider: sp.OperatorAddress, GlobalVirtualGroupFamilyId: familyID})
	require.NoError(s.T(), err)

	_, found := s.virtualgroupKeeper.GetSwapInInfo(s.ctx, familyID, 0)
	require.False(s.T(), found)
}

func (s *TestSuite) TestCompleteSwapIn_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.CompleteSwapIn(s.ctx, &types.MsgCompleteSwapIn{StorageProvider: sample.RandAccAddress().String()})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestCompleteSwapIn_NotRegisteredPropagatesError() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(5, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)

	_, err := msgServer.CompleteSwapIn(s.ctx, &types.MsgCompleteSwapIn{StorageProvider: sp.OperatorAddress, GlobalVirtualGroupFamilyId: 1})
	require.ErrorIs(s.T(), err, types.ErrSwapInFailed)
}

// The full happy path completes a family-level swap-in, handing the family to the successor.
func (s *TestSuite) TestCompleteSwapIn_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const (
		familyID  = uint32(6)
		targetID  = uint32(1)
		successID = uint32(2)
	)
	successor := spWithOperator(successID, sample.RandAccAddress())
	target := &sptypes.StorageProvider{Id: targetID, Status: sptypes.STATUS_GRACEFUL_EXITING}

	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: familyID, PrimarySpId: targetID, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: targetID, PrimaryCount: 1})
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: targetID, GlobalVirtualGroupFamilyIds: []uint32{familyID},
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, familyID, 0, successID, target, s.ctx.BlockTime().Unix()+1000))

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), targetID).Return(target, true)
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil)

	_, err := msgServer.CompleteSwapIn(s.ctx, &types.MsgCompleteSwapIn{StorageProvider: successor.OperatorAddress, GlobalVirtualGroupFamilyId: familyID})
	require.NoError(s.T(), err)

	got, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, familyID)
	require.True(s.T(), found)
	require.Equal(s.T(), successID, got.PrimarySpId)

	_, found = s.virtualgroupKeeper.GetSwapInInfo(s.ctx, familyID, 0)
	require.False(s.T(), found)
}

// The successor's status is checked again at completion: an SP that started exiting
// between reserving and completing must not be handed the family.
func (s *TestSuite) TestCompleteSwapIn_SuccessorNotInService() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const (
		familyID  = uint32(7)
		targetID  = uint32(1)
		successID = uint32(2)
	)
	successor := spWithOperator(successID, sample.RandAccAddress())
	target := &sptypes.StorageProvider{Id: targetID, Status: sptypes.STATUS_GRACEFUL_EXITING}

	s.virtualgroupKeeper.SetGVGFamily(s.ctx, &types.GlobalVirtualGroupFamily{
		Id: familyID, PrimarySpId: targetID, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.virtualgroupKeeper.SetGVGStatisticsWithSP(s.ctx, &types.GVGStatisticsWithinSP{StorageProviderId: targetID, PrimaryCount: 1})
	s.virtualgroupKeeper.SetGVGFamilyStatisticsWithinSP(s.ctx, &types.GVGFamilyStatisticsWithinSP{
		SpId: targetID, GlobalVirtualGroupFamilyIds: []uint32{familyID},
	})
	require.NoError(s.T(), s.virtualgroupKeeper.SwapIn(s.ctx, familyID, 0, successID, target, s.ctx.BlockTime().Unix()+1000))

	successor.Status = sptypes.STATUS_GRACEFUL_EXITING
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(successor, true)
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), targetID).Return(target, true).AnyTimes()
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil).AnyTimes()

	_, err := msgServer.CompleteSwapIn(s.ctx, &types.MsgCompleteSwapIn{
		StorageProvider: successor.OperatorAddress, GlobalVirtualGroupFamilyId: familyID,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotInService)

	got, found := s.virtualgroupKeeper.GetGVGFamily(s.ctx, familyID)
	require.True(s.T(), found)
	require.Equal(s.T(), targetID, got.PrimarySpId, "the family must stay with the target SP")
}

func (s *TestSuite) TestStorageProviderForcedExit_RejectsWrongAuthority() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	_, err := msgServer.StorageProviderForcedExit(s.ctx, &types.MsgStorageProviderForcedExit{
		Authority:       sample.RandAccAddress().String(),
		StorageProvider: sample.RandAccAddress().String(),
	})
	require.ErrorIs(s.T(), err, govtypes.ErrInvalidSigner)
}

func (s *TestSuite) TestStorageProviderForcedExit_SPNotFound() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false)

	_, err := msgServer.StorageProviderForcedExit(s.ctx, &types.MsgStorageProviderForcedExit{
		Authority:       s.virtualgroupKeeper.GetAuthority(),
		StorageProvider: sample.RandAccAddress().String(),
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

func (s *TestSuite) TestStorageProviderForcedExit_ConcurrentLimitReached() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	alreadyExiting := sptypes.StorageProvider{Id: 2, Status: sptypes.STATUS_FORCED_EXITING}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetAllStorageProviders(gomock.Any()).Return([]sptypes.StorageProvider{alreadyExiting})

	_, err := msgServer.StorageProviderForcedExit(s.ctx, &types.MsgStorageProviderForcedExit{
		Authority:       s.virtualgroupKeeper.GetAuthority(),
		StorageProvider: sp.OperatorAddress,
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderExitFailed)
}

// Governance can force any SP -- including one currently in service -- into forced-exit.
func (s *TestSuite) TestStorageProviderForcedExit_Success() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	sp := spWithOperator(1, sample.RandAccAddress())
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true)
	s.spKeeper.EXPECT().GetAllStorageProviders(gomock.Any()).Return(nil)

	var stored *sptypes.StorageProvider
	s.spKeeper.EXPECT().SetStorageProvider(gomock.Any(), gomock.Any()).
		Do(func(_ sdk.Context, sp *sptypes.StorageProvider) { stored = sp })

	_, err := msgServer.StorageProviderForcedExit(s.ctx, &types.MsgStorageProviderForcedExit{
		Authority:       s.virtualgroupKeeper.GetAuthority(),
		StorageProvider: sp.OperatorAddress,
	})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sptypes.STATUS_FORCED_EXITING, stored.Status)
}

// A GVG id repeated in the same request is settled only once -- the strict mock
// controller fails the test if the dedup guard is removed and a second call occurs.
func (s *TestSuite) TestSettle_PerGVGSkipsDuplicateID() {
	msgServer := keeper.NewMsgServerImpl(*s.virtualgroupKeeper)
	const gvgID = uint32(1)
	s.virtualgroupKeeper.SetGVG(s.ctx, &types.GlobalVirtualGroup{
		Id: gvgID, VirtualPaymentAddress: sample.RandAccAddress().String(),
	})
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.ZeroInt(), nil).Times(1)

	_, err := msgServer.Settle(s.ctx, &types.MsgSettle{
		StorageProvider:       sample.RandAccAddress().String(),
		GlobalVirtualGroupIds: []uint32{gvgID, gvgID},
	})
	require.NoError(s.T(), err)
}
