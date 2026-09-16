package keeper_test

import (
	"errors"

	sdkmath "cosmossdk.io/math"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func (s *TestSuite) TestWithdraw_Fail() {
	creator1 := sample.RandAccAddress()
	paymentAddr1 := sample.RandAccAddress()

	// stream record not found
	msg := types.NewMsgWithdraw(creator1.String(), sample.RandAccAddress().String(), sdkmath.NewInt(100))
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().Error(err)

	// stream record is frozen
	record1 := types.NewStreamRecord(paymentAddr1, s.ctx.BlockTime().Unix())
	record1.Status = types.STREAM_ACCOUNT_STATUS_FROZEN
	s.paymentKeeper.SetStreamRecord(s.ctx, record1)

	msg = types.NewMsgWithdraw(creator1.String(), paymentAddr1.String(), sdkmath.NewInt(100))
	_, err = s.msgServer.Withdraw(s.ctx, msg)
	s.Require().Error(err)

	record1.Status = types.STREAM_ACCOUNT_STATUS_ACTIVE
	s.paymentKeeper.SetStreamRecord(s.ctx, record1)

	// payment account does not exist
	msg = types.NewMsgWithdraw(creator1.String(), paymentAddr1.String(), sdkmath.NewInt(100))
	_, err = s.msgServer.Withdraw(s.ctx, msg)
	s.Require().Error(err)

	// the message is not from the owner
	creator2 := sample.RandAccAddress()
	createAccountMsg := types.NewMsgCreatePaymentAccount(creator2.String())
	_, err = s.msgServer.CreatePaymentAccount(s.ctx, createAccountMsg)
	s.Require().NoError(err)
	paymentAddr2 := s.paymentKeeper.DerivePaymentAccountAddress(creator2, 0)
	paymentAccountRecord, _ := s.paymentKeeper.GetPaymentAccount(s.ctx, paymentAddr2)
	s.Require().True(paymentAccountRecord.Owner == creator2.String())

	record2 := types.NewStreamRecord(paymentAddr2, s.ctx.BlockTime().Unix())
	s.paymentKeeper.SetStreamRecord(s.ctx, record2)

	msg = types.NewMsgWithdraw(creator1.String(), paymentAddr2.String(), sdkmath.NewInt(100))
	_, err = s.msgServer.Withdraw(s.ctx, msg)
	s.Require().Error(err)

	// cannot withdraw after disable refund
	disableRefundMsg := types.NewMsgDisableRefund(creator2.String(), paymentAddr2.String())
	_, err = s.msgServer.DisableRefund(s.ctx, disableRefundMsg)
	s.Require().NoError(err)
	paymentAccountRecord, _ = s.paymentKeeper.GetPaymentAccount(s.ctx, paymentAddr2)
	s.Require().True(paymentAccountRecord.Refundable == false)

	msg = types.NewMsgWithdraw(creator2.String(), paymentAddr2.String(), sdkmath.NewInt(100))
	_, err = s.msgServer.Withdraw(s.ctx, msg)
	s.Require().Error(err)
}

func (s *TestSuite) TestWithdraw_Success() {
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()

	creator := sample.RandAccAddress()
	createAccountMsg := types.NewMsgCreatePaymentAccount(creator.String())
	_, err := s.msgServer.CreatePaymentAccount(s.ctx, createAccountMsg)
	s.Require().NoError(err)
	paymentAddr := s.paymentKeeper.DerivePaymentAccountAddress(creator, 0)
	paymentAccountRecord, _ := s.paymentKeeper.GetPaymentAccount(s.ctx, paymentAddr)
	s.Require().True(paymentAccountRecord.Owner == creator.String())

	record := types.NewStreamRecord(paymentAddr, s.ctx.BlockTime().Unix())
	record.StaticBalance = sdkmath.NewInt(200)
	s.paymentKeeper.SetStreamRecord(s.ctx, record)

	msg := types.NewMsgWithdraw(creator.String(), paymentAddr.String(), sdkmath.NewInt(100))
	_, err = s.msgServer.Withdraw(s.ctx, msg)
	s.Require().NoError(err)
}

// TestWithdraw_DelayedRedemption_NotFound covers redeeming a locked withdrawal
// (msg.From == "") when the creator has no delayed withdrawal queued.
func (s *TestSuite) TestWithdraw_DelayedRedemption_NotFound() {
	creator := sample.RandAccAddress()
	msg := types.NewMsgWithdraw(creator.String(), "", sdkmath.NewInt(100))
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrNoDelayedWithdrawal)
}

