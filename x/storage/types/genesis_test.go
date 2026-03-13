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
					PolygonMirrorGroupRelayerFee:     "15",
					PolygonMirrorGroupAckRelayerFee:  "16",
					PolygonMirrorObjectRelayerFee:    "17",
					PolygonMirrorObjectAckRelayerFee: "18",
					ScrollMirrorBucketRelayerFee:     "19",
					ScrollMirrorBucketAckRelayerFee:  "20",
					ScrollMirrorGroupRelayerFee:      "21",
					ScrollMirrorGroupAckRelayerFee:   "22",
					ScrollMirrorObjectRelayerFee:     "23",
					ScrollMirrorObjectAckRelayerFee:  "24",
					LineaMirrorBucketRelayerFee:      "25",
					LineaMirrorBucketAckRelayerFee:   "26",
					LineaMirrorGroupRelayerFee:       "27",
					LineaMirrorGroupAckRelayerFee:    "28",
					LineaMirrorObjectRelayerFee:      "29",
					LineaMirrorObjectAckRelayerFee:   "30",
					MantleMirrorBucketRelayerFee:     "31",
					MantleMirrorBucketAckRelayerFee:  "32",
					MantleMirrorGroupRelayerFee:      "33",
					MantleMirrorGroupAckRelayerFee:   "34",
					MantleMirrorObjectRelayerFee:     "35",
					MantleMirrorObjectAckRelayerFee:  "36",
					ArbitrumMirrorBucketRelayerFee:    "37",
					ArbitrumMirrorBucketAckRelayerFee: "38",
					ArbitrumMirrorGroupRelayerFee:     "39",
					ArbitrumMirrorGroupAckRelayerFee:  "40",
					ArbitrumMirrorObjectRelayerFee:    "41",
					ArbitrumMirrorObjectAckRelayerFee: "42",
					OptimismMirrorBucketRelayerFee:    "43",
					OptimismMirrorBucketAckRelayerFee: "44",
					OptimismMirrorGroupRelayerFee:     "45",
					OptimismMirrorGroupAckRelayerFee:  "46",
					OptimismMirrorObjectRelayerFee:    "47",
					OptimismMirrorObjectAckRelayerFee: "48",
					BaseMirrorBucketRelayerFee:        "49",
					BaseMirrorBucketAckRelayerFee:     "50",
					BaseMirrorGroupRelayerFee:         "51",
					BaseMirrorGroupAckRelayerFee:      "52",
					BaseMirrorObjectRelayerFee:        "53",
					BaseMirrorObjectAckRelayerFee:     "54",
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
