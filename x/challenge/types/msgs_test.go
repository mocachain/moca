package types

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/votepool"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/testutil/sample"
	gnfderrors "github.com/mocachain/moca/v2/types/errors"
)

// Shared test fixtures reused across this package's message tests.
const (
	testBucketName     = "bucket"
	testObjectName     = "object"
	testInvalidAddress = "invalid_address"
)

func TestNewMsgSubmit(t *testing.T) {
	challenger := sample.RandAccAddress()
	spOperatorAddress := sample.RandAccAddress()

	msg := NewMsgSubmit(challenger, spOperatorAddress, testBucketName, testObjectName, true, 5)

	require.Equal(t, challenger.String(), msg.Challenger)
	require.Equal(t, spOperatorAddress.String(), msg.SpOperatorAddress)
	require.Equal(t, testBucketName, msg.BucketName)
	require.Equal(t, testObjectName, msg.ObjectName)
	require.True(t, msg.RandomIndex)
	require.Equal(t, uint32(5), msg.SegmentIndex)
}

func TestMsgSubmit_RouteAndType(t *testing.T) {
	msg := MsgSubmit{}
	require.Equal(t, RouterKey, msg.Route())
	require.Equal(t, TypeMsgSubmit, msg.Type())
}

func TestMsgSubmit_GetSigners(t *testing.T) {
	challenger := sample.RandAccAddress()
	msg := MsgSubmit{Challenger: challenger.String()}

	signers := msg.GetSigners()
	require.Len(t, signers, 1)
	require.Equal(t, challenger, signers[0])

	invalid := MsgSubmit{Challenger: testInvalidAddress}
	require.Panics(t, func() { invalid.GetSigners() })
}

func TestMsgSubmit_GetSignBytes(t *testing.T) {
	msg := MsgSubmit{
		Challenger:        sample.RandAccAddressHex(),
		SpOperatorAddress: sample.RandAccAddressHex(),
		BucketName:        testBucketName,
		ObjectName:        testObjectName,
	}

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)

	var decoded MsgSubmit
	require.NoError(t, ModuleCdc.UnmarshalJSON(bz, &decoded))
	require.Equal(t, msg.Challenger, decoded.Challenger)
	require.Equal(t, msg.BucketName, decoded.BucketName)
	require.Equal(t, msg.ObjectName, decoded.ObjectName)
}

func TestMsgSubmit_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgSubmit
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgSubmit{
				Challenger: testInvalidAddress,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid sp operator address",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: testInvalidAddress,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid bucket name",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        "1",
			},
			err: gnfderrors.ErrInvalidBucketName,
		}, {
			name: "invalid object name",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        "",
			},
			err: gnfderrors.ErrInvalidObjectName,
		}, {
			name: "valid message with random index",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        testObjectName,
				RandomIndex:       true,
				SegmentIndex:      10,
			},
		}, {
			name: "valid message with specific index",
			msg: MsgSubmit{
				Challenger:        sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				BucketName:        testBucketName,
				ObjectName:        testObjectName,
				RandomIndex:       false,
				SegmentIndex:      2,
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

	invalid := MsgAttest{Submitter: testInvalidAddress}
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
				Submitter: testInvalidAddress,
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid sp operator address",
			msg: MsgAttest{
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: testInvalidAddress,
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
				ChallengerAddress: testInvalidAddress,
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

	// Assert against the vote pool's own preimage so the two cannot drift unnoticed.
	want := (&votepool.Vote{EventType: votepool.DataAvailabilityChallengeEvent, EventHash: eventHash[:]}).SignBytes()

	require.Equal(t, want, msg.GetVotePoolSignBytes(chainID),
		"x/challenge and votepool must sign byte-identical payloads")

	// The event type has to actually be bound in, otherwise the whole point of
	// the change is lost and this test would pass against the bare hash.
	require.NotEqual(t, eventHash[:], msg.GetVotePoolSignBytes(chainID),
		"the preimage must not be the bare event hash")
}

func TestMsgUpdateParams_GetSignBytes(t *testing.T) {
	msg := MsgUpdateParams{
		Authority: sample.RandAccAddressHex(),
		Params:    DefaultParams(),
	}

	bz := msg.GetSignBytes()
	require.NotEmpty(t, bz)

	var decoded MsgUpdateParams
	require.NoError(t, ModuleCdc.UnmarshalJSON(bz, &decoded))
	require.Equal(t, msg.Authority, decoded.Authority)
}

func TestMsgUpdateParams_GetSigners(t *testing.T) {
	authority := sample.RandAccAddress()
	msg := MsgUpdateParams{Authority: authority.String()}

	signers := msg.GetSigners()
	require.Len(t, signers, 1)
	require.Equal(t, authority, signers[0])
}

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	wrongParams := DefaultParams()
	wrongParams.HeartbeatInterval = 0

	tests := []struct {
		name string
		msg  MsgUpdateParams
		err  error
	}{
		{
			name: "invalid authority",
			msg: MsgUpdateParams{
				Authority: testInvalidAddress,
				Params:    DefaultParams(),
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "invalid params",
			msg: MsgUpdateParams{
				Authority: sample.RandAccAddressHex(),
				Params:    wrongParams,
			},
			err: ErrInvalidParams,
		}, {
			name: "valid authority and params",
			msg: MsgUpdateParams{
				Authority: sample.RandAccAddressHex(),
				Params:    DefaultParams(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
