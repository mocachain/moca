package keeper_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/0xPolygon/polygon-edge/bls"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/challenge/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgrouptypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func (s *TestSuite) TestAttest_Invalid() {
	// prepare challenge
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{
		Id: 100,
	})

	validSubmitter := sample.RandAccAddress()

	blsKey, _ := bls.GenerateBlsKey()
	historicalInfo := stakingtypes.HistoricalInfo{
		Header: tmproto.Header{},
		Valset: []stakingtypes.Validator{{
			BlsKey:            blsKey.PublicKey().Marshal(),
			ChallengerAddress: validSubmitter.String(),
		}},
	}
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).
		Return(historicalInfo, nil).AnyTimes()

	existObjectName := "existobject"
	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(10),
		ObjectName:   existObjectName,
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(math.NewUint(10))).
		Return(existObject, true).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 10, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()

	tests := []struct {
		name string
		msg  types.MsgAttest
		err  error
	}{
		{
			name: "unknown challenge",
			msg: types.MsgAttest{
				ChallengeId:       1,
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
			},
			err: types.ErrInvalidChallengeID,
		},
		{
			name: "not valid submitter",
			msg: types.MsgAttest{
				ChallengeId:       100,
				Submitter:         sample.RandAccAddressHex(),
				SpOperatorAddress: sample.RandAccAddressHex(),
			},
			err: types.ErrNotChallenger,
		},
		{
			name: "votes are not enough",
			msg: types.MsgAttest{
				ChallengeId:       100,
				Submitter:         validSubmitter.String(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				ObjectId:          math.NewUint(10),
				VoteValidatorSet:  []uint64{},
				VoteAggSignature:  []byte{},
			},
			err: types.ErrNotEnoughVotes,
		},
		{
			name: "invalid signature",
			msg: types.MsgAttest{
				ChallengeId:       100,
				Submitter:         validSubmitter.String(),
				SpOperatorAddress: sample.RandAccAddressHex(),
				ObjectId:          math.NewUint(10),
				VoteValidatorSet:  []uint64{1},
				VoteAggSignature:  []byte{},
			},
			err: types.ErrInvalidVoteAggSignature,
		},
	}
	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			msg := tt.msg
			_, err := s.msgServer.Attest(s.ctx, &msg)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func (s *TestSuite) TestAttest_Heartbeat() {
	// prepare challenge
	challengeID := s.challengeKeeper.GetParams(s.ctx).HeartbeatInterval
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{
		Id: challengeID,
	})

	validSubmitter := sample.RandAccAddress()

	blsKey, _ := bls.GenerateBlsKey()
	historicalInfo := stakingtypes.HistoricalInfo{
		Header: tmproto.Header{},
		Valset: []stakingtypes.Validator{{
			BlsKey:            blsKey.PublicKey().Marshal(),
			ChallengerAddress: validSubmitter.String(),
		}},
	}
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).
		Return(historicalInfo, nil).AnyTimes()

	existBucket := &storagetypes.BucketInfo{
		Id:                         math.NewUint(10),
		GlobalVirtualGroupFamilyId: 10,
		BucketName:                 "existbucket",
	}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(existBucket.BucketName)).
		Return(existBucket, true).AnyTimes()

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(10),
		ObjectName:   "existobject",
		BucketName:   existBucket.BucketName,
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(math.NewUint(10))).
		Return(existObject, true).AnyTimes()

	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1000000), nil).AnyTimes()
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 10, OperatorAddress: spOperatorAcc.String()}

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()

	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{
		SecondarySpIds: []uint32{10},
	}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Eq(existBucket.Id), gomock.Any()).
		Return(gvg, true).AnyTimes()

	attestMsg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challengeID,
		ObjectId:          math.NewUint(10),
		SpOperatorAddress: sp.OperatorAddress,
		VoteResult:        types.CHALLENGE_FAILED,
		ChallengerAddress: "",
		VoteValidatorSet:  []uint64{1},
	}
	toSign := attestMsg.GetVotePoolSignBytes(s.ctx.ChainID())

	voteAggSignature, _ := blsKey.Sign(toSign, votepool.DST)
	attestMsg.VoteAggSignature, _ = voteAggSignature.Marshal()

	_, err := s.msgServer.Attest(s.ctx, attestMsg)
	require.NoError(s.T(), err)

	attestedChallenges := s.challengeKeeper.GetAttestedChallenges(s.ctx)
	found := false
	for _, c := range attestedChallenges {
		if c.Id == challengeID {
			found = true
		}
	}
	s.Require().True(found)
}

