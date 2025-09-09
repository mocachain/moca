package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/x/storage/types"
)

func TestGenesisState_Validate(t *testing.T) {
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
			desc: "valid genesis state",
			genState: &types.GenesisState{
				Params: types.Params{
					VersionedParams: types.VersionedParams{
						MaxSegmentSize:          20,
						RedundantDataChunkNum:   10,
						RedundantParityChunkNum: 8,
						MinChargeSize:           100,
					},
					MaxPayloadSize:                   2000,
					MaxBucketsPerAccount:             100,
					BscMirrorBucketRelayerFee:        "1",
					BscMirrorBucketAckRelayerFee:     "2",
					BscMirrorGroupRelayerFee:         "3",
					BscMirrorGroupAckRelayerFee:      "4",
					BscMirrorObjectRelayerFee:        "5",
					BscMirrorObjectAckRelayerFee:     "6",
					OpMirrorBucketRelayerFee:         "7",
					OpMirrorBucketAckRelayerFee:      "8",
					OpMirrorGroupRelayerFee:          "9",
					OpMirrorGroupAckRelayerFee:       "10",
					OpMirrorObjectRelayerFee:         "11",
					OpMirrorObjectAckRelayerFee:      "12",
					PolygonMirrorBucketRelayerFee:    "13",
					PolygonMirrorBucketAckRelayerFee: "14",
					PolygonMirrorObjectRelayerFee:    "15",
					PolygonMirrorObjectAckRelayerFee: "16",
					PolygonMirrorGroupRelayerFee:     "17",
					PolygonMirrorGroupAckRelayerFee:  "18",
					ScrollMirrorBucketRelayerFee:     "19",
					ScrollMirrorBucketAckRelayerFee:  "20",
					ScrollMirrorObjectRelayerFee:     "21",
					ScrollMirrorObjectAckRelayerFee:  "22",
					ScrollMirrorGroupRelayerFee:      "23",
					ScrollMirrorGroupAckRelayerFee:   "24",
					LineaMirrorBucketRelayerFee:      "25",
					LineaMirrorBucketAckRelayerFee:   "26",
					LineaMirrorObjectRelayerFee:      "27",
					LineaMirrorObjectAckRelayerFee:   "28",
					LineaMirrorGroupRelayerFee:       "29",
					LineaMirrorGroupAckRelayerFee:    "30",
					MantleMirrorBucketRelayerFee:     "31",
					MantleMirrorBucketAckRelayerFee:  "32",
					MantleMirrorObjectRelayerFee:     "33",
					MantleMirrorObjectAckRelayerFee:  "34",
					MantleMirrorGroupRelayerFee:      "35",
					MantleMirrorGroupAckRelayerFee:   "36",
					ArbitrumMirrorBucketRelayerFee:   "37",
					ArbitrumMirrorBucketAckRelayerFee: "38",
					ArbitrumMirrorObjectRelayerFee:   "39",
					ArbitrumMirrorObjectAckRelayerFee: "40",
					ArbitrumMirrorGroupRelayerFee:    "41",
					ArbitrumMirrorGroupAckRelayerFee: "42",
					OptimismMirrorBucketRelayerFee:   "43",
					OptimismMirrorBucketAckRelayerFee: "44",
					OptimismMirrorObjectRelayerFee:   "45",
					OptimismMirrorObjectAckRelayerFee: "46",
					OptimismMirrorGroupRelayerFee:    "47",
					OptimismMirrorGroupAckRelayerFee: "48",
					BaseMirrorBucketRelayerFee:       "49",
					BaseMirrorBucketAckRelayerFee:    "50",
					BaseMirrorObjectRelayerFee:       "51",
					BaseMirrorObjectAckRelayerFee:    "52",
					BaseMirrorGroupRelayerFee:        "53",
					BaseMirrorGroupAckRelayerFee:     "54",
					DiscontinueCountingWindow:        1000,
					DiscontinueObjectMax:             10000,
					DiscontinueBucketMax:             10000,
					DiscontinueConfirmPeriod:         100,
					DiscontinueDeletionMax:           10,
					StalePolicyCleanupMax:            10,
					MinQuotaUpdateInterval:           10000,
					MaxLocalVirtualGroupNumPerBucket: 100,
				},
			},
			valid: true,
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