// TestWithdraw_DelayedRedemption_AmountMismatch covers redeeming with an amount
// that does not equal the queued delayed withdrawal.
func (s *TestSuite) TestWithdraw_DelayedRedemption_AmountMismatch() {
	creator := sample.RandAccAddress()
	s.paymentKeeper.SetDelayedWithdrawalRecord(s.ctx, &types.DelayedWithdrawalRecord{
		Addr:            creator.String(),
		Amount:          sdkmath.NewInt(500),
		From:            sample.RandAccAddress().String(),
		UnlockTimestamp: s.ctx.BlockTime().Unix() - 1,
	})

	msg := types.NewMsgWithdraw(creator.String(), "", sdkmath.NewInt(100))
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrIncorrectWithdrawAmount)
}

// TestWithdraw_DelayedRedemption_TimeLockNotReached covers redeeming before the
// unlock timestamp has passed.
func (s *TestSuite) TestWithdraw_DelayedRedemption_TimeLockNotReached() {
	creator := sample.RandAccAddress()
	s.paymentKeeper.SetDelayedWithdrawalRecord(s.ctx, &types.DelayedWithdrawalRecord{
		Addr:            creator.String(),
		Amount:          sdkmath.NewInt(500),
		From:            sample.RandAccAddress().String(),
		UnlockTimestamp: s.ctx.BlockTime().Unix() + 1_000_000,
	})

	msg := types.NewMsgWithdraw(creator.String(), "", sdkmath.NewInt(500))
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrNotReachTimeLockDuration)
}

// TestWithdraw_DelayedRedemption_Success covers the happy path: the delayed
// withdrawal record is removed and the funds transferred directly from the
// module account.
func (s *TestSuite) TestWithdraw_DelayedRedemption_Success() {
	creator := sample.RandAccAddress()
	from := sample.RandAccAddress()
	s.paymentKeeper.SetDelayedWithdrawalRecord(s.ctx, &types.DelayedWithdrawalRecord{
		Addr:            creator.String(),
		Amount:          sdkmath.NewInt(500),
		From:            from.String(),
		UnlockTimestamp: s.ctx.BlockTime().Unix() - 1,
	})
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), creator, gomock.Any()).
		Return(nil).Times(1)

	msg := types.NewMsgWithdraw(creator.String(), "", sdkmath.NewInt(500))
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().NoError(err)

	_, found := s.paymentKeeper.GetDelayedWithdrawalRecord(s.ctx, creator)
	s.Require().False(found, "the redeemed delayed withdrawal must be removed")
}

// TestWithdraw_DelayedRedemption_BankTransferError covers bankTransfer's own
// SendCoinsFromModuleToAccount call failing, reached from the delayed-withdrawal
// redemption path.
func (s *TestSuite) TestWithdraw_DelayedRedemption_BankTransferError() {
	creator := sample.RandAccAddress()
	from := sample.RandAccAddress()
	s.paymentKeeper.SetDelayedWithdrawalRecord(s.ctx, &types.DelayedWithdrawalRecord{
		Addr:            creator.String(),
		Amount:          sdkmath.NewInt(500),
		From:            from.String(),
		UnlockTimestamp: s.ctx.BlockTime().Unix() - 1,
	})
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), creator, gomock.Any()).
		Return(errors.New("transfer failed")).Times(1)

	msg := types.NewMsgWithdraw(creator.String(), "", sdkmath.NewInt(500))
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().Error(err)

	// the delayed withdrawal was already removed before the failed transfer;
	// this documents the current (not-rolled-back) behavior rather than a bug
	// in the withdrawal flow itself.
	_, found := s.paymentKeeper.GetDelayedWithdrawalRecord(s.ctx, creator)
	s.Require().False(found)
}