// TestAttest_DuplicateRejected reproduces the multi-relayer scenario: the same
// valid heartbeat attestation is submitted twice (e.g. by redundant relayers or
// a resubmission by the in-turn submitter). The first attestation must succeed
// and retire the challenge; the second must be rejected by the ExistsChallenge
// gate instead of re-running doHeartbeatAndRewards and re-emitting events.
func (s *TestSuite) TestAttest_DuplicateRejected() {
	challengeID := s.challengeKeeper.GetParams(s.ctx).HeartbeatInterval
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{
		Id: challengeID,
	})

	validSubmitter := sample.RandAccAddress()

	blsKey, _ := bls.GenerateBlsKey()
	historicalInfo := stakingtypes.HistoricalInfo{
		Header: tmproto.Header{},
		Valset: []stakingtypes.Validator{{
			BlsKey:            blsKey.PublicKey().Marshal(),
			ChallengerAddress: validSubmitter.String(),
		}},
	}
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).
		Return(historicalInfo, nil).AnyTimes()

	existBucket := &storagetypes.BucketInfo{
		Id:                         math.NewUint(10),
		GlobalVirtualGroupFamilyId: 10,
		BucketName:                 "existbucket",
	}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(existBucket.BucketName)).
		Return(existBucket, true).AnyTimes()

	existObject := &storagetypes.ObjectInfo{
		Id:           math.NewUint(10),
		ObjectName:   "existobject",
		BucketName:   existBucket.BucketName,
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(math.NewUint(10))).
		Return(existObject, true).AnyTimes()

	// QueryDynamicBalance is the first line of doHeartbeatAndRewards, so asserting
	// it runs exactly once proves the duplicate attestation never reaches the
	// payout path (a single heartbeat itself issues two Withdraws: validator + submitter).
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1000000), nil).Times(1)
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 10, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()

	gvg := &virtualgrouptypes.GlobalVirtualGroup{
		SecondarySpIds: []uint32{10},
	}
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Eq(existBucket.Id), gomock.Any()).
		Return(gvg, true).AnyTimes()

	attestMsg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challengeID,
		ObjectId:          math.NewUint(10),
		SpOperatorAddress: sp.OperatorAddress,
		VoteResult:        types.CHALLENGE_FAILED,
		ChallengerAddress: "",
		VoteValidatorSet:  []uint64{1},
	}
	toSign := attestMsg.GetVotePoolSignBytes(s.ctx.ChainID())
	voteAggSignature, _ := blsKey.Sign(toSign, votepool.DST)
	attestMsg.VoteAggSignature, _ = voteAggSignature.Marshal()

	// First (legitimate) attestation succeeds and retires the challenge.
	s.Require().True(s.challengeKeeper.ExistsChallenge(s.ctx, challengeID))
	_, err := s.msgServer.Attest(s.ctx, attestMsg)
	s.Require().NoError(err)
	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, challengeID))

	// Second (duplicate) attestation with the very same signature is rejected.
	dupMsg := *attestMsg
	_, err = s.msgServer.Attest(s.ctx, &dupMsg)
	s.Require().ErrorIs(err, types.ErrInvalidChallengeID)
}

func (s *TestSuite) TestAttest_Normal() {
	// prepare challenge
	challenge1Id := uint64(99)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{
		Id: challenge1Id,
	})
	challenge2Id := uint64(100)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{
		Id: challenge2Id,
	})

	validSubmitter := sample.RandAccAddress()

	blsKey, _ := bls.GenerateBlsKey()
	historicalInfo := stakingtypes.HistoricalInfo{
		Header: tmproto.Header{},
		Valset: []stakingtypes.Validator{{
			BlsKey:            blsKey.PublicKey().Marshal(),
			ChallengerAddress: validSubmitter.String(),
		}},
	}
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).
		Return(historicalInfo, nil).AnyTimes()

	existBucket := &storagetypes.BucketInfo{
		Id:         math.NewUint(10),
		BucketName: "existbucket",
	}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(existBucket.BucketName)).
		Return(existBucket, true).AnyTimes()

	existObject1 := &storagetypes.ObjectInfo{
		Id:           math.NewUint(10),
		ObjectName:   "existobject1",
		BucketName:   existBucket.BucketName,
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(math.NewUint(10))).
		Return(existObject1, true).AnyTimes()

	existObject2 := &storagetypes.ObjectInfo{
		Id:           math.NewUint(100),
		ObjectName:   "existobject2",
		BucketName:   existBucket.BucketName,
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(math.NewUint(100))).
		Return(existObject2, true).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).
		Return("amoca").AnyTimes()
	s.spKeeper.EXPECT().Slash(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()

	// success attestation
	attestMsg1 := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challenge1Id,
		ObjectId:          math.NewUint(10),
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_SUCCEED,
		ChallengerAddress: "",
		VoteValidatorSet:  []uint64{1},
	}
	toSign1 := attestMsg1.GetVotePoolSignBytes(s.ctx.ChainID())
	voteAggSignature1, _ := blsKey.Sign(toSign1, votepool.DST)
	attestMsg1.VoteAggSignature, _ = voteAggSignature1.Marshal()
	_, err := s.msgServer.Attest(s.ctx, attestMsg1)
	require.NoError(s.T(), err)

	attestedChallenges := s.challengeKeeper.GetAttestedChallenges(s.ctx)
	attest1Found := false
	for _, c := range attestedChallenges {
		if c.Id == challenge1Id {
			attest1Found = true
		}
	}
	s.Require().True(attest1Found)
	s.Require().True(s.challengeKeeper.ExistsSlash(s.ctx, sp.Id, attestMsg1.ObjectId))

	// success attestation even exceed the max slash amount
	params := s.challengeKeeper.GetParams(s.ctx)
	params.SpSlashMaxAmount = math.NewInt(1)
	_ = s.challengeKeeper.SetParams(s.ctx, params)

	attestMsg2 := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challenge2Id,
		ObjectId:          math.NewUint(100),
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_SUCCEED,
		ChallengerAddress: sample.RandAccAddress().String(),
		VoteValidatorSet:  []uint64{1},
	}
	toSign2 := attestMsg2.GetVotePoolSignBytes(s.ctx.ChainID())
	voteAggSignature2, _ := blsKey.Sign(toSign2, votepool.DST)
	attestMsg2.VoteAggSignature, _ = voteAggSignature2.Marshal()
	_, err = s.msgServer.Attest(s.ctx, attestMsg2)
	require.NoError(s.T(), err)

	attestedChallenges = s.challengeKeeper.GetAttestedChallenges(s.ctx)
	attest2Found := false
	for _, c := range attestedChallenges {
		if c.Id == challenge1Id {
			attest2Found = true
		}
	}
	s.Require().True(attest1Found)
	s.Require().True(s.challengeKeeper.ExistsSlash(s.ctx, sp.Id, attestMsg1.ObjectId))
	s.Require().True(attest2Found)
	s.Require().True(s.challengeKeeper.ExistsSlash(s.ctx, sp.Id, attestMsg2.ObjectId))

	// the sp and the object had been slashed
	attestMsg3 := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challenge2Id,
		ObjectId:          math.NewUint(100),
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_SUCCEED,
		ChallengerAddress: sample.RandAccAddress().String(),
		VoteValidatorSet:  []uint64{1},
	}
	toSign3 := attestMsg3.GetVotePoolSignBytes(s.ctx.ChainID())
	voteAggSignature3, _ := blsKey.Sign(toSign3, votepool.DST)
	attestMsg3.VoteAggSignature, _ = voteAggSignature3.Marshal()
	_, err = s.msgServer.Attest(s.ctx, attestMsg3)
	require.Error(s.T(), err)
}

