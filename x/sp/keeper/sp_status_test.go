package keeper_test

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

// TestForceUpdateMaintenanceRecords is a behavior regression test for
// Keeper.ForceUpdateMaintenanceRecords. It verifies that an SP whose maintenance
// window has elapsed is forced back into service and its record back-filled. It
// also exercises the prefix iterator that the function must close on every call.
func (s *KeeperTestSuite) TestForceUpdateMaintenanceRecords() {
	k := s.spKeeper

	startTime := time.Unix(1000, 0)
	ctx := s.ctx.WithBlockHeight(100).WithBlockTime(startTime)

	spAcc := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
	sp := &types.StorageProvider{
		Id:              100,
		OperatorAddress: spAcc.String(),
		Status:          types.STATUS_IN_SERVICE,
	}
	k.SetStorageProvider(ctx, sp)

	// Put the SP into maintenance with a short requested duration, then persist the
	// updated status so the iterator observes STATUS_IN_MAINTENANCE.
	requestDuration := int64(100)
	s.Require().NoError(k.UpdateToInMaintenance(ctx, sp, requestDuration))
	s.Require().Equal(types.STATUS_IN_MAINTENANCE, sp.Status)
	k.SetStorageProvider(ctx, sp)

	// Advance the clock past RequestAt + RequestDuration so the record is overdue.
	overdueCtx := ctx.WithBlockTime(startTime.Add(time.Duration(requestDuration+1) * time.Second))

	k.ForceUpdateMaintenanceRecords(overdueCtx)

	// The SP must be forced back into service.
	updated, found := k.GetStorageProvider(overdueCtx, sp.Id)
	s.Require().True(found)
	s.Require().Equal(types.STATUS_IN_SERVICE, updated.Status)

	// The overdue record must have its actual duration back-filled.
	resp, err := k.StorageProviderMaintenanceRecordsByOperatorAddress(overdueCtx, &types.QueryStorageProviderMaintenanceRecordsRequest{
		OperatorAddress: spAcc.String(),
	})
	s.Require().NoError(err)
	s.Require().Len(resp.Records, 1)
	s.Require().Equal(requestDuration, resp.Records[0].ActualDuration)
}

// TestUpdateToInMaintenance table-drives the remaining branches: a single
// request that alone exceeds the full quota, a second request inside the
// lock-up window, a second request past lock-up but over the cumulative quota,
// and a second request that succeeds and appends.
func (s *KeeperTestSuite) TestUpdateToInMaintenance() {
	k := s.spKeeper
	params := k.GetParams(s.ctx)
	quota := params.MaintenanceDurationQuota
	lockup := params.NumOfLockupBlocksForMaintenance

	startTime := time.Unix(1_000_000, 0)
	ctx := s.ctx.WithBlockHeight(100).WithBlockTime(startTime)

	sp := &types.StorageProvider{
		Id:              1,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		Status:          types.STATUS_IN_SERVICE,
	}
	k.SetStorageProvider(ctx, sp)

	// A single request that alone exceeds the full quota is rejected outright,
	// even with no prior record.
	err := k.UpdateToInMaintenance(ctx, sp, quota+1)
	require.ErrorIs(s.T(), err, types.ErrStorageProviderStatusUpdateNotAllow)

	// First (successful) maintenance window, well within quota.
	firstDuration := quota - 1600
	require.NoError(s.T(), k.UpdateToInMaintenance(ctx, sp, firstDuration))
	require.Equal(s.T(), types.STATUS_IN_MAINTENANCE, sp.Status)
	k.SetStorageProvider(ctx, sp)

	// Bring it back into service; the actual duration back-fills to firstDuration.
	inServiceCtx := ctx.WithBlockTime(startTime.Add(time.Duration(firstDuration) * time.Second))
	k.UpdateToInService(inServiceCtx, sp)
	require.Equal(s.T(), types.STATUS_IN_SERVICE, sp.Status)
	k.SetStorageProvider(inServiceCtx, sp)

	// Requesting again before the lock-up window (anchored on the first
	// record's height) elapses is rejected, even though the SP is back in service.
	stillLocked := inServiceCtx.WithBlockHeight(ctx.BlockHeight() + 1)
	err = k.UpdateToInMaintenance(stillLocked, sp, 100)
	require.ErrorIs(s.T(), err, types.ErrStorageProviderStatusUpdateNotAllow)

	// After the lock-up window, a request that would push the cumulative used
	// time (firstDuration, now recorded as ActualDuration) over quota is rejected.
	afterLockup := inServiceCtx.WithBlockHeight(ctx.BlockHeight() + lockup)
	err = k.UpdateToInMaintenance(afterLockup, sp, 1601)
	require.ErrorIs(s.T(), err, types.ErrStorageProviderStatusUpdateNotAllow)

	// A request within the remaining quota succeeds and appends a new record.
	require.NoError(s.T(), k.UpdateToInMaintenance(afterLockup, sp, 1600))
	k.SetStorageProvider(afterLockup, sp)

	resp, err := k.StorageProviderMaintenanceRecordsByOperatorAddress(afterLockup, &types.QueryStorageProviderMaintenanceRecordsRequest{
		OperatorAddress: sp.OperatorAddress,
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), resp.Records, 2)
}

