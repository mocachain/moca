package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func TestDefaultGenesis(t *testing.T) {
	gs := types.DefaultGenesis()

	require.Equal(t, types.DefaultParams(), gs.Params)
	require.Empty(t, gs.StreamRecordList)
	require.Empty(t, gs.PaymentAccountCountList)
	require.Empty(t, gs.PaymentAccountList)
	require.Empty(t, gs.AutoSettleRecordList)
}

func TestGenesisState_Validate(t *testing.T) {
	addr1 := sample.RandAccAddressHex()
	addr2 := sample.RandAccAddressHex()

	invalidParams := types.DefaultParams()
	invalidParams.ForcedSettleTime = 0

	for _, tc := range []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "valid genesis with populated lists",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				StreamRecordList: []types.StreamRecord{
					{Account: addr1},
					{Account: addr2},
				},
				PaymentAccountCountList: []types.PaymentAccountCount{
					{Owner: addr1},
					{Owner: addr2},
				},
				PaymentAccountList: []types.PaymentAccount{
					{Addr: addr1},
					{Addr: addr2},
				},
				AutoSettleRecordList: []types.AutoSettleRecord{
					{Addr: addr1, Timestamp: 1},
					{Addr: addr2, Timestamp: 2},
				},
			},
			valid: true,
		},
		{
			desc: "duplicated streamRecord",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				StreamRecordList: []types.StreamRecord{
					{Account: addr1},
					{Account: addr1},
				},
			},
			valid: false,
		},
		{
			desc: "duplicated paymentAccountCount",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				PaymentAccountCountList: []types.PaymentAccountCount{
					{Owner: addr1},
					{Owner: addr1},
				},
			},
			valid: false,
		},
		{
			desc: "duplicated paymentAccount",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				PaymentAccountList: []types.PaymentAccount{
					{Addr: addr1},
					{Addr: addr1},
				},
			},
			valid: false,
		},
		{
			desc: "duplicated autoSettleRecord",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				AutoSettleRecordList: []types.AutoSettleRecord{
					{Addr: addr1, Timestamp: 1},
					{Addr: addr1, Timestamp: 1},
				},
			},
			valid: false,
		},
		{
			desc: "invalid params",
			genState: &types.GenesisState{
				Params: invalidParams,
			},
			valid: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
