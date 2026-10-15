package keeper_test

import (
	"strings"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func (s *TestSuite) TestDisableRefund() {
	// payment account does not exist
	creator1 := sample.RandAccAddress()
	msg := types.NewMsgDisableRefund(creator1.String(), sample.RandAccAddress().String())
	_, err := s.msgServer.DisableRefund(s.ctx, msg)
	s.Require().Error(err)

	// the message is not from the owner
	creator2 := sample.RandAccAddress()
	createAccountMsg := types.NewMsgCreatePaymentAccount(creator2.String())
	_, err = s.msgServer.CreatePaymentAccount(s.ctx, createAccountMsg)
	s.Require().NoError(err)
	paymentAccountAddr := s.paymentKeeper.DerivePaymentAccountAddress(creator2, 0)
	record, _ := s.paymentKeeper.GetPaymentAccount(s.ctx, paymentAccountAddr)
	s.Require().True(record.Owner == creator2.String())

	msg = types.NewMsgDisableRefund(creator1.String(), paymentAccountAddr.String())
	_, err = s.msgServer.DisableRefund(s.ctx, msg)
	s.Require().Error(err)

	// disable refund success
	msg = types.NewMsgDisableRefund(creator2.String(), paymentAccountAddr.String())
	_, err = s.msgServer.DisableRefund(s.ctx, msg)
	s.Require().NoError(err)
	record, _ = s.paymentKeeper.GetPaymentAccount(s.ctx, paymentAccountAddr)
	s.Require().True(record.Refundable == false)

	// cannot disable it again
	msg = types.NewMsgDisableRefund(creator2.String(), paymentAccountAddr.String())
	_, err = s.msgServer.DisableRefund(s.ctx, msg)
	s.Require().Error(err)
}

// Hex casing carries no meaning, so an owner who submits a lowercase address must
// still be recognized as the owner. Clients routinely lowercase addresses, and the
// account is created and later operated on in whatever casing each message used.
func (s *TestSuite) TestDisableRefund_OwnerAddressCasingIsIgnored() {
	owner := sample.RandAccAddress()
	canonical := owner.String()
	lowered := strings.ToLower(canonical)
	s.Require().NotEqual(canonical, lowered, "the sample address must contain hex letters")

	// Create the account with the lowercase form, as a client may well send it.
	_, err := s.msgServer.CreatePaymentAccount(s.ctx, types.NewMsgCreatePaymentAccount(lowered))
	s.Require().NoError(err)

	paymentAccountAddr := s.paymentKeeper.DerivePaymentAccountAddress(owner, 0)

	// The owner is the owner whichever casing was used to create the account.
	s.Require().True(s.paymentKeeper.IsPaymentAccountOwner(s.ctx, paymentAccountAddr, owner))

	// And can act on it using the canonical form.
	_, err = s.msgServer.DisableRefund(s.ctx,
		types.NewMsgDisableRefund(canonical, paymentAccountAddr.String()))
	s.Require().NoError(err)

	account, found := s.paymentKeeper.GetPaymentAccount(s.ctx, paymentAccountAddr)
	s.Require().True(found)
	s.Require().False(account.Refundable)
}

// An account stored before addresses were written in a canonical casing must still
// resolve to its owner. This is what makes rewriting the stored values unnecessary.
func (s *TestSuite) TestIsPaymentAccountOwner_StoredOwnerCasingIsIgnored() {
	owner := sample.RandAccAddress()
	lowered := strings.ToLower(owner.String())
	s.Require().NotEqual(owner.String(), lowered, "the sample address must contain hex letters")

	paymentAccountAddr := s.paymentKeeper.DerivePaymentAccountAddress(owner, 0)
	s.paymentKeeper.SetPaymentAccount(s.ctx, &types.PaymentAccount{
		Addr:       paymentAccountAddr.String(),
		Owner:      lowered, // as an older binary would have written it
		Refundable: true,
	})

	s.Require().True(s.paymentKeeper.IsPaymentAccountOwner(s.ctx, paymentAccountAddr, owner))

	_, err := s.msgServer.DisableRefund(s.ctx,
		types.NewMsgDisableRefund(owner.String(), paymentAccountAddr.String()))
	s.Require().NoError(err)
}

// Only the account's recorded owner may disable its refund. Asking whether the
// sender owns the account is a wider question than that: an account counts as its
// own owner, which is right where that helper is used to decide whether a payment
// account belongs to an operator, but would let the check pass on identity alone.
func (s *TestSuite) TestDisableRefund_OnlyTheRecordedOwnerMayDisable() {
	owner := sample.RandAccAddress()
	_, err := s.msgServer.CreatePaymentAccount(s.ctx, types.NewMsgCreatePaymentAccount(owner.String()))
	s.Require().NoError(err)

	paymentAccountAddr := s.paymentKeeper.DerivePaymentAccountAddress(owner, 0)

	// The account named as its own owner is not the recorded owner.
	_, err = s.msgServer.DisableRefund(s.ctx,
		types.NewMsgDisableRefund(paymentAccountAddr.String(), paymentAccountAddr.String()))
	s.Require().ErrorIs(err, types.ErrNotPaymentAccountOwner)

	account, found := s.paymentKeeper.GetPaymentAccount(s.ctx, paymentAccountAddr)
	s.Require().True(found)
	s.Require().True(account.Refundable, "the account must still be refundable")
}
