package keeper

import (
	"encoding/binary"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/x/challenge/types"
)

// GetChallengeId gets the challenge id
func (k Keeper) GetChallengeId(ctx sdk.Context) uint64 { //nolint
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte{})
	bz := store.Get(types.ChallengeIDKey)

	if bz == nil {
		return 0
	}

	return binary.BigEndian.Uint64(bz)
}

// setChallengeID sets the new challenge id to the store
func (k Keeper) setChallengeID(ctx sdk.Context, challengeID uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), []byte{})
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, challengeID)
	store.Set(types.ChallengeIDKey, bz)
}

// SaveChallenge set a specific challenge in the store
func (k Keeper) SaveChallenge(ctx sdk.Context, challenge types.Challenge) {
	k.setChallengeID(ctx, challenge.Id)

	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeKeyPrefix)

	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, challenge.ExpiredHeight)

	store.Set(getChallengeKeyBytes(challenge.Id), heightBytes)
}

// SaveChallengeSpID records which sp a challenge was raised against. The attestation resolves
// the sp from this binding instead of from live state, so the sp cannot escape the slash by
// leaving the virtual group that stores the object.
func (k Keeper) SaveChallengeSpID(ctx sdk.Context, challengeID uint64, spID uint32) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeSpKeyPrefix)

	spBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(spBytes, spID)

	store.Set(getChallengeKeyBytes(challengeID), spBytes)
}

// GetChallengeSpID returns the sp a challenge was raised against. Challenges created before this
// binding existed have none, in which case the caller falls back to resolving the sp from state.
func (k Keeper) GetChallengeSpID(ctx sdk.Context, challengeID uint64) (uint32, bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeSpKeyPrefix)

	bz := store.Get(getChallengeKeyBytes(challengeID))
	if bz == nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(bz), true
}

// RemoveChallengeUntil removes challenges which are expired
func (k Keeper) RemoveChallengeUntil(ctx sdk.Context, height uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeKeyPrefix)
	spStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeSpKeyPrefix)
	iterator := storetypes.KVStorePrefixIterator(store, []byte{})
	defer iterator.Close()

	// Record the sps in the order the iterator yields them, which is the same on every node,
	// and use the set only to skip repeats: ranging a map would visit them in a random order.
	var affected []uint32
	seen := make(map[uint32]struct{})
	for ; iterator.Valid(); iterator.Next() {
		expiredHeight := binary.BigEndian.Uint64(iterator.Value())
		if expiredHeight <= height {
			if bz := spStore.Get(iterator.Key()); bz != nil {
				spID := binary.BigEndian.Uint32(bz)
				if _, ok := seen[spID]; !ok {
					seen[spID] = struct{}{}
					affected = append(affected, spID)
				}
			}
			store.Delete(iterator.Key())
			spStore.Delete(iterator.Key())
		}
	}
	for _, spID := range affected {
		k.releaseDepositLock(ctx, spID)
	}
}

// RemoveChallenge retires an attested challenge from the active set, making re-attestation idempotent.
func (k Keeper) RemoveChallenge(ctx sdk.Context, challengeID uint64) {
	spID, bound := k.GetChallengeSpID(ctx, challengeID)

	key := getChallengeKeyBytes(challengeID)
	prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeKeyPrefix).Delete(key)
	prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeSpKeyPrefix).Delete(key)

	if bound {
		k.releaseDepositLock(ctx, spID)
	}
}

// ExistsChallenge check whether there exists ongoing challenge for an id
func (k Keeper) ExistsChallenge(ctx sdk.Context, challengeID uint64) bool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeKeyPrefix)

	return store.Has(getChallengeKeyBytes(challengeID))
}

// getChallengeKeyBytes returns the byte representation of challenge key
func getChallengeKeyBytes(challengeID uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, challengeID)
	return bz
}

func (k Keeper) encodeUint64(data uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, data)
	return bz
}

// GetAttestedChallenges gets the latest attested challenges
func (k Keeper) GetAttestedChallenges(ctx sdk.Context) []*types.AttestedChallenge {
	store := ctx.KVStore(k.storeKey)
	sizeBz := store.Get(types.AttestedChallengesSizeKey)

	if sizeBz == nil {
		return []*types.AttestedChallenge{}
	}

	size := binary.BigEndian.Uint64(sizeBz)
	cursor := binary.BigEndian.Uint64(store.Get(types.AttestedChallengesCursorKey))

	result := []*types.AttestedChallenge{}
	current := cursor
	challengeStore := prefix.NewStore(store, types.AttestedChallengesPrefix)
	for {
		current = (current + 1) % size
		challengeBz := challengeStore.Get(k.encodeUint64(current))
		if challengeBz != nil {
			var challenge types.AttestedChallenge
			k.cdc.MustUnmarshal(challengeBz, &challenge)
			result = append(result, &challenge)
		}
		if current == cursor {
			break
		}
	}
	return result
}

