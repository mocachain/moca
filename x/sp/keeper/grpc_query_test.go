package keeper_test

import (
	gocontext "context"
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func (s *KeeperTestSuite) TestQueryParams() {
	res, err := s.queryClient.Params(gocontext.Background(), &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(res)
	s.Require().Equal(s.spKeeper.GetParams(s.ctx), res.GetParams())
}

// requireInvalidArgument asserts err is a gRPC status error carrying
// codes.InvalidArgument, the code every nil-request guard in grpc_query.go
// reports.
func requireInvalidArgument(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestStorageProvidersQuery calls the keeper method directly (rather than
// through s.queryClient, which never sends a nil request) to also exercise the
// req == nil guard. StorageProviders shuffles its result, so the assertion is
// on set membership, not order.
func (s *KeeperTestSuite) TestStorageProvidersQuery() {
	_, err := s.spKeeper.StorageProviders(gocontext.Background(), nil)
	requireInvalidArgument(s.T(), err)

	ids := []uint32{10, 20, 30}
	want := make(map[uint32]bool, len(ids))
	for _, id := range ids {
		sp := &types.StorageProvider{Id: id, OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String()}
		s.spKeeper.SetStorageProvider(s.ctx, sp)
		want[id] = true
	}

	resp, err := s.spKeeper.StorageProviders(s.ctx, &types.QueryStorageProvidersRequest{})
	require.NoError(s.T(), err)
	require.Len(s.T(), resp.Sps, len(ids))
	got := make(map[uint32]bool, len(resp.Sps))
	for _, sp := range resp.Sps {
		got[sp.Id] = true
	}
	require.Equal(s.T(), want, got)

	// Setting both Offset and Key on the same request is rejected by the
	// underlying pagination helper; that error must propagate as Internal.
	_, err = s.spKeeper.StorageProviders(s.ctx, &types.QueryStorageProvidersRequest{
		Pagination: &query.PageRequest{Offset: 1, Key: []byte("x")},
	})
	require.Error(s.T(), err)
	st, ok := status.FromError(err)
	require.True(s.T(), ok)
	require.Equal(s.T(), codes.Internal, st.Code())
}

func (s *KeeperTestSuite) TestQuerySpStoragePrice() {
	_, err := s.spKeeper.QuerySpStoragePrice(s.ctx, nil)
	requireInvalidArgument(s.T(), err)

	_, err = s.spKeeper.QuerySpStoragePrice(s.ctx, &types.QuerySpStoragePriceRequest{SpAddr: "not-a-hex-address"})
	requireInvalidArgument(s.T(), err)

	_, err = s.spKeeper.QuerySpStoragePrice(s.ctx, &types.QuerySpStoragePriceRequest{SpAddr: sample.RandAccAddressHex()})
	requireInvalidArgument(s.T(), err) // unknown sp operator address

	opAddr := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
	sp := &types.StorageProvider{Id: 50, OperatorAddress: opAddr.String()}
	s.spKeeper.SetStorageProvider(s.ctx, sp)
	s.spKeeper.SetStorageProviderByOperatorAddr(s.ctx, sp)

	_, err = s.spKeeper.QuerySpStoragePrice(s.ctx, &types.QuerySpStoragePriceRequest{SpAddr: opAddr.String()})
	require.Error(s.T(), err) // sp exists, but no price has been set yet

	price := types.SpStoragePrice{SpId: sp.Id, ReadPrice: math.LegacyNewDec(1), StorePrice: math.LegacyNewDec(2)}
	s.spKeeper.SetSpStoragePrice(s.ctx, price)

	resp, err := s.spKeeper.QuerySpStoragePrice(s.ctx, &types.QuerySpStoragePriceRequest{SpAddr: opAddr.String()})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sp.Id, resp.SpStoragePrice.SpId)
}

func (s *KeeperTestSuite) TestQueryGlobalSpStorePriceByTime() {
	_, err := s.spKeeper.QueryGlobalSpStorePriceByTime(s.ctx, nil)
	requireInvalidArgument(s.T(), err)

	_, err = s.spKeeper.QueryGlobalSpStorePriceByTime(s.ctx, &types.QueryGlobalSpStorePriceByTimeRequest{Timestamp: -1})
	requireInvalidArgument(s.T(), err)

	ctx := s.ctx.WithBlockTime(time.Unix(1000, 0))
	_, err = s.spKeeper.QueryGlobalSpStorePriceByTime(ctx, &types.QueryGlobalSpStorePriceByTimeRequest{Timestamp: 0})
	require.Error(s.T(), err) // nothing stored yet

	price := types.GlobalSpStorePrice{UpdateTimeSec: 500, PrimaryStorePrice: math.LegacyNewDec(1)}
	s.spKeeper.SetGlobalSpStorePrice(ctx, price)

	resp, err := s.spKeeper.QueryGlobalSpStorePriceByTime(ctx, &types.QueryGlobalSpStorePriceByTimeRequest{Timestamp: 600})
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(500), resp.GlobalSpStorePrice.UpdateTimeSec)

	// Timestamp == 0 defaults to ctx.BlockTime().Unix()+1.
	resp, err = s.spKeeper.QueryGlobalSpStorePriceByTime(ctx, &types.QueryGlobalSpStorePriceByTimeRequest{Timestamp: 0})
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(500), resp.GlobalSpStorePrice.UpdateTimeSec)
}

func (s *KeeperTestSuite) TestStorageProviderQuery() {
	_, err := s.spKeeper.StorageProvider(gocontext.Background(), nil)
	requireInvalidArgument(s.T(), err)

	_, err = s.spKeeper.StorageProvider(s.ctx, &types.QueryStorageProviderRequest{Id: 999999})
	require.ErrorIs(s.T(), err, types.ErrStorageProviderNotFound)

	sp := &types.StorageProvider{Id: 60, OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String()}
	s.spKeeper.SetStorageProvider(s.ctx, sp)

	resp, err := s.spKeeper.StorageProvider(s.ctx, &types.QueryStorageProviderRequest{Id: 60})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sp.Id, resp.StorageProvider.Id)
}

