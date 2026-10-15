package keeper

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	"github.com/mocachain/moca/v2/x/storage/types"
)

func (k Keeper) VerifyPaymentAccount(_ sdk.Context, paymentAddress string, ownerAcc sdk.AccAddress) (sdk.AccAddress, error) {
	paymentAcc, err := sdk.AccAddressFromHexUnsafe(paymentAddress)
	if err == sdk.ErrEmptyHexAddress {
		return ownerAcc, nil
	} else if err != nil {
		return nil, err
	}
	// The payment governance account only receives; it is never a payer.
	if paymentAcc.Equals(paymenttypes.GovernanceAddress) {
		return nil, errorsmod.Wrap(types.ErrInvalidPaymentAddress, "the payment governance account cannot be a payment address")
	}

	return paymentAcc, nil
}
