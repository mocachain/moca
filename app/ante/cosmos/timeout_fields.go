package cosmos

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// RejectUnsupportedTxFieldsDecorator rejects tx fields that moca does not
// currently support.
type RejectUnsupportedTxFieldsDecorator struct{}

// AnteHandle rejects a non-zero timeout_timestamp and, defensively, an
// unordered flag (already rejected later in the chain since moca never
// enables unordered transactions).
func (rtf RejectUnsupportedTxFieldsDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	if tsTx, ok := tx.(sdk.TxWithTimeoutTimeStamp); ok {
		ts := tsTx.GetTimeoutTimeStamp()
		if !ts.IsZero() && ts.Unix() != 0 {
			return ctx, errorsmod.Wrap(errortypes.ErrNotSupported, "timeout_timestamp is not supported")
		}
	}
	if uTx, ok := tx.(sdk.TxWithUnordered); ok && uTx.GetUnordered() {
		return ctx, errorsmod.Wrap(errortypes.ErrNotSupported, "unordered is not supported")
	}
	return next(ctx, tx, simulate)
}
