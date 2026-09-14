// Package cli (internal, unlike this package's other _test.go files) because
// the toPbXxx/toSppPageReq conversion helpers under test here are unexported.
package cli

import (
	"context"
	"math/big"
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"

	spp "github.com/mocachain/moca/v2/precompiles/storageprovider"
	"github.com/mocachain/moca/v2/x/sp/types"
)

func TestNewQueryClientEVM(t *testing.T) {
	qc := NewQueryClientEVM(nil)
	require.NotNil(t, qc)

	c, ok := qc.(*QueryClientEVM)
	require.True(t, ok)
	require.Nil(t, c.cc)
}

func TestToPbDescription(t *testing.T) {
	require.Nil(t, toPbDescription(nil))

	d := &spp.Description{
		Moniker:         FlagMoniker,
		Identity:        "identity",
		Website:         "https://example.com",
		SecurityContact: "security@example.com",
		Details:         FlagDetails,
	}
	require.Equal(t, &types.Description{
		Moniker:         d.Moniker,
		Identity:        d.Identity,
		Website:         d.Website,
		SecurityContact: d.SecurityContact,
		Details:         d.Details,
	}, toPbDescription(d))
}

func TestToPbSP(t *testing.T) {
	require.Nil(t, toPbSP(nil))

	p := &spp.StorageProvider{
		Id:                 7,
		OperatorAddress:    "0x1111111111111111111111111111111111111a",
		FundingAddress:     "0x2222222222222222222222222222222222222b",
		SealAddress:        "0x3333333333333333333333333333333333333c",
		ApprovalAddress:    "0x4444444444444444444444444444444444444d",
		GcAddress:          "0x5555555555555555555555555555555555555e",
		MaintenanceAddress: "0x6666666666666666666666666666666666666f",
		TotalDeposit:       big.NewInt(123456),
		Status:             2,
		Endpoint:           "https://sp.example.com",
		Description: spp.Description{
			Moniker: FlagMoniker,
			Details: FlagDetails,
		},
		BlsKey: "blskeybytes",
	}

	got := toPbSP(p)
	require.Equal(t, &types.StorageProvider{
		Id:                 p.Id,
		OperatorAddress:    p.OperatorAddress,
		FundingAddress:     p.FundingAddress,
		SealAddress:        p.SealAddress,
		ApprovalAddress:    p.ApprovalAddress,
		GcAddress:          p.GcAddress,
		MaintenanceAddress: p.MaintenanceAddress,
		TotalDeposit:       math.NewIntFromBigInt(p.TotalDeposit),
		Status:             types.Status(p.Status),
		Endpoint:           p.Endpoint,
		Description: types.Description{
			Moniker: FlagMoniker,
			Details: FlagDetails,
		},
		BlsKey: []byte(p.BlsKey),
	}, got)
}

func TestToSppPageReq(t *testing.T) {
	require.Nil(t, toSppPageReq(nil))

	in := &query.PageRequest{
		Key:        []byte("some-key"),
		Offset:     3,
		Limit:      10,
		CountTotal: true,
		Reverse:    true,
	}
	require.Equal(t, &spp.PageRequest{
		Key:        in.Key,
		Offset:     in.Offset,
		Limit:      in.Limit,
		CountTotal: in.CountTotal,
		Reverse:    in.Reverse,
	}, toSppPageReq(in))
}

func TestToPbPageResp(t *testing.T) {
	require.Nil(t, toPbPageResp(nil))

	in := &spp.PageResponse{
		NextKey: []byte("next-key"),
		Total:   7,
	}
	require.Equal(t, &query.PageResponse{
		NextKey: in.NextKey,
		Total:   in.Total,
	}, toPbPageResp(in))
}

func TestToPbPrice(t *testing.T) {
	require.Nil(t, toPbPrice(nil))

	in := &spp.SpStoragePrice{
		SpId:          3,
		UpdateTimeSec: big.NewInt(1234),
		ReadPrice:     big.NewInt(500),
		FreeReadQuota: 999,
		StorePrice:    big.NewInt(700),
	}
	require.Equal(t, &types.SpStoragePrice{
		SpId:          in.SpId,
		UpdateTimeSec: in.UpdateTimeSec.Int64(),
		ReadPrice:     math.LegacyNewDecFromInt(math.NewIntFromBigInt(in.ReadPrice)),
		FreeReadQuota: in.FreeReadQuota,
		StorePrice:    math.LegacyNewDecFromInt(math.NewIntFromBigInt(in.StorePrice)),
	}, toPbPrice(in))
}

// TestQueryClientEVM_TrivialMethods covers the three QueryClientEVM methods
// that are always no-ops (they never dial out): each unconditionally returns
// (nil, nil) regardless of the request or the underlying *ethclient.Client.
func TestQueryClientEVM_TrivialMethods(t *testing.T) {
	c := NewQueryClientEVM(nil)
	ctx := context.Background()

	paramsResp, err := c.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Nil(t, paramsResp)

	globalPriceResp, err := c.QueryGlobalSpStorePriceByTime(ctx, &types.QueryGlobalSpStorePriceByTimeRequest{Timestamp: 100})
	require.NoError(t, err)
	require.Nil(t, globalPriceResp)

	maintenanceResp, err := c.StorageProviderMaintenanceRecordsByOperatorAddress(ctx, &types.QueryStorageProviderMaintenanceRecordsRequest{OperatorAddress: "0xabc"})
	require.NoError(t, err)
	require.Nil(t, maintenanceResp)
}
