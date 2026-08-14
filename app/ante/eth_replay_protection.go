package ante

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// EthReplayProtectionDecorator rejects Ethereum transactions that carry no chain
// id in their signature.
//
// A signature that binds the chain id is what stops the same bytes from being
// valid on another chain. Signature verification cannot stand in for this check:
// the chain-id comparison in the signer only runs for a transaction that has one,
// and a transaction without one takes the pre-EIP-155 recovery path and yields a
// valid sender against any chain. So the two are rejected for different reasons —
// a mismatched chain id by the signer, an absent one only here.
//
// This has to run before signature verification: it reads the transaction's own
// signature values and does not depend on the sender being resolved.
type EthReplayProtectionDecorator struct{}

func NewEthReplayProtectionDecorator() EthReplayProtectionDecorator {
	return EthReplayProtectionDecorator{}
}

func (d EthReplayProtectionDecorator) AnteHandle(
	ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler,
) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		msgEthTx, ok := msg.(*evmtypes.MsgEthereumTx)
		if !ok {
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest,
				"invalid message type %T, expected %T", msg, (*evmtypes.MsgEthereumTx)(nil))
		}

		if !msgEthTx.AsTransaction().Protected() {
			return ctx, errorsmod.Wrap(errortypes.ErrNotSupported,
				"rejected unprotected Ethereum transaction: sign it under EIP-155 so it is bound to this chain")
		}
	}

	return next(ctx, tx, simulate)
}
