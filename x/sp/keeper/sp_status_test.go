package keeper_test

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

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
