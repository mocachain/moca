package keeper_test

import (
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func (s *TestSuite) TestCreatePaymentAccount() {
	creator := sample.RandAccAddress()

	// create first one
	msg := types.NewMsgCreatePaymentAccount(creator.String())
	_, err := s.msgServer.CreatePaymentAccount(s.ctx, msg)
	s.Require().NoError(err)

	record, _ := s.paymentKeeper.GetPaymentAccountCount(s.ctx, creator)
	s.Require().True(record.Count == 1)

	// create another one
	msg = types.NewMsgCreatePaymentAccount(creator.String())
	_, err = s.msgServer.CreatePaymentAccount(s.ctx, msg)
	s.Require().NoError(err)

	record, _ = s.paymentKeeper.GetPaymentAccountCount(s.ctx, creator)
	s.Require().True(record.Count == 2)

	// limit the number of payment account
	params := s.paymentKeeper.GetParams(s.ctx)
	params.PaymentAccountCountLimit = 2
	_ = s.paymentKeeper.SetParams(s.ctx, params)

	msg = types.NewMsgCreatePaymentAccount(creator.String())
	_, err = s.msgServer.CreatePaymentAccount(s.ctx, msg)
	s.Require().Error(err)
}

func (s *TestSuite) TestCreatePaymentAccount_RejectsGovernanceCreator() {
	msg := &types.MsgCreatePaymentAccount{Creator: types.GovernanceAddress.String()}
	_, err := s.msgServer.CreatePaymentAccount(s.ctx, msg)
	s.Require().ErrorIs(err, types.ErrGovernancePaymentAccount)

	count, _ := s.paymentKeeper.GetPaymentAccountCount(s.ctx, types.GovernanceAddress)
	s.Require().Equal(uint64(0), count.Count, "no payment account may be created for the governance account")
}
