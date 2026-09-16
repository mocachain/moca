package keeper_test

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"

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

// TestForceUpdateMaintenanceRecordsKeepsNonMaintenanceStatus guards the status
// guard in Keeper.ForceUpdateMaintenanceRecords: when a maintenance window lapses
// after the SP was moved to a jailed or exiting status, the SP must keep that
// status and no status-change event may be emitted.
func (s *KeeperTestSuite) TestForceUpdateMaintenanceRecordsKeepsNonMaintenanceStatus() {
	k := s.spKeeper
	statusEvent := proto.MessageName(&types.EventUpdateStorageProviderStatus{})
	spID := uint32(200)

	for _, status := range []types.Status{
		types.STATUS_FORCED_EXITING,
		types.STATUS_GRACEFUL_EXITING,
		types.STATUS_IN_JAILED,
	} {
		s.Run(status.String(), func() {
			startTime := time.Unix(1000, 0)
			ctx := s.ctx.WithBlockHeight(100).WithBlockTime(startTime)

			spID++
			spAcc := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
			sp := &types.StorageProvider{
				Id:              spID,
				OperatorAddress: spAcc.String(),
				Status:          types.STATUS_IN_SERVICE,
			}
			k.SetStorageProvider(ctx, sp)

			requestDuration := int64(100)
			s.Require().NoError(k.UpdateToInMaintenance(ctx, sp, requestDuration))
			k.SetStorageProvider(ctx, sp)

			// The status changes while the maintenance window is still open, and the
			// maintenance record is left as is (as x/virtualgroup's forced exit does).
			sp.Status = status
			k.SetStorageProvider(ctx, sp)

			overdueCtx := ctx.
				WithBlockTime(startTime.Add(time.Duration(requestDuration+1) * time.Second)).
				WithEventManager(sdk.NewEventManager())

			k.ForceUpdateMaintenanceRecords(overdueCtx)

			updated, found := k.GetStorageProvider(overdueCtx, sp.Id)
			s.Require().True(found)
			s.Require().Equal(status, updated.Status)
			for _, ev := range overdueCtx.EventManager().Events() {
				s.Require().NotEqual(statusEvent, ev.Type)
			}
		})
	}
}
