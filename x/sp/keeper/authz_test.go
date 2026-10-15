package keeper_test

import (
	"context"
	"errors"
	"time"

	"cosmossdk.io/math"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/types"
)

// rejectingAuthorization is a minimal authz.Authorization whose Accept always
// reports Accept=false with a nil error. No Authorization type in this repo (or
// in cosmos-sdk's own x/authz/x/bank/x/staking implementations) ever produces
// that combination on its own -- a rejection is always signaled by returning an
// error instead. It exists purely to reach CheckDepositAuthorization's
// `!resp.Accept` branch.
type rejectingAuthorization struct{}

func (rejectingAuthorization) Reset()                   {}
func (rejectingAuthorization) String() string           { return "rejectingAuthorization" }
func (rejectingAuthorization) ProtoMessage()            {}
func (rejectingAuthorization) Marshal() ([]byte, error) { return nil, nil }
func (rejectingAuthorization) ValidateBasic() error     { return nil }
func (rejectingAuthorization) MsgTypeURL() string {
	return sdk.MsgTypeURL(&types.MsgDeposit{})
}

func (rejectingAuthorization) Accept(context.Context, sdk.Msg) (authz.AcceptResponse, error) {
	return authz.AcceptResponse{Accept: false}, nil
}

// newDepositMsg builds a random grantee/granter pair and a matching MsgDeposit,
// mirroring how CreateStorageProvider constructs the authorization check.
func newDepositMsg() (grantee, granter sdk.AccAddress, msg *types.MsgDeposit) {
	grantee = sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
	granter = sdk.MustAccAddressFromHex(sample.RandAccAddressHex())
	msg = types.NewMsgDeposit(granter, sample.RandAccAddress(), sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(1)))
	return grantee, granter, msg
}

func (s *KeeperTestSuite) TestCheckDepositAuthorizationNotFound() {
	grantee, granter, msg := newDepositMsg()
	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(authz.Grant{}, false)

	err := s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, msg)
	require.ErrorIs(s.T(), err, authz.ErrNoAuthorizationFound)
}

func (s *KeeperTestSuite) TestCheckDepositAuthorizationExpired() {
	grantee, granter, msg := newDepositMsg()

	past := s.ctx.BlockTime().Add(-time.Hour)
	grant := authz.Grant{Expiration: &past}
	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)

	err := s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, msg)
	require.ErrorIs(s.T(), err, authz.ErrAuthorizationExpired)
}

// TestCheckDepositAuthorizationGetAuthorizationError builds a Grant by hand
// (rather than via authz.NewGrant/codectypes.NewAnyWithValue) so its
// Authorization Any carries no cached value; Grant.GetAuthorization() then
// cannot resolve it back to an Authorization and errors.
func (s *KeeperTestSuite) TestCheckDepositAuthorizationGetAuthorizationError() {
	grantee, granter, msg := newDepositMsg()

	grant := authz.Grant{Authorization: &cdctypes.Any{TypeUrl: "/moca.sp.DepositAuthorization"}}
	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)

	err := s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, msg)
	require.Error(s.T(), err)
}

// TestCheckDepositAuthorizationAcceptErrorWrongMsgType passes a message that
// isn't *MsgDeposit, tripping DepositAuthorization.Accept's own type guard.
func (s *KeeperTestSuite) TestCheckDepositAuthorizationAcceptErrorWrongMsgType() {
	grantee, granter, _ := newDepositMsg()
	spAddr := sample.RandAccAddress()

	auth := types.NewDepositAuthorization(spAddr, nil)
	grant, err := authz.NewGrant(s.ctx.BlockTime(), auth, nil)
	s.Require().NoError(err)
	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)

	err = s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, &types.MsgUpdateParams{})
	require.Error(s.T(), err)
}

