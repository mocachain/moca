package keeper

import (
	"cosmossdk.io/errors"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/bits-and-blooms/bitset"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/mocachain/moca/v2/x/challenge/types"
)

// BlsSignedMsg defined the interface of a bls signed message.
type BlsSignedMsg interface {
	// GetBlsSignBytes returns the bls signed message in bytes.
	GetBlsSignBytes(chainID string) [32]byte

	// GetVotePoolSignBytes returns the same message in the form the vote pool
	// signs it, with the event type bound into the preimage.
	GetVotePoolSignBytes(chainID string) []byte

	// GetVoteValidatorSet returns the validators who signed the message.
	GetVoteValidatorSet() []uint64

	// GetVoteAggSignature returns the aggregated bls signature.
	GetVoteAggSignature() []byte
}

// verifySignature verifies whether the signature is valid or not.
func (k Keeper) verifySignature(ctx sdk.Context, signedMsg BlsSignedMsg, validators []stakingtypes.Validator) ([]string, error) {
	validatorsBitSet := bitset.From(signedMsg.GetVoteValidatorSet())
	if validatorsBitSet.Count() > uint(len(validators)) {
		return nil, errors.Wrap(types.ErrInvalidVoteValidatorSet, "number of validator set is larger than validators")
	}

	signedChallengers := make([]string, 0, validatorsBitSet.Count())
	votedPubKeys := make([]*bls.PublicKey, 0, validatorsBitSet.Count())
	for index, val := range validators {
		if !validatorsBitSet.Test(uint(index)) {
			continue
		}

		signedChallengers = append(signedChallengers, val.ChallengerAddress)
		votePubKey, err := bls.UnmarshalPublicKey(val.BlsKey)
		if err != nil {
			return nil, errors.Wrapf(types.ErrInvalidBlsPubKey, "BLS public key converts failed: %v", err)
		}
		votedPubKeys = append(votedPubKeys, votePubKey)
	}

	if len(votedPubKeys) <= len(validators)*2/3 {
		return nil, errors.Wrapf(types.ErrNotEnoughVotes, "Not enough validators voted, need: %d, voted: %d", len(validators)*2/3, len(votedPubKeys))
	}

	aggSig, err := bls.UnmarshalSignature(signedMsg.GetVoteAggSignature())
	if err != nil {
		return nil, errors.Wrapf(types.ErrInvalidVoteAggSignature, "BLS signature converts failed: %v", err)
	}

	// Only the event-type-bound payload is accepted. Attestations signed the old
	// way are rejected outright rather than tolerated for a migration window,
	// because a window that accepts both leaves the old form valid for as long
	// as it lasts. Challengers must be restarted with the matching build at the
	// upgrade height; they re-sign every still-open challenge on restart.
	if !aggSig.VerifyAggregated(votedPubKeys, signedMsg.GetVotePoolSignBytes(ctx.ChainID()), votepool.DST) {
		return nil, errors.Wrap(types.ErrInvalidVoteAggSignature, "Signature verify failed")
	}

	return signedChallengers, nil
}
