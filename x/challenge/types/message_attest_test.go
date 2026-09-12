package types

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/votepool"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
)

func TestNewMsgAttest(t *testing.T) {
	submitter := sample.RandAccAddress()
	objectID := math.NewUint(7)
	voteValidatorSet := []uint64{1, 2}
	sig := []byte{1, 2, 3}

	msg := NewMsgAttest(submitter, 3, objectID, sample.RandAccAddressHex(), CHALLENGE_SUCCEED, "challenger", voteValidatorSet, sig)

	require.Equal(t, submitter.String(), msg.Submitter)
	require.Equal(t, uint64(3), msg.ChallengeId)
	require.Equal(t, objectID, msg.ObjectId)
	require.Equal(t, CHALLENGE_SUCCEED, msg.VoteResult)
	require.Equal(t, "challenger", msg.ChallengerAddress)
	require.Equal(t, voteValidatorSet, msg.VoteValidatorSet)
	require.Equal(t, sig, msg.VoteAggSignature)
}

func TestMsgAttest_RouteAndType(t *testing.T) {
	msg := MsgAttest{}
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgAttest, msg.Type())
}

func TestMsgAttest_GetSigners(t *testing.T) {
	submitter := sample.RandAccAddress()
	msg := MsgAttest{Submitter: submitter.String()}

	signers := msg.GetSigners()
	require.Len(t, signers, 1)
	require.Equal(t, submitter, signers[0])

	invalid := MsgAttest{Submitter: "invalid_address"}
	require.Panics(t, func() { invalid.GetSigners() })
}

func TestMsgAttest_GetSignBytes(t *testing.T) {
	msg := MsgAttest{
		Submitter:         sample.RandAccAddressHex(),
		ChallengeId:       7,
		SpOperatorAddress: sample.RandAccAddressHex(),
	}

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)

	var decoded MsgAttest
	require.NoError(t, ModuleCdc.UnmarshalJSON(bz, &decoded))
	require.Equal(t, msg.Submitter, decoded.Submitter)
	require.Equal(t, msg.ChallengeId, decoded.ChallengeId)
	require.Equal(t, msg.SpOperatorAddress, decoded.SpOperatorAddress)
}

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
			name: "invalid sp operator address",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: "invalid_address",
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
			name: "invalid challenger address",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				VoteResult:        CHALLENGE_SUCCEED,
				ChallengerAddress: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
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
		}, {
			name: "valid message with challenger address",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				VoteResult:        CHALLENGE_SUCCEED,
				ChallengerAddress: sample.RandAccAddressHex(),
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