// lastSlashAmount returns the slash amount from the most recent attestation event.
func lastSlashAmount(t *testing.T, ctx sdk.Context) math.Int {
	var out math.Int
	found := false
	for _, ev := range ctx.EventManager().Events() {
		if ev.Type != proto.MessageName(&types.EventAttestChallenge{}) {
			continue
		}
		for _, a := range ev.Attributes {
			if a.Key != "slash_amount" {
				continue
			}
			v, ok := math.NewIntFromString(strings.Trim(a.Value, `"`))
			require.True(t, ok, "unparsable slash_amount %q", a.Value)
			out, found = v, true
		}
	}
	require.True(t, found, "no attestation event was emitted")
	return out
}

// Once an SP is near SpSlashMaxAmount, a further slash has to be reduced to what
// is left under the cap. It was set to zero instead, so an SP that reached the cap
// was slashed nothing at all until the counting window rolled over.
func (s *TestSuite) TestAttest_SlashAmountIsClampedNotZeroed() {
	challenge1Id, challenge2Id := uint64(99), uint64(100)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challenge1Id})
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challenge2Id})

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).
		Return(stakingtypes.HistoricalInfo{
			Header: tmproto.Header{},
			Valset: []stakingtypes.Validator{{
				BlsKey:            blsKey.PublicKey().Marshal(),
				ChallengerAddress: validSubmitter.String(),
			}},
		}, nil).AnyTimes()

	existBucket := &storagetypes.BucketInfo{Id: math.NewUint(10), BucketName: "clampbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(existBucket.BucketName)).
		Return(existBucket, true).AnyTimes()
	for _, id := range []uint64{10, 100} {
		s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(math.NewUint(id))).
			Return(&storagetypes.ObjectInfo{
				Id:           math.NewUint(id),
				ObjectName:   "clampobject",
				BucketName:   existBucket.BucketName,
				ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
				PayloadSize:  500,
			}, true).AnyTimes()
	}

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return("amoca").AnyTimes()
	s.spKeeper.EXPECT().Slash(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Eq(existBucket.Id), gomock.Any()).
		Return(&virtualgrouptypes.GlobalVirtualGroup{SecondarySpIds: []uint32{1}}, true).AnyTimes()

	attest := func(challengeID uint64, objectID uint64) {
		msg := &types.MsgAttest{
			Submitter:         validSubmitter.String(),
			ChallengeId:       challengeID,
			ObjectId:          math.NewUint(objectID),
			SpOperatorAddress: spOperatorAcc.String(),
			VoteResult:        types.CHALLENGE_SUCCEED,
			ChallengerAddress: "",
			VoteValidatorSet:  []uint64{1},
		}
		toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
		sig, _ := blsKey.Sign(toSign, votepool.DST)
		msg.VoteAggSignature, _ = sig.Marshal()
		_, err := s.msgServer.Attest(s.ctx, msg)
		require.NoError(s.T(), err)
	}

	// The first slash in a window is not capped, so it sets the running total.
	attest(challenge1Id, 10)
	firstSlash := lastSlashAmount(s.T(), s.ctx)
	require.True(s.T(), firstSlash.IsPositive())
	require.Equal(s.T(), firstSlash, s.challengeKeeper.GetSpSlashAmount(s.ctx, sp.Id))

	// Leave exactly one unit of room under the cap.
	params := s.challengeKeeper.GetParams(s.ctx)
	params.SpSlashMaxAmount = firstSlash.AddRaw(1)
	require.NoError(s.T(), s.challengeKeeper.SetParams(s.ctx, params))

	attest(challenge2Id, 100)
	require.Equal(s.T(), "1", lastSlashAmount(s.T(), s.ctx).String(),
		"the slash must be reduced to what is left under the cap, not dropped to zero")
}

