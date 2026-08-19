package types

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/votepool"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/testutil/sample"
)

func TestMsgAttest_ValidateBasic(t *testing.T) {
	var sig [BlsSignatureLength]byte
	tests := []struct {
		name string
		msg  MsgAttest
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgAttest{
				Submitter: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid vote result",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				VoteResult:        100,
			},
			err: ErrInvalidVoteResult,
		}, {
			name: "invalid vote result",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				VoteResult:        CHALLENGE_SUCCEED,
				VoteValidatorSet:  make([]uint64, 0),
			},
			err: ErrInvalidVoteValidatorSet,
		}, {
			name: "invalid vote aggregated signature",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				VoteResult:        CHALLENGE_SUCCEED,
				VoteValidatorSet:  []uint64{1},
				VoteAggSignature:  []byte{1, 2, 3},
			},
			err: ErrInvalidVoteAggSignature,
		}, {
			name: "valid message",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				VoteResult:        CHALLENGE_SUCCEED,
				VoteValidatorSet:  []uint64{1},
				VoteAggSignature:  sig[:],
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// The vote pool and this module verify the SAME signature: the pool checks each
// vote on gossip, and x/challenge checks the aggregate of those votes. If the
// two preimages ever diverge, no signer can satisfy both and attestations stop
// reaching quorum, so pin them against each other rather than against a
// hardcoded digest.
func TestMsgAttest_GetVotePoolSignBytesMatchesVotePool(t *testing.T) {
	msg := MsgAttest{
		Submitter:         sample.RandAccAddressHex(),
		ChallengeId:       42,
		ObjectId:          math.NewUint(1234567890),
		SpOperatorAddress: sample.RandAccAddressHex(),
		VoteResult:        CHALLENGE_SUCCEED,
		ChallengerAddress: sample.RandAccAddressHex(),
	}
	const chainID = "moca_1-1"

	eventHash := msg.GetBlsSignBytes(chainID)

	// Mirrors votepool.Vote.SignBytes. Kept inline only because the pinned
	// cometbft does not export it yet; once the vote-pool side lands, assert
	// against vote.SignBytes() directly so the two cannot drift unnoticed.
	want := crypto.Keccak256(append([]byte{byte(votepool.DataAvailabilityChallengeEvent)}, eventHash[:]...))

	require.Equal(t, want, msg.GetVotePoolSignBytes(chainID),
		"x/challenge and votepool must sign byte-identical payloads")

	// The event type has to actually be bound in, otherwise the whole point of
	// the change is lost and this test would pass against the bare hash.
	require.NotEqual(t, eventHash[:], msg.GetVotePoolSignBytes(chainID),
		"the preimage must not be the bare event hash")
}