func (s *KeeperTestSuite) TestStorageProviderByOperatorAddressQuery() {
	_, err := s.spKeeper.StorageProviderByOperatorAddress(gocontext.Background(), nil)
	requireInvalidArgument(s.T(), err)

	_, err = s.spKeeper.StorageProviderByOperatorAddress(s.ctx, &types.QueryStorageProviderByOperatorAddressRequest{OperatorAddress: "not-a-hex-address"})
	require.Error(s.T(), err)

	_, err = s.spKeeper.StorageProviderByOperatorAddress(s.ctx, &types.QueryStorageProviderByOperatorAddressRequest{OperatorAddress: sample.RandAccAddressHex()})
	require.ErrorIs(s.T(), err, types.ErrStorageProviderNotFound)

	opAddr := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
	sp := &types.StorageProvider{Id: 70, OperatorAddress: opAddr.String()}
	s.spKeeper.SetStorageProvider(s.ctx, sp)
	s.spKeeper.SetStorageProviderByOperatorAddr(s.ctx, sp)

	resp, err := s.spKeeper.StorageProviderByOperatorAddress(s.ctx, &types.QueryStorageProviderByOperatorAddressRequest{OperatorAddress: opAddr.String()})
	require.NoError(s.T(), err)
	require.Equal(s.T(), sp.Id, resp.StorageProvider.Id)
}

// TestStorageProviderMaintenanceRecordsByOperatorAddressQueryGaps covers this
// RPC's own nil-request and invalid-address branches; its "records found" path
// is already covered via sp_status_test.go's maintenance-record tests.
func (s *KeeperTestSuite) TestStorageProviderMaintenanceRecordsByOperatorAddressQueryGaps() {
	_, err := s.spKeeper.StorageProviderMaintenanceRecordsByOperatorAddress(gocontext.Background(), nil)
	requireInvalidArgument(s.T(), err)

	_, err = s.spKeeper.StorageProviderMaintenanceRecordsByOperatorAddress(s.ctx, &types.QueryStorageProviderMaintenanceRecordsRequest{OperatorAddress: "not-a-hex-address"})
	requireInvalidArgument(s.T(), err)

	opAddr := sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
	resp, err := s.spKeeper.StorageProviderMaintenanceRecordsByOperatorAddress(s.ctx, &types.QueryStorageProviderMaintenanceRecordsRequest{OperatorAddress: opAddr.String()})
	require.NoError(s.T(), err)
	require.Empty(s.T(), resp.Records)
}

func (s *KeeperTestSuite) TestQueryParamsNilRequest() {
	_, err := s.spKeeper.Params(gocontext.Background(), nil)
	requireInvalidArgument(s.T(), err)
}