// lastAttestEventAmount returns a named numeric attribute from the most recent attestation event.
func lastAttestEventAmount(t *testing.T, ctx sdk.Context, key string) math.Int {
	var out math.Int
	found := false
	for _, ev := range ctx.EventManager().Events() {
		if ev.Type != proto.MessageName(&types.EventAttestChallenge{}) {
			continue
		}
		for _, a := range ev.Attributes {
			if a.Key != key {
				continue
			}
			v, ok := math.NewIntFromString(strings.Trim(a.Value, `"`))
			require.True(t, ok, "unparsable %s %q", key, a.Value)
			out, found = v, true
		}
	}
	require.True(t, found, "no attestation event carried attribute %q", key)
	return out
}

// resolveChallengedSp must reject a challenge whose sp cannot be found at all, and must also
// reject one that IS bound to a real sp when the caller names a different operator address -
// exercising both the bound and not-found resolution paths, which no other test reaches.
func (s *TestSuite) TestAttest_ResolveChallengedSp() {
	validSubmitter := sample.RandAccAddress()

	// sp truly not found: no binding, and no sp registered under the operator address either.
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(nil, false).Times(1)
	_, err := s.msgServer.Attest(s.ctx, &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       1,
		SpOperatorAddress: sample.RandAccAddressHex(),
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	})
	s.Require().ErrorIs(err, types.ErrUnknownSp)

	// bound to a real sp, but the caller names a different operator: rejected even though the sp
	// exists, before ever reaching the validator/signature checks.
	boundSp := &sptypes.StorageProvider{Id: 55, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Eq(boundSp.Id)).Return(boundSp, true).Times(1)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 2})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 2, boundSp.Id)
	_, err = s.msgServer.Attest(s.ctx, &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       2,
		SpOperatorAddress: sample.RandAccAddressHex(), // not boundSp.OperatorAddress
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	})
	s.Require().ErrorIs(err, types.ErrUnknownSp)
}

func (s *TestSuite) TestAttest_HistoricalInfoError() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})
	spOperator := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperator.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).
		Return(stakingtypes.HistoricalInfo{}, errors.New("boom")).Times(1)

	_, err := s.msgServer.Attest(s.ctx, &types.MsgAttest{
		Submitter:         sample.RandAccAddressHex(),
		ChallengeId:       1,
		SpOperatorAddress: spOperator.String(),
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	})
	s.Require().ErrorIs(err, types.ErrInvalidVoteValidatorSet)
}

// isInturnAttestation calls GetHistoricalInfo a second time, internally, to resolve the in-turn
// submitter. This must surface that second call's own error, distinct from Attest's first,
// direct call succeeding - which needs the mock to succeed once and then fail.
func (s *TestSuite) TestAttest_IsInturnAttestationHistoricalInfoError() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	historicalInfo := stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	c1 := s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(historicalInfo, nil).Times(1)
	c2 := s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).
		Return(stakingtypes.HistoricalInfo{}, errors.New("boom")).Times(1)
	gomock.InOrder(c1, c2)

	_, err := s.msgServer.Attest(s.ctx, &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       1,
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	})
	s.Require().Error(err)
}

// TestAttest_NotInTurnSubmitter proves a registered validator who is not the in-turn submitter
// for the current interval is rejected, distinguishing "not a validator at all" (already covered
// by TestAttest_Invalid) from "valid validator, wrong turn".
func (s *TestSuite) TestAttest_NotInTurnSubmitter() {
	s.ctx = s.ctx.WithBlockTime(time.Unix(0, 0))
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})

	inTurnSubmitter := sample.RandAccAddress()
	otherSubmitter := sample.RandAccAddress()
	inTurnBls, _ := bls.GenerateBlsKey()
	otherBls, _ := bls.GenerateBlsKey()

	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{
			{BlsKey: inTurnBls.PublicKey().Marshal(), ChallengerAddress: inTurnSubmitter.String()},
			{BlsKey: otherBls.PublicKey().Marshal(), ChallengerAddress: otherSubmitter.String()},
		},
	}, nil).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	// block time 0 makes validator index 0 (inTurnSubmitter) the in-turn challenger; the
	// submitter here is a different, but still registered, validator.
	_, err := s.msgServer.Attest(s.ctx, &types.MsgAttest{
		Submitter:         otherSubmitter.String(),
		ChallengeId:       1,
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	})
	s.Require().ErrorIs(err, types.ErrNotInturnChallenger)
}