// TestCheckDepositAuthorizationDeleteOnExactExhaustion drives a real
// DepositAuthorization to the exact-exhaustion case (Accept returns
// Delete=true), asserting the grant gets deleted rather than updated.
func (s *KeeperTestSuite) TestCheckDepositAuthorizationDeleteOnExactExhaustion() {
	grantee, granter, _ := newDepositMsg()
	spAddr := sample.RandAccAddress()
	limit := sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(10))

	auth := types.NewDepositAuthorization(spAddr, &limit)
	grant, err := authz.NewGrant(s.ctx.BlockTime(), auth, nil)
	s.Require().NoError(err)

	depositMsg := types.NewMsgDeposit(granter, spAddr, limit)

	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)
	s.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), grantee, granter, gomock.Any()).Return(nil)

	err = s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, depositMsg)
	require.NoError(s.T(), err)
}

// TestCheckDepositAuthorizationUpdateOnPartialUse drives a real
// DepositAuthorization to the partial-use case (Accept returns a remaining
// Updated authorization), asserting the grant gets updated rather than deleted.
func (s *KeeperTestSuite) TestCheckDepositAuthorizationUpdateOnPartialUse() {
	grantee, granter, _ := newDepositMsg()
	spAddr := sample.RandAccAddress()
	limit := sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(10))

	auth := types.NewDepositAuthorization(spAddr, &limit)
	grant, err := authz.NewGrant(s.ctx.BlockTime(), auth, nil)
	s.Require().NoError(err)

	depositMsg := types.NewMsgDeposit(granter, spAddr, sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(4)))

	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)
	s.authzKeeper.EXPECT().Update(gomock.Any(), grantee, granter, gomock.Any()).Return(nil)

	err = s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, depositMsg)
	require.NoError(s.T(), err)
}

// TestCheckDepositAuthorizationUpdateOnNilMaxDeposit covers the unlimited-grant
// case (MaxDeposit == nil), which Accept always answers with an update rather
// than a delete, regardless of the deposited amount.
func (s *KeeperTestSuite) TestCheckDepositAuthorizationUpdateOnNilMaxDeposit() {
	grantee, granter, _ := newDepositMsg()
	spAddr := sample.RandAccAddress()

	auth := types.NewDepositAuthorization(spAddr, nil)
	grant, err := authz.NewGrant(s.ctx.BlockTime(), auth, nil)
	s.Require().NoError(err)

	depositMsg := types.NewMsgDeposit(granter, spAddr, sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(4)))

	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)
	s.authzKeeper.EXPECT().Update(gomock.Any(), grantee, granter, gomock.Any()).Return(nil)

	err = s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, depositMsg)
	require.NoError(s.T(), err)
}

// TestCheckDepositAuthorizationDeleteGrantError asserts an error from the
// authz keeper's DeleteGrant call is propagated to the caller.
func (s *KeeperTestSuite) TestCheckDepositAuthorizationDeleteGrantError() {
	grantee, granter, _ := newDepositMsg()
	spAddr := sample.RandAccAddress()
	limit := sdk.NewCoin(types.DefaultDepositDenom, math.NewInt(10))

	auth := types.NewDepositAuthorization(spAddr, &limit)
	grant, err := authz.NewGrant(s.ctx.BlockTime(), auth, nil)
	s.Require().NoError(err)

	depositMsg := types.NewMsgDeposit(granter, spAddr, limit)

	wantErr := errors.New("delete failed")
	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)
	s.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), grantee, granter, gomock.Any()).Return(wantErr)

	err = s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, depositMsg)
	require.ErrorIs(s.T(), err, wantErr)
}

// TestCheckDepositAuthorizationNotAccepted uses the local rejectingAuthorization
// fake to reach the `!resp.Accept` branch, which no real Authorization in this
// codebase can produce without also returning an error.
func (s *KeeperTestSuite) TestCheckDepositAuthorizationNotAccepted() {
	grantee, granter, msg := newDepositMsg()

	packedAuth, err := cdctypes.NewAnyWithValue(&rejectingAuthorization{})
	s.Require().NoError(err)
	grant := authz.Grant{Authorization: packedAuth}

	s.authzKeeper.EXPECT().GetGrant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(grant, true)

	err = s.spKeeper.CheckDepositAuthorization(s.ctx, grantee, granter, msg)
	require.ErrorIs(s.T(), err, sdkerrors.ErrUnauthorized)
}