// TestWithdraw_UpdateStreamRecordError covers the direct-withdrawal path's
// UpdateStreamRecord call itself returning an error (insufficient balance).
func (s *TestSuite) TestWithdraw_UpdateStreamRecordError() {
	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	from := sample.RandAccAddress()
	record := types.NewStreamRecord(from, s.ctx.BlockTime().Unix())
	record.StaticBalance = sdkmath.NewInt(100)
	s.paymentKeeper.SetStreamRecord(s.ctx, record)

	msg := types.NewMsgWithdraw(from.String(), from.String(), sdkmath.NewInt(200))
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().ErrorContains(err, "balance not enough")
}

// TestWithdraw_TimeLockThreshold covers a withdrawal at or above the configured
// threshold: it is queued as a delayed withdrawal instead of transferring funds
// immediately, and a second such withdrawal is rejected while one is pending.
func (s *TestSuite) TestWithdraw_TimeLockThreshold() {
	params := s.paymentKeeper.GetParams(s.ctx)
	threshold := *params.WithdrawTimeLockThreshold

	from := sample.RandAccAddress()
	record := types.NewStreamRecord(from, s.ctx.BlockTime().Unix())
	record.StaticBalance = threshold.MulRaw(5)
	s.paymentKeeper.SetStreamRecord(s.ctx, record)

	// No SendCoinsFromModuleToAccount expectation is registered: a threshold
	// withdrawal must not transfer funds immediately, so any such call here
	// would fail the test as unexpected.
	msg := types.NewMsgWithdraw(from.String(), from.String(), threshold)
	_, err := s.msgServer.Withdraw(s.ctx, msg)
	s.Require().NoError(err)

	delayed, found := s.paymentKeeper.GetDelayedWithdrawalRecord(s.ctx, from)
	s.Require().True(found)
	s.Require().True(threshold.Equal(delayed.Amount))
	s.Require().Equal(from.String(), delayed.From)
	s.Require().Equal(s.ctx.BlockTime().Unix()+int64(params.WithdrawTimeLockDuration), delayed.UnlockTimestamp) //nolint:gosec // G115

	// a second threshold withdrawal must not queue a new one while one is pending
	_, err = s.msgServer.Withdraw(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrExistsDelayedWithdrawal)
}

// TestWithdraw_DelayedRedemption_UnlockInstantBoundary pins the exact
// now<=end comparison that gates a delayed withdrawal's unlock: at now==end
// the withdrawal is still locked, and one second later, at now==end+1, it is
// unlocked.
func (s *TestSuite) TestWithdraw_DelayedRedemption_UnlockInstantBoundary() {
	now := s.ctx.BlockTime().Unix()

	// now == end: still locked
	lockedCreator := sample.RandAccAddress()
	s.paymentKeeper.SetDelayedWithdrawalRecord(s.ctx, &types.DelayedWithdrawalRecord{
		Addr:            lockedCreator.String(),
		Amount:          sdkmath.NewInt(500),
		From:            sample.RandAccAddress().String(),
		UnlockTimestamp: now,
	})
	lockedMsg := types.NewMsgWithdraw(lockedCreator.String(), "", sdkmath.NewInt(500))
	_, err := s.msgServer.Withdraw(s.ctx, lockedMsg)
	s.Require().ErrorIs(err, types.ErrNotReachTimeLockDuration, "now == end must still be locked")

	// now == end+1: unlocked
	unlockedCreator := sample.RandAccAddress()
	s.paymentKeeper.SetDelayedWithdrawalRecord(s.ctx, &types.DelayedWithdrawalRecord{
		Addr:            unlockedCreator.String(),
		Amount:          sdkmath.NewInt(500),
		From:            sample.RandAccAddress().String(),
		UnlockTimestamp: now - 1,
	})
	s.bankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), unlockedCreator, gomock.Any()).
		Return(nil).Times(1)
	unlockedMsg := types.NewMsgWithdraw(unlockedCreator.String(), "", sdkmath.NewInt(500))
	_, err = s.msgServer.Withdraw(s.ctx, unlockedMsg)
	s.Require().NoError(err, "now == end+1 must be unlocked")
}