func (s *TestSuite) TestAttest_ObjectNotFound() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})
	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	msg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       1,
		ObjectId:          math.NewUint(1),
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	}
	toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
	sig, _ := blsKey.Sign(toSign, votepool.DST)
	msg.VoteAggSignature, _ = sig.Marshal()

	_, err := s.msgServer.Attest(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrUnknownBucketObject)
}

func (s *TestSuite) TestAttest_BucketNotFound() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})
	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	object := &storagetypes.ObjectInfo{
		Id:           math.NewUint(1),
		ObjectName:   "orphanobject",
		BucketName:   "missingbucket",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED,
		PayloadSize:  500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(object.BucketName)).Return(nil, false).AnyTimes()

	msg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       1,
		ObjectId:          object.Id,
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	}
	toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
	sig, _ := blsKey.Sign(toSign, votepool.DST)
	msg.VoteAggSignature, _ = sig.Marshal()

	_, err := s.msgServer.Attest(s.ctx, msg)
	s.Require().ErrorIs(err, storagetypes.ErrNoSuchBucket)
}

// TestAttest_ResolvesSpFromLiveGVG covers unbound challenges (created before the sp binding
// existed) whose challenged sp is no longer the bucket's primary: the sp must still be resolvable
// via the bucket's current GVG membership, and rejected once it no longer appears there at all.
func (s *TestSuite) TestAttest_ResolvesSpFromLiveGVG() {
	heartbeatInterval := s.challengeKeeper.GetParams(s.ctx).HeartbeatInterval

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	primarySp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	challengedSp := &sptypes.StorageProvider{Id: 2, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Eq(sdk.MustAccAddressFromHex(challengedSp.OperatorAddress))).
		Return(challengedSp, true).AnyTimes()

	bucket := &storagetypes.BucketInfo{Id: math.NewUint(50), BucketName: "gvgbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(primarySp).AnyTimes()
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).Return(math.NewInt(0), nil).AnyTimes()

	attest := func(challengeID, objectID uint64) error {
		s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challengeID})
		msg := &types.MsgAttest{
			Submitter:         validSubmitter.String(),
			ChallengeId:       challengeID,
			ObjectId:          math.NewUint(objectID),
			SpOperatorAddress: challengedSp.OperatorAddress,
			VoteResult:        types.CHALLENGE_FAILED,
			VoteValidatorSet:  []uint64{1},
		}
		toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
		sig, _ := blsKey.Sign(toSign, votepool.DST)
		msg.VoteAggSignature, _ = sig.Marshal()
		_, err := s.msgServer.Attest(s.ctx, msg)
		return err
	}

	// found-with-match: the challenged sp is a secondary on the bucket's GVG, so it is still
	// resolvable even though it is not the primary.
	matchedObject := &storagetypes.ObjectInfo{
		Id: math.NewUint(1), BucketName: bucket.BucketName, ObjectName: "matched",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500, LocalVirtualGroupId: 1,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(matchedObject.Id)).Return(matchedObject, true).AnyTimes()
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Eq(bucket.Id), gomock.Eq(matchedObject.LocalVirtualGroupId)).
		Return(&virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: primarySp.Id, SecondarySpIds: []uint32{challengedSp.Id}}, true).AnyTimes()
	require.NoError(s.T(), attest(heartbeatInterval, 1))

	// found-without-match: the GVG exists but the challenged sp is not one of its members anymore.
	unmatchedObject := &storagetypes.ObjectInfo{
		Id: math.NewUint(2), BucketName: bucket.BucketName, ObjectName: "unmatched",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500, LocalVirtualGroupId: 2,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(unmatchedObject.Id)).Return(unmatchedObject, true).AnyTimes()
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Eq(bucket.Id), gomock.Eq(unmatchedObject.LocalVirtualGroupId)).
		Return(&virtualgrouptypes.GlobalVirtualGroup{PrimarySpId: primarySp.Id, SecondarySpIds: []uint32{99}}, true).AnyTimes()
	require.ErrorIs(s.T(), attest(heartbeatInterval*2, 2), types.ErrNotStoredOnSp)

	// not-found: the object's GVG cannot be resolved at all.
	orphanObject := &storagetypes.ObjectInfo{
		Id: math.NewUint(3), BucketName: bucket.BucketName, ObjectName: "orphan",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500, LocalVirtualGroupId: 3,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(orphanObject.Id)).Return(orphanObject, true).AnyTimes()
	s.storageKeeper.EXPECT().GetObjectGVG(gomock.Any(), gomock.Eq(bucket.Id), gomock.Eq(orphanObject.LocalVirtualGroupId)).
		Return(nil, false).AnyTimes()
	require.ErrorIs(s.T(), attest(heartbeatInterval*3, 3), types.ErrNotStoredOnSp)
}

func (s *TestSuite) TestAttest_TooManyVoteValidators() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})
	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	_, err := s.msgServer.Attest(s.ctx, &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       1,
		SpOperatorAddress: sp.OperatorAddress,
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1, 1}, // two 64-bit words => bitset.Count() == 2, but there is only 1 validator
	})
	s.Require().ErrorIs(err, types.ErrInvalidVoteValidatorSet)
}

