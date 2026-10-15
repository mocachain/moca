package keeper_test

import (
	"errors"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment"
	"github.com/mocachain/moca/v2/x/payment/keeper"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func (s *TestSuite) TestDeposit_ToBankAccount() {
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()

	// deposit to self
	owner := sample.RandAccAddress()
	msg := types.NewMsgDeposit(owner.String(), owner.String(), sdkmath.NewInt(1000))
	_, err := s.msgServer.Deposit(s.ctx, msg)
	s.Require().NoError(err)
	record, _ := s.paymentKeeper.GetStreamRecord(s.ctx, owner)
	s.Require().True(record.StaticBalance.Int64() == msg.Amount.Int64())

	// deposit to other account
	to := sample.RandAccAddress()
	msg = types.NewMsgDeposit(owner.String(), to.String(), sdkmath.NewInt(1000))
	_, err = s.msgServer.Deposit(s.ctx, msg)
	s.Require().NoError(err)
	record, _ = s.paymentKeeper.GetStreamRecord(s.ctx, to)
	s.Require().True(record.StaticBalance.Int64() == msg.Amount.Int64())
}

func (s *TestSuite) TestDeposit_ToActiveStreamRecord() {
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()

	owner := sample.RandAccAddress()
	paymentAddr := sample.RandAccAddress()
	record := types.NewStreamRecord(paymentAddr, s.ctx.BlockTime().Unix())
	s.paymentKeeper.SetStreamRecord(s.ctx, record)

	// deposit to active stream record
	msg := types.NewMsgDeposit(owner.String(), paymentAddr.String(), sdkmath.NewInt(1000))
	_, err := s.msgServer.Deposit(s.ctx, msg)
	s.Require().NoError(err)
	recordAfter, _ := s.paymentKeeper.GetStreamRecord(s.ctx, paymentAddr)
	s.Require().True(recordAfter.StaticBalance.Int64() == msg.Amount.Int64())
}

func (s *TestSuite) TestDeposit_ToFrozenStreamRecord() {
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).
		Return(true).AnyTimes()

	owner := sample.RandAccAddress()
	paymentAddr := sample.RandAccAddress()
	record := types.NewStreamRecord(paymentAddr, s.ctx.BlockTime().Unix())
	record.Status = types.STREAM_ACCOUNT_STATUS_FROZEN
	record.FrozenNetflowRate = sdkmath.NewInt(-10)
	s.paymentKeeper.SetStreamRecord(s.ctx, record)

	// deposit to frozen stream record
	msg := types.NewMsgDeposit(owner.String(), paymentAddr.String(), sdkmath.NewInt(1000))
	_, err := s.msgServer.Deposit(s.ctx, msg)
	s.Require().NoError(err)
	recordAfter, _ := s.paymentKeeper.GetStreamRecord(s.ctx, paymentAddr)
	s.Require().True(recordAfter.StaticBalance.Int64() == msg.Amount.Int64())
}

// TestDeposit_InitialTransferError covers the very first bank transfer (creator
// funding the module) failing before any state is touched.
func (s *TestSuite) TestDeposit_InitialTransferError() {
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("insufficient funds")).Times(1)

	owner := sample.RandAccAddress()
	msg := types.NewMsgDeposit(owner.String(), sample.RandAccAddress().String(), sdkmath.NewInt(1000))
	_, err := s.msgServer.Deposit(s.ctx, msg)
	s.Require().Error(err)
}

// TestDeposit_ReceiveAccountNotExist covers depositing to an address that is
// neither a payment account nor a real bank account.
func (s *TestSuite) TestDeposit_ReceiveAccountNotExist() {
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	owner := sample.RandAccAddress()
	to := sample.RandAccAddress()
	msg := types.NewMsgDeposit(owner.String(), to.String(), sdkmath.NewInt(1000))
	_, err := s.msgServer.Deposit(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrReceiveAccountNotExist)
}

// TestDeposit_SweepsPositiveBankBalanceOfPaymentAccount covers depositing to a
// payment account that is separately holding a positive bank balance: that
// balance is swept into the deposit rather than left stranded outside the
// stream-record system.
func (s *TestSuite) TestDeposit_SweepsPositiveBankBalanceOfPaymentAccount() {
	creator := sample.RandAccAddress()
	createMsg := types.NewMsgCreatePaymentAccount(creator.String())
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	_, err := s.msgServer.CreatePaymentAccount(s.ctx, createMsg)
	s.Require().NoError(err)
	paymentAddr := s.paymentKeeper.DerivePaymentAccountAddress(creator, 0)

	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	strandedBalance := sdk.NewCoin(types.DefaultFeeDenom, sdkmath.NewInt(250))
	s.bankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(strandedBalance).AnyTimes()

	msg := types.NewMsgDeposit(creator.String(), paymentAddr.String(), sdkmath.NewInt(1000))
	_, err = s.msgServer.Deposit(s.ctx, msg)
	s.Require().NoError(err)

	record, _ := s.paymentKeeper.GetStreamRecord(s.ctx, paymentAddr)
	s.Require().Equal(sdkmath.NewInt(1250), record.StaticBalance)
}