// TestUpdateToInServiceNoPriorRecord covers the branch where the SP has never
// had a maintenance record written, so the back-fill block is skipped entirely
// but the status flip still applies.
func (s *KeeperTestSuite) TestUpdateToInServiceNoPriorRecord() {
	k := s.spKeeper
	sp := &types.StorageProvider{
		Id:              2,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		Status:          types.STATUS_IN_MAINTENANCE,
	}

	k.UpdateToInService(s.ctx, sp)
	require.Equal(s.T(), types.STATUS_IN_SERVICE, sp.Status)
}

// TestForceUpdateMaintenanceRecordsPurgesToEmpty seeds a single maintenance
// record old enough to be purged outright by ForceUpdateMaintenanceRecords,
// with the SP already STATUS_IN_SERVICE so the force-to-service loop must be
// skipped entirely (it only runs for SPs not already in service).
func (s *KeeperTestSuite) TestForceUpdateMaintenanceRecordsPurgesToEmpty() {
	k := s.spKeeper
	params := k.GetParams(s.ctx)
	spAcc := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())

	createCtx := s.ctx.WithBlockHeight(100).WithBlockTime(time.Unix(1000, 0))
	sp := &types.StorageProvider{Id: 3, OperatorAddress: spAcc.String(), Status: types.STATUS_IN_SERVICE}
	k.SetStorageProvider(createCtx, sp)
	require.NoError(s.T(), k.UpdateToInMaintenance(createCtx, sp, 100))
	k.SetStorageProvider(createCtx, sp)
	k.UpdateToInService(createCtx, sp)
	k.SetStorageProvider(createCtx, sp)

	purgeHeight := createCtx.BlockHeight() + params.NumOfHistoricalBlocksForMaintenanceRecords + 1
	purgeCtx := createCtx.WithBlockHeight(purgeHeight)

	k.ForceUpdateMaintenanceRecords(purgeCtx)

	resp, err := k.StorageProviderMaintenanceRecordsByOperatorAddress(purgeCtx, &types.QueryStorageProviderMaintenanceRecordsRequest{
		OperatorAddress: spAcc.String(),
	})
	require.NoError(s.T(), err)
	require.Empty(s.T(), resp.Records)

	updated, found := k.GetStorageProvider(purgeCtx, sp.Id)
	require.True(s.T(), found)
	require.Equal(s.T(), types.STATUS_IN_SERVICE, updated.Status)
}

// TestForceUpdateMaintenanceRecordsPartialPurge seeds two maintenance records:
// an old one that must be purged and a recent one that must survive, asserting
// the surviving record is kept (the store.Set branch, as opposed to the
// delete-when-empty branch covered above).
func (s *KeeperTestSuite) TestForceUpdateMaintenanceRecordsPartialPurge() {
	k := s.spKeeper
	params := k.GetParams(s.ctx)
	spAcc := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())

	ctx1 := s.ctx.WithBlockHeight(100).WithBlockTime(time.Unix(1000, 0))
	sp := &types.StorageProvider{Id: 4, OperatorAddress: spAcc.String(), Status: types.STATUS_IN_SERVICE}
	k.SetStorageProvider(ctx1, sp)
	require.NoError(s.T(), k.UpdateToInMaintenance(ctx1, sp, 10))
	k.SetStorageProvider(ctx1, sp)
	k.UpdateToInService(ctx1, sp)
	k.SetStorageProvider(ctx1, sp)

	ctx2 := ctx1.WithBlockHeight(ctx1.BlockHeight() + params.NumOfLockupBlocksForMaintenance)
	require.NoError(s.T(), k.UpdateToInMaintenance(ctx2, sp, 10))
	k.SetStorageProvider(ctx2, sp)

	forceHeight := ctx1.BlockHeight() + params.NumOfHistoricalBlocksForMaintenanceRecords + 1
	forceCtx := ctx2.WithBlockHeight(forceHeight)

	k.ForceUpdateMaintenanceRecords(forceCtx)

	resp, err := k.StorageProviderMaintenanceRecordsByOperatorAddress(forceCtx, &types.QueryStorageProviderMaintenanceRecordsRequest{
		OperatorAddress: spAcc.String(),
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), resp.Records, 1)
	require.Equal(s.T(), ctx2.BlockHeight(), resp.Records[0].Height)
}