func (s *TestSuite) TestAttest_InvalidBlsPublicKey() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})
	validSubmitter := sample.RandAccAddress()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: []byte("not-a-bls-key"), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	_, err := s.msgServer.Attest(s.ctx, &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       1,
		SpOperatorAddress: sp.OperatorAddress,
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	})
	s.Require().ErrorIs(err, types.ErrInvalidBlsPubKey)
}

func (s *TestSuite) TestAttest_SignatureVerificationFails() {
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1})
	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	wrongBlsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()

	msg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       1,
		ObjectId:          math.NewUint(1),
		SpOperatorAddress: sp.OperatorAddress,
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	}
	toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
	sig, _ := wrongBlsKey.Sign(toSign, votepool.DST) // signed with a key different from the registered one
	msg.VoteAggSignature, _ = sig.Marshal()

	_, err := s.msgServer.Attest(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrInvalidVoteAggSignature)
}

// TestAttest_DuplicatedSlashRejected reproduces two open challenges raised against the same
// (sp, object) pair: attesting the first records the slash; attesting the second - a different,
// still-open challenge ID - must be rejected specifically for the existing slash record, not
// treated as an unknown or already-removed challenge.
func (s *TestSuite) TestAttest_DuplicatedSlashRejected() {
	challenge1, challenge2 := uint64(11), uint64(12)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challenge1})
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challenge2})

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	bucket := &storagetypes.BucketInfo{Id: math.NewUint(30), BucketName: "dupslashbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(sp).AnyTimes()
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return("amoca").AnyTimes()
	s.spKeeper.EXPECT().Slash(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	object := &storagetypes.ObjectInfo{
		Id: math.NewUint(40), BucketName: bucket.BucketName, ObjectName: "dupslashobject",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()

	attest := func(challengeID uint64) error {
		msg := &types.MsgAttest{
			Submitter:         validSubmitter.String(),
			ChallengeId:       challengeID,
			ObjectId:          object.Id,
			SpOperatorAddress: spOperatorAcc.String(),
			VoteResult:        types.CHALLENGE_SUCCEED,
			VoteValidatorSet:  []uint64{1},
		}
		toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
		sig, _ := blsKey.Sign(toSign, votepool.DST)
		msg.VoteAggSignature, _ = sig.Marshal()
		_, err := s.msgServer.Attest(s.ctx, msg)
		return err
	}

	require.NoError(s.T(), attest(challenge1))
	s.Require().True(s.challengeKeeper.ExistsSlash(s.ctx, sp.Id, object.Id))

	err := attest(challenge2)
	s.Require().ErrorIs(err, types.ErrDuplicatedSlash)
}

// TestAttest_SlashPropagatesSpKeeperError proves a payout failure from the sp module aborts the
// attestation instead of being swallowed, leaving no slash record behind.
func (s *TestSuite) TestAttest_SlashPropagatesSpKeeperError() {
	challengeID := uint64(21)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challengeID})

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	bucket := &storagetypes.BucketInfo{Id: math.NewUint(31), BucketName: "slasherrbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(sp).AnyTimes()
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return("amoca").AnyTimes()
	s.spKeeper.EXPECT().Slash(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("boom")).AnyTimes()

	object := &storagetypes.ObjectInfo{
		Id: math.NewUint(41), BucketName: bucket.BucketName, ObjectName: "slasherrobject",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()

	msg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challengeID,
		ObjectId:          object.Id,
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_SUCCEED,
		VoteValidatorSet:  []uint64{1},
	}
	toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
	sig, _ := blsKey.Sign(toSign, votepool.DST)
	msg.VoteAggSignature, _ = sig.Marshal()

	_, err := s.msgServer.Attest(s.ctx, msg)
	s.Require().Error(err)
	s.Require().False(s.challengeKeeper.ExistsSlash(s.ctx, sp.Id, object.Id),
		"the slash record must not be persisted when the payout fails")
}

// TestAttest_SlashRewards_ChallengerReceivesShare proves a slash triggered by an explicit
// challenger rewards that challenger with a positive share, instead of silently dropping it -
// the "challenger present" branch is only reached on a fresh sp's first slash in a window, since
// any running-total cap check that clamps the amount to zero skips it entirely.
func (s *TestSuite) TestAttest_SlashRewards_ChallengerReceivesShare() {
	challengeID := uint64(22)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challengeID})

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	bucket := &storagetypes.BucketInfo{Id: math.NewUint(32), BucketName: "challengerrewardbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()

	spOperatorAcc := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: spOperatorAcc.String()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(sp).AnyTimes()
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return("amoca").AnyTimes()
	s.spKeeper.EXPECT().Slash(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	object := &storagetypes.ObjectInfo{
		Id: math.NewUint(42), BucketName: bucket.BucketName, ObjectName: "challengerrewardobject",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()

	msg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challengeID,
		ObjectId:          object.Id,
		SpOperatorAddress: spOperatorAcc.String(),
		VoteResult:        types.CHALLENGE_SUCCEED,
		ChallengerAddress: sample.RandAccAddress().String(),
		VoteValidatorSet:  []uint64{1},
	}
	toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
	sig, _ := blsKey.Sign(toSign, votepool.DST)
	msg.VoteAggSignature, _ = sig.Marshal()

	_, err := s.msgServer.Attest(s.ctx, msg)
	require.NoError(s.T(), err)

	s.Require().True(lastAttestEventAmount(s.T(), s.ctx, "challenger_reward_amount").IsPositive(),
		"the challenger must receive a positive share of the slash")
	s.Require().True(lastAttestEventAmount(s.T(), s.ctx, "validator_reward_amount").IsPositive())
}

// TestAttest_SlashAmountSizeBasedBounds proves calculateSlashAmount's size-proportional amount is
// used as-is between the configured min and max, and clamped down when it would exceed the max -
// every other test's object is small enough to always clamp up to the minimum instead.
func (s *TestSuite) TestAttest_SlashAmountSizeBasedBounds() {
	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return("amoca").AnyTimes()
	s.spKeeper.EXPECT().Slash(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Each sp gets its own bucket (of which it is the primary): MustGetPrimarySPForBucket is
	// matched by bucket, so sharing one bucket across two sps would make the second call's mock
	// registration shadow the first's instead of resolving to the right sp.
	attest := func(challengeID uint64, sp *sptypes.StorageProvider, objectID, payloadSize uint64) {
		bucket := &storagetypes.BucketInfo{Id: math.NewUint(objectID), BucketName: fmt.Sprintf("bigobjectbucket%d", sp.Id)}
		s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challengeID})
		s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Eq(sdk.MustAccAddressFromHex(sp.OperatorAddress))).
			Return(sp, true).AnyTimes()
		s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()
		s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(sp).AnyTimes()
		object := &storagetypes.ObjectInfo{
			Id: math.NewUint(objectID), BucketName: bucket.BucketName, ObjectName: "bigobject",
			ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: payloadSize,
		}
		s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()

		msg := &types.MsgAttest{
			Submitter:         validSubmitter.String(),
			ChallengeId:       challengeID,
			ObjectId:          object.Id,
			SpOperatorAddress: sp.OperatorAddress,
			VoteResult:        types.CHALLENGE_SUCCEED,
			VoteValidatorSet:  []uint64{1},
		}
		toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
		sig, _ := blsKey.Sign(toSign, votepool.DST)
		msg.VoteAggSignature, _ = sig.Marshal()
		_, err := s.msgServer.Attest(s.ctx, msg)
		require.NoError(s.T(), err)
	}

	// 10 GiB: 0.0085 * 10 * 1e18 = 8.5e16, strictly between the default min (1e16) and max (1e18):
	// the size-proportional amount is used as-is, with neither bound kicking in.
	sp1 := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	attest(51, sp1, 51, 10*1024*1024*1024)
	require.Equal(s.T(), math.NewInt(85).MulRaw(1e15).String(), s.challengeKeeper.GetSpSlashAmount(s.ctx, sp1.Id).String())

	// 200 GiB: 0.0085 * 200 * 1e18 = 1.7e18, above the default max (1e18): clamped down to it.
	sp2 := &sptypes.StorageProvider{Id: 2, OperatorAddress: sample.RandAccAddressHex()}
	attest(52, sp2, 52, 200*1024*1024*1024)
	require.Equal(s.T(), s.challengeKeeper.GetParams(s.ctx).SlashAmountMax, s.challengeKeeper.GetSpSlashAmount(s.ctx, sp2.Id))
}

