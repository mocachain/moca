package keeper_test

import (
	"reflect"
	"testing"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func (s *KeeperTestSuite) TestGetSpStoragePriceByTime() {
	ctx := s.ctx.WithBlockTime(time.Unix(100, 0))
	spID := uint32(10)

	_, found := s.spKeeper.GetSpStoragePrice(ctx, spID)
	s.Require().True(!found)

	spStoragePrice := types.SpStoragePrice{
		SpId:          spID,
		UpdateTimeSec: 1,
		ReadPrice:     math.LegacyNewDec(100),
		StorePrice:    math.LegacyNewDec(100),
	}
	s.spKeeper.SetSpStoragePrice(ctx, spStoragePrice)

	price, found := s.spKeeper.GetSpStoragePrice(ctx, spID)
	s.Require().True(found)
	s.Require().True(reflect.DeepEqual(price, spStoragePrice))

	spStoragePrice2 := types.SpStoragePrice{
		SpId:          spID,
		UpdateTimeSec: 100,
		ReadPrice:     math.LegacyNewDec(200),
		StorePrice:    math.LegacyNewDec(200),
	}
	s.spKeeper.SetSpStoragePrice(ctx, spStoragePrice2)

	price, found = s.spKeeper.GetSpStoragePrice(ctx, spID)
	s.Require().True(found)
	s.Require().True(reflect.DeepEqual(price, spStoragePrice2))
}

func (s *KeeperTestSuite) TestGetGlobalSpStorePriceByTime() {
	keeper := s.spKeeper
	ctx := s.ctx
	secondarySpStorePrice := types.GlobalSpStorePrice{
		UpdateTimeSec:       1,
		PrimaryStorePrice:   math.LegacyNewDec(100),
		SecondaryStorePrice: math.LegacyNewDec(40),
		ReadPrice:           math.LegacyNewDec(80),
	}
	keeper.SetGlobalSpStorePrice(ctx, secondarySpStorePrice)
	secondarySpStorePrice2 := types.GlobalSpStorePrice{
		UpdateTimeSec:       100,
		PrimaryStorePrice:   math.LegacyNewDec(200),
		SecondaryStorePrice: math.LegacyNewDec(70),
		ReadPrice:           math.LegacyNewDec(90),
	}
	keeper.SetGlobalSpStorePrice(ctx, secondarySpStorePrice2)
	type args struct {
		time int64
	}
	tests := []struct {
		name    string
		args    args
		wantVal types.GlobalSpStorePrice
		wantErr bool
	}{
		{"test 0", args{time: 0}, types.GlobalSpStorePrice{}, true},
		{"test 1", args{time: 1}, types.GlobalSpStorePrice{}, true},
		{"test 2", args{time: 2}, secondarySpStorePrice, false},
		{"test 100", args{time: 100}, secondarySpStorePrice, false},
		{"test 101", args{time: 101}, secondarySpStorePrice2, false},
	}
	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			gotVal, err := keeper.GetGlobalSpStorePriceByTime(ctx, tt.args.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpStoragePriceByTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotVal, tt.wantVal) {
				t.Errorf("GetSpStoragePriceByTime() gotVal = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}

func (s *KeeperTestSuite) TestGetAllSpStoragePrice() {
	k := s.spKeeper
	ctx := s.ctx

	want := map[uint32]math.LegacyDec{}
	for i, id := range []uint32{1, 2, 3} {
		price := types.SpStoragePrice{
			SpId:       id,
			ReadPrice:  math.LegacyNewDec(int64(i + 1)),
			StorePrice: math.LegacyNewDec(int64(i + 1)),
		}
		k.SetSpStoragePrice(ctx, price)
		want[id] = price.ReadPrice
	}

	all := k.GetAllSpStoragePrice(ctx)
	require.Len(s.T(), all, len(want))
	for _, p := range all {
		require.True(s.T(), want[p.SpId].Equal(p.ReadPrice))
	}
}

// TestUpdateGlobalSpStorePriceNoStorageProviders covers the zero-SPs no-op: no
// global price is written, so a subsequent lookup still errors.
func (s *KeeperTestSuite) TestUpdateGlobalSpStorePriceNoStorageProviders() {
	err := s.spKeeper.UpdateGlobalSpStorePrice(s.ctx)
	require.NoError(s.T(), err)

	_, err = s.spKeeper.GetGlobalSpStorePriceByTime(s.ctx, s.ctx.BlockTime().Unix())
	require.Error(s.T(), err)
}

// TestUpdateGlobalSpStorePriceMissingPrice covers the not-found-price error: an
// in-service SP that never had SetSpStoragePrice called for it.
func (s *KeeperTestSuite) TestUpdateGlobalSpStorePriceMissingPrice() {
	k := s.spKeeper
	ctx := s.ctx
	sp := &types.StorageProvider{
		Id:              10,
		OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
		Status:          types.STATUS_IN_SERVICE,
	}
	k.SetStorageProvider(ctx, sp)

	err := k.UpdateGlobalSpStorePrice(ctx)
	require.Error(s.T(), err)
}

// TestUpdateGlobalSpStorePriceMedian covers calculateMedian's odd and even
// branches via 3 and then 4 priced storage providers (STATUS_IN_MAINTENANCE
// counts toward the median alongside STATUS_IN_SERVICE).
func (s *KeeperTestSuite) TestUpdateGlobalSpStorePriceMedian() {
	k := s.spKeeper
	ctx := s.ctx.WithBlockTime(time.Unix(5000, 0))

	seed := func(id uint32, status types.Status, price int64) {
		sp := &types.StorageProvider{
			Id:              id,
			OperatorAddress: sdk.MustAccAddressFromHex(sample.RandAccAddressHex()).String(),
			Status:          status,
		}
		k.SetStorageProvider(ctx, sp)
		k.SetSpStoragePrice(ctx, types.SpStoragePrice{SpId: id, ReadPrice: math.LegacyNewDec(price), StorePrice: math.LegacyNewDec(price)})
	}

	// Odd count (3): median of {30, 10, 20} is 20.
	seed(100, types.STATUS_IN_SERVICE, 30)
	seed(101, types.STATUS_IN_SERVICE, 10)
	seed(102, types.STATUS_IN_SERVICE, 20)

	require.NoError(s.T(), k.UpdateGlobalSpStorePrice(ctx))
	got, err := k.GetGlobalSpStorePriceByTime(ctx, ctx.BlockTime().Unix()+1)
	require.NoError(s.T(), err)
	require.True(s.T(), math.LegacyNewDec(20).Equal(got.PrimaryStorePrice))
	require.True(s.T(), types.DefaultSecondarySpStorePriceRatio.MulInt64(20).Equal(got.SecondaryStorePrice))

	// Even count (4): median of {30, 10, 20, 40} is (20+30)/2 = 25.
	seed(103, types.STATUS_IN_MAINTENANCE, 40)

	ctx2 := ctx.WithBlockTime(ctx.BlockTime().Add(time.Second))
	require.NoError(s.T(), k.UpdateGlobalSpStorePrice(ctx2))
	got2, err := k.GetGlobalSpStorePriceByTime(ctx2, ctx2.BlockTime().Unix()+1)
	require.NoError(s.T(), err)
	require.True(s.T(), math.LegacyNewDec(25).Equal(got2.PrimaryStorePrice))
}
