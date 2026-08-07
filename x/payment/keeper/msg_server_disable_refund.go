package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/utils"
	"github.com/mocachain/moca/v2/x/payment/types"
)

func (k msgServer) DisableRefund(goCtx context.Context, msg *types.MsgDisableRefund) (*types.MsgDisableRefundResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	addr := sdk.MustAccAddressFromHex(msg.Addr)
	paymentAccount, found := k.Keeper.GetPaymentAccount(ctx, addr)
	if !found {
		return nil, types.ErrPaymentAccountNotFound
	}
	// Compare this account's recorded owner, rather than asking whether the sender
	// owns it: IsPaymentAccountOwner also treats an account as its own owner, which
	// is right for its other callers but wider than this check should be.
	if !utils.SameAddress(paymentAccount.Owner, msg.Owner) {
		return nil, types.ErrNotPaymentAccountOwner
	}
	if !paymentAccount.Refundable {
		return nil, types.ErrPaymentAccountAlreadyNonRefundable
	}
	paymentAccount.Refundable = false
	k.Keeper.SetPaymentAccount(ctx, paymentAccount)
	return &types.MsgDisableRefundResponse{}, nil
}