func (s *TestSuite) TestAttest_SlashRewards_SubmitterThresholdClamp() {
	params := s.challengeKeeper.GetParams(s.ctx)
	params.RewardSubmitterThreshold = math.NewInt(1)
	require.NoError(s.T(), s.challengeKeeper.SetParams(s.ctx, params))

	challengeID := uint64(23)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challengeID})

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	bucket := &storagetypes.BucketInfo{Id: math.NewUint(34), BucketName: "thresholdbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(sp).AnyTimes()
	s.spKeeper.EXPECT().DepositDenomForSP(gomock.Any()).Return("amoca").AnyTimes()
	s.spKeeper.EXPECT().Slash(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	object := &storagetypes.ObjectInfo{
		Id: math.NewUint(43), BucketName: bucket.BucketName, ObjectName: "thresholdobject",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()

	msg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challengeID,
		ObjectId:          object.Id,
		SpOperatorAddress: sp.OperatorAddress,
		VoteResult:        types.CHALLENGE_SUCCEED,
		VoteValidatorSet:  []uint64{1},
	}
	toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
	sig, _ := blsKey.Sign(toSign, votepool.DST)
	msg.VoteAggSignature, _ = sig.Marshal()

	_, err := s.msgServer.Attest(s.ctx, msg)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "1", lastAttestEventAmount(s.T(), s.ctx, "submitter_reward_amount").String(),
		"the submitter reward must be clamped down to the threshold")
}

