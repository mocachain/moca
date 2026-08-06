package keeper_test

import (
	"strings"
	"testing"

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
