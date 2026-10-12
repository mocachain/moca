package keeper_test

import (
	"strconv"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/x/challenge/keeper"
	"github.com/mocachain/moca/v2/x/challenge/types"
)

// Prevent strconv unused error
var _ = strconv.IntSize

func TestGetChallengeId(t *testing.T) {
	keeper, ctx := makeKeeper(t)
	keeper.SaveChallenge(ctx, types.Challenge{
		Id:            100,
		ExpiredHeight: 1000,
	})
	require.True(t, keeper.GetChallengeId(ctx) == 100)
}

func TestAttestedChallenges(t *testing.T) {
	keeper, ctx := makeKeeper(t)
	params := types.DefaultParams()
	params.AttestationKeptCount = 5
	err := keeper.SetParams(ctx, params)
	require.NoError(t, err)

	// Nothing has been appended yet, so the size marker itself is still unset.
	require.Empty(t, keeper.GetAttestedChallenges(ctx))

	c1 := &types.AttestedChallenge{Id: 1, Result: types.CHALLENGE_FAILED}
	c2 := &types.AttestedChallenge{Id: 2, Result: types.CHALLENGE_SUCCEED}
	c3 := &types.AttestedChallenge{Id: 3, Result: types.CHALLENGE_FAILED}

	keeper.AppendAttestedChallenge(ctx, c1)
	keeper.AppendAttestedChallenge(ctx, c2)
	keeper.AppendAttestedChallenge(ctx, c3)
	require.Equal(t, []*types.AttestedChallenge{c1, c2, c3}, keeper.GetAttestedChallenges(ctx))

	c4 := &types.AttestedChallenge{Id: 4, Result: types.CHALLENGE_FAILED}
	c5 := &types.AttestedChallenge{Id: 5, Result: types.CHALLENGE_FAILED}
	c6 := &types.AttestedChallenge{Id: 6, Result: types.CHALLENGE_SUCCEED}

	keeper.AppendAttestedChallenge(ctx, c4)
	keeper.AppendAttestedChallenge(ctx, c5)
	keeper.AppendAttestedChallenge(ctx, c6)
	require.Equal(t, []*types.AttestedChallenge{c2, c3, c4, c5, c6}, keeper.GetAttestedChallenges(ctx))

	params.AttestationKeptCount = 8
	err = keeper.SetParams(ctx, params)
	require.NoError(t, err)
	c7 := &types.AttestedChallenge{Id: 7, Result: types.CHALLENGE_FAILED}
	c8 := &types.AttestedChallenge{Id: 8, Result: types.CHALLENGE_SUCCEED}
	keeper.AppendAttestedChallenge(ctx, c7)
	keeper.AppendAttestedChallenge(ctx, c8)
	require.Equal(t, []*types.AttestedChallenge{c2, c3, c4, c5, c6, c7, c8}, keeper.GetAttestedChallenges(ctx))

	params.AttestationKeptCount = 3
	err = keeper.SetParams(ctx, params)
	require.NoError(t, err)
	c9 := &types.AttestedChallenge{Id: 9, Result: types.CHALLENGE_SUCCEED}
	keeper.AppendAttestedChallenge(ctx, c9)
	require.Equal(t, []*types.AttestedChallenge{c7, c8, c9}, keeper.GetAttestedChallenges(ctx))

	params.AttestationKeptCount = 5
	err = keeper.SetParams(ctx, params)
	require.NoError(t, err)
	c10 := &types.AttestedChallenge{Id: 10, Result: types.CHALLENGE_SUCCEED}
	keeper.AppendAttestedChallenge(ctx, c10)
	require.Equal(t, []*types.AttestedChallenge{c7, c8, c9, c10}, keeper.GetAttestedChallenges(ctx))
}

func makeKeeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	encCfg := moduletestutil.MakeTestEncodingConfig(mint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		key,
		&types.MockBankKeeper{},
		&types.MockStorageKeeper{},
		&types.MockSpKeeper{},
		&types.MockStakingKeeper{},
		&types.MockPaymentKeeper{},
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)

	return k, testCtx.Ctx
}

// A challenge that is attested or expires no longer holds the provider's deposit, so the lock
// is re-derived from what is still open rather than left at the height the challenge set.
func (s *TestSuite) TestDepositLockFollowsOpenChallenges() {
	const spID = uint32(7)

	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1, ExpiredHeight: 100})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 1, spID)
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 2, ExpiredHeight: 300})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 2, spID)

	// retiring the earlier one leaves the later one holding the deposit
	s.spKeeper.EXPECT().ReleaseDepositLockUntil(gomock.Any(), gomock.Eq(spID), gomock.Eq(uint64(300))).Times(1)
	s.challengeKeeper.RemoveChallenge(s.ctx, 1)

	// retiring the last one releases it
	s.spKeeper.EXPECT().ReleaseDepositLockUntil(gomock.Any(), gomock.Eq(spID), gomock.Eq(uint64(0))).Times(1)
	s.challengeKeeper.RemoveChallenge(s.ctx, 2)
}

// RemoveChallengeUntil must retire every challenge at or before the cutoff height, release each
// distinct affected sp's deposit lock exactly once (even when several removed challenges bound
// the same sp), skip challenges that predate the sp binding, and leave still-open challenges
// untouched.
func (s *TestSuite) TestRemoveChallengeUntil() {
	const (
		spA    = uint32(7)
		spB    = uint32(9)
		height = uint64(100)
	)

	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1, ExpiredHeight: 50})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 1, spA)

	// Same sp as challenge 1, expired exactly at the cutoff: the dedup guard must prevent a
	// second release call for spA.
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 2, ExpiredHeight: height})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 2, spA)

	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 3, ExpiredHeight: 60})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 3, spB)

	// Expired but never bound to an sp: must be removed without any sp lookup at all.
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 4, ExpiredHeight: 80})

	// Still open at the cutoff: must survive untouched.
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 5, ExpiredHeight: height + 50})

	s.spKeeper.EXPECT().ReleaseDepositLockUntil(gomock.Any(), gomock.Eq(spA), gomock.Any()).Times(1)
	s.spKeeper.EXPECT().ReleaseDepositLockUntil(gomock.Any(), gomock.Eq(spB), gomock.Any()).Times(1)

	s.challengeKeeper.RemoveChallengeUntil(s.ctx, height)

	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, 1))
	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, 2))
	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, 3))
	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, 4))
	s.Require().True(s.challengeKeeper.ExistsChallenge(s.ctx, 5))
}

// depositLockUntil must ignore sp-index entries for other sps and any sp-index entry left behind
// without a matching challenge record (an inconsistent state that should never arise in practice,
// but must not be treated as an open challenge either).
func (s *TestSuite) TestDepositLockUntil_SkipsUnrelatedEntries() {
	const spA, spB = uint32(7), uint32(8)

	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 1, ExpiredHeight: 100})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 1, spA)

	// Bound to a different sp: must be skipped while resolving spA's lock height.
	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 2, ExpiredHeight: 999})
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 2, spB)

	// An sp-index entry with no matching challenge record: must be skipped, not misread as an
	// open challenge at height 0.
	s.challengeKeeper.SaveChallengeSpID(s.ctx, 3, spA)

	// Removing the only real spA challenge, with nothing else genuinely open for spA, must
	// release its lock down to zero rather than being confused by the orphaned entry or spB's.
	s.spKeeper.EXPECT().ReleaseDepositLockUntil(gomock.Any(), gomock.Eq(spA), gomock.Eq(uint64(0))).Times(1)
	s.challengeKeeper.RemoveChallenge(s.ctx, 1)
}