func (s *TestSuite) TestAttest_HeartbeatIntervalMismatch() {
	heartbeatInterval := s.challengeKeeper.GetParams(s.ctx).HeartbeatInterval
	challengeID := heartbeatInterval + 1 // deliberately not a multiple of the heartbeat interval
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challengeID})

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	bucket := &storagetypes.BucketInfo{Id: math.NewUint(35), BucketName: "intervalmismatchbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(sp).AnyTimes()

	object := &storagetypes.ObjectInfo{
		Id: math.NewUint(44), BucketName: bucket.BucketName, ObjectName: "intervalmismatchobject",
		ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500,
	}
	s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()

	msg := &types.MsgAttest{
		Submitter:         validSubmitter.String(),
		ChallengeId:       challengeID,
		ObjectId:          object.Id,
		SpOperatorAddress: sp.OperatorAddress,
		VoteResult:        types.CHALLENGE_FAILED,
		VoteValidatorSet:  []uint64{1},
	}
	toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
	sig, _ := blsKey.Sign(toSign, votepool.DST)
	msg.VoteAggSignature, _ = sig.Marshal()

	_, err := s.msgServer.Attest(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrInvalidChallengeID)
}

// TestAttest_HeartbeatRewardPropagation exercises doHeartbeatAndRewards' three internal error
// returns and its submitter-reward threshold clamp. Each sub-case is a fresh heartbeat challenge,
// so the mocks registered for one case cannot leak into the next.
func (s *TestSuite) TestAttest_HeartbeatRewardPropagation() {
	heartbeatInterval := s.challengeKeeper.GetParams(s.ctx).HeartbeatInterval

	validSubmitter := sample.RandAccAddress()
	blsKey, _ := bls.GenerateBlsKey()
	s.stakingKeeper.EXPECT().GetHistoricalInfo(gomock.Any(), gomock.Any()).Return(stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{{BlsKey: blsKey.PublicKey().Marshal(), ChallengerAddress: validSubmitter.String()}},
	}, nil).AnyTimes()

	bucket := &storagetypes.BucketInfo{Id: math.NewUint(36), BucketName: "heartbeatrewardbucket"}
	s.storageKeeper.EXPECT().GetBucketInfo(gomock.Any(), gomock.Eq(bucket.BucketName)).Return(bucket, true).AnyTimes()
	sp := &sptypes.StorageProvider{Id: 1, OperatorAddress: sample.RandAccAddressHex()}
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.storageKeeper.EXPECT().MustGetPrimarySPForBucket(gomock.Any(), gomock.Eq(bucket)).Return(sp).AnyTimes()

	attest := func(challengeID, objectID uint64) error {
		s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: challengeID})
		object := &storagetypes.ObjectInfo{
			Id: math.NewUint(objectID), BucketName: bucket.BucketName, ObjectName: "heartbeatrewardobject",
			ObjectStatus: storagetypes.OBJECT_STATUS_SEALED, PayloadSize: 500,
		}
		s.storageKeeper.EXPECT().GetObjectInfoById(gomock.Any(), gomock.Eq(object.Id)).Return(object, true).AnyTimes()

		msg := &types.MsgAttest{
			Submitter:         validSubmitter.String(),
			ChallengeId:       challengeID,
			ObjectId:          object.Id,
			SpOperatorAddress: sp.OperatorAddress,
			VoteResult:        types.CHALLENGE_FAILED,
			VoteValidatorSet:  []uint64{1},
		}
		toSign := msg.GetVotePoolSignBytes(s.ctx.ChainID())
		sig, _ := blsKey.Sign(toSign, votepool.DST)
		msg.VoteAggSignature, _ = sig.Marshal()
		_, err := s.msgServer.Attest(s.ctx, msg)
		return err
	}

	// QueryDynamicBalance failing must abort before any reward is computed or withdrawn.
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.Int{}, errors.New("boom")).Times(1)
	require.Error(s.T(), attest(heartbeatInterval, 1))

	// The validator-reward withdraw failing must be surfaced, not swallowed.
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1_000_000_000_000_000_000), nil).Times(1)
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("boom")).Times(1)
	require.Error(s.T(), attest(heartbeatInterval*2, 2))

	// The submitter-reward withdraw failing (after the validator withdraw succeeds) must also be
	// surfaced. Both calls share the same argument shape, so the order they are registered in
	// must match the order the code makes them in.
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1_000_000_000_000_000_000), nil).Times(1)
	wc1 := s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	wc2 := s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("boom")).Times(1)
	gomock.InOrder(wc1, wc2)
	require.Error(s.T(), attest(heartbeatInterval*3, 3))

	// A submitter reward above the (now tiny) threshold is clamped, leaving the much larger
	// remainder for validators; both withdraws succeed and the heartbeat completes normally.
	params := s.challengeKeeper.GetParams(s.ctx)
	params.RewardSubmitterThreshold = math.NewInt(1)
	require.NoError(s.T(), s.challengeKeeper.SetParams(s.ctx, params))
	s.paymentKeeper.EXPECT().QueryDynamicBalance(gomock.Any(), gomock.Any()).
		Return(math.NewInt(1_000_000_000_000_000_000), nil).Times(1)
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Eq(math.NewInt(1_000_000_000_000_000_000).SubRaw(1))).Return(nil).Times(1)
	s.paymentKeeper.EXPECT().Withdraw(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(math.NewInt(1))).
		Return(nil).Times(1)
	require.NoError(s.T(), attest(heartbeatInterval*4, 4))
	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, heartbeatInterval*4))
}