// TestDeposit_SweepOfPaymentAccountBalanceFails covers the sweep transfer itself
// (moving the payment account's own stranded bank balance into the module)
// failing.
func (s *TestSuite) TestDeposit_SweepOfPaymentAccountBalanceFails() {
	creator := sample.RandAccAddress()
	createMsg := types.NewMsgCreatePaymentAccount(creator.String())
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), creator, gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	_, err := s.msgServer.CreatePaymentAccount(s.ctx, createMsg)
	s.Require().NoError(err)
	paymentAddr := s.paymentKeeper.DerivePaymentAccountAddress(creator, 0)

	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	strandedBalance := sdk.NewCoin(types.DefaultFeeDenom, sdkmath.NewInt(250))
	s.bankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(strandedBalance).AnyTimes()
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), paymentAddr, gomock.Any(), gomock.Any()).
		Return(errors.New("sweep failed")).Times(1)

	msg := types.NewMsgDeposit(creator.String(), paymentAddr.String(), sdkmath.NewInt(1000))
	_, err = s.msgServer.Deposit(s.ctx, msg)
	s.Require().Error(err)
}

// TestDeposit_ActiveStreamRecordUpdateError covers a deposit to an ACTIVE stream
// record that is already so far past its buffer window that even the deposit
// does not clear the non-forced "close to forced settlement" guard.
func (s *TestSuite) TestDeposit_ActiveStreamRecordUpdateError() {
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().HasAccount(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

	owner := sample.RandAccAddress()
	to := sample.RandAccAddress()
	record := &types.StreamRecord{
		Account:       to.String(),
		Status:        types.STREAM_ACCOUNT_STATUS_ACTIVE,
		StaticBalance: sdkmath.ZeroInt(),
		BufferBalance: sdkmath.ZeroInt(),
		LockBalance:   sdkmath.ZeroInt(),
		NetflowRate:   sdkmath.NewInt(-1000),
		OutFlowCount:  1,
		CrudTimestamp: s.ctx.BlockTime().Unix(),
	}
	s.paymentKeeper.SetStreamRecord(s.ctx, record)

	// a small deposit leaves payDuration well under ForcedSettleTime, and a
	// non-forced (live-tx) update must reject that rather than force-settle it
	msg := types.NewMsgDeposit(owner.String(), to.String(), sdkmath.NewInt(1000))
	_, err := s.msgServer.Deposit(s.ctx, msg)
	s.Require().Error(err)
}

// TestDeposit_FrozenStreamRecordResumeError covers TryResumeStreamRecord itself
// returning an error (the account is already queued for auto-resume).
func (s *TestSuite) TestDeposit_FrozenStreamRecordResumeError() {
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	owner := sample.RandAccAddress()
	to := sample.RandAccAddress()
	record := types.NewStreamRecord(to, s.ctx.BlockTime().Unix())
	record.Status = types.STREAM_ACCOUNT_STATUS_FROZEN
	record.FrozenNetflowRate = sdkmath.NewInt(-10)
	s.paymentKeeper.SetStreamRecord(s.ctx, record)
	s.paymentKeeper.SetAutoResumeRecord(s.ctx, &types.AutoResumeRecord{
		Timestamp: s.ctx.BlockTime().Unix(),
		Addr:      to.String(),
	})

	msg := types.NewMsgDeposit(owner.String(), to.String(), sdkmath.NewInt(1000))
	_, err := s.msgServer.Deposit(s.ctx, msg)
	s.Require().ErrorContains(err, "is resuming")
}

// TestDeposit_InvalidStreamAccountStatus covers the defensive default branch: a
// stored stream record whose status is neither ACTIVE nor FROZEN. Such a record
// can never be written through SetStreamRecord (CheckStreamRecord rejects it),
// so it is seeded directly into the store, bypassing the keeper, to prove
// Deposit itself still rejects it rather than assuming CheckStreamRecord already
// guarantees a valid status everywhere it is read.
func (s *TestSuite) TestDeposit_InvalidStreamAccountStatus() {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test_invalid_status"))
	ctx := testCtx.Ctx

	ctrl := gomock.NewController(s.T())
	bankKeeper := types.NewMockBankKeeper(ctrl)
	accountKeeper := types.NewMockAccountKeeper(ctrl)
	k := keeper.NewKeeper(encCfg.Codec, key, bankKeeper, accountKeeper, s.paymentKeeper.GetAuthority())
	s.Require().NoError(k.SetParams(ctx, types.DefaultParams()))
	msgServer := keeper.NewMsgServerImpl(*k)

	owner := sample.RandAccAddress()
	to := sample.RandAccAddress()
	malformed := &types.StreamRecord{
		Account:       "",
		Status:        99,
		StaticBalance: sdkmath.ZeroInt(),
		BufferBalance: sdkmath.ZeroInt(),
		LockBalance:   sdkmath.ZeroInt(),
		NetflowRate:   sdkmath.ZeroInt(),
	}
	store := prefix.NewStore(ctx.KVStore(key), types.StreamRecordKeyPrefix)
	store.Set(types.StreamRecordKey(to), encCfg.Codec.MustMarshal(malformed))

	bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	msg := types.NewMsgDeposit(owner.String(), to.String(), sdkmath.NewInt(1000))
	_, err := msgServer.Deposit(ctx, msg)
	s.Require().ErrorIs(err, types.ErrInvalidStreamAccountStatus)
}