// AppendAttestedChallenge sets the new id of challenge to the store
func (k Keeper) AppendAttestedChallenge(ctx sdk.Context, challenge *types.AttestedChallenge) {
	toKeep := k.GetParams(ctx).AttestationKeptCount

	store := ctx.KVStore(k.storeKey)
	sizeBz := store.Get(types.AttestedChallengesSizeKey)

	challengeStore := prefix.NewStore(store, types.AttestedChallengesPrefix)
	if sizeBz == nil { // the first time to append
		store.Set(types.AttestedChallengesSizeKey, k.encodeUint64(toKeep))
		k.enqueueAttestedChallenge(store, challengeStore, challenge)
		return
	}

	size := binary.BigEndian.Uint64(sizeBz)
	if size != toKeep { // the parameter changes, which is not frequent
		currentChallenges := k.GetAttestedChallenges(ctx)

		iterator := storetypes.KVStorePrefixIterator(challengeStore, []byte{})
		defer iterator.Close()

		for ; iterator.Valid(); iterator.Next() {
			challengeStore.Delete(iterator.Key())
		}

		store.Set(types.AttestedChallengesSizeKey, k.encodeUint64(toKeep))
		store.Delete(types.AttestedChallengesCursorKey)

		for _, c := range currentChallenges {
			k.enqueueAttestedChallenge(store, challengeStore, c)
		}
	}
	k.enqueueAttestedChallenge(store, challengeStore, challenge)
}

func (k Keeper) enqueueAttestedChallenge(store, challengeStore storetypes.KVStore, challenge *types.AttestedChallenge) {
	size := binary.BigEndian.Uint64(store.Get(types.AttestedChallengesSizeKey))
	cursorBz := store.Get(types.AttestedChallengesCursorKey)
	cursor := uint64(0)
	if cursorBz != nil {
		cursor = binary.BigEndian.Uint64(cursorBz)
		cursor = (cursor + 1) % size
	}

	cursorBz = k.encodeUint64(cursor)
	store.Set(types.AttestedChallengesCursorKey, cursorBz)

	challengeStore.Set(cursorBz, k.cdc.MustMarshal(challenge))
}

// GetChallengeCountCurrentBlock gets the count of challenges
func (k Keeper) GetChallengeCountCurrentBlock(ctx sdk.Context) uint64 {
	store := ctx.TransientStore(k.tKey)
	bz := store.Get(types.CurrentBlockChallengeCountKey)
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

// setGetChallengeCountCurrentBlock sets the new count of challenge to the store
func (k Keeper) setGetChallengeCountCurrentBlock(ctx sdk.Context, challengeID uint64) {
	store := ctx.TransientStore(k.tKey)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, challengeID)
	store.Set(types.CurrentBlockChallengeCountKey, bz)
}

// IncrChallengeCountCurrentBlock increases the count of challenge by one
func (k Keeper) IncrChallengeCountCurrentBlock(ctx sdk.Context) {
	k.setGetChallengeCountCurrentBlock(ctx, k.GetChallengeCountCurrentBlock(ctx)+1)
}

// depositLockUntil returns the furthest expiry among the challenges still open against an sp,
// or zero when none remain.
func (k Keeper) depositLockUntil(ctx sdk.Context, spID uint32) uint64 {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeKeyPrefix)
	spStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.ChallengeSpKeyPrefix)
	iterator := storetypes.KVStorePrefixIterator(spStore, []byte{})
	defer iterator.Close()

	var until uint64
	for ; iterator.Valid(); iterator.Next() {
		if binary.BigEndian.Uint32(iterator.Value()) != spID {
			continue
		}
		bz := store.Get(iterator.Key())
		if bz == nil {
			continue
		}
		if height := binary.BigEndian.Uint64(bz); height > until {
			until = height
		}
	}
	return until
}

// releaseDepositLock re-derives an sp's deposit lock from what is still open against it. A
// challenge that has been attested or has expired no longer holds the deposit, so an sp is not
// kept waiting on a challenge that is already settled.
func (k Keeper) releaseDepositLock(ctx sdk.Context, spID uint32) {
	k.SpKeeper.ReleaseDepositLockUntil(ctx, spID, k.depositLockUntil(ctx, spID))
}
