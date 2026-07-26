package ante

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
)

// NewAnteHandler returns an ante handler responsible for attempting to route
// an Ethereum or SDK transaction to its respective ante handler chain. The
// route is selected by the transaction's first extension option:
//
//   - /cosmos.evm.vm.v1.ExtensionOptionsEthereumTx → EVM-tx mono pipeline
//     (cosmos/evm v0.6.0's NewEVMMonoDecorator).
//   - /cosmos.evm.ante.v1.ExtensionOptionDynamicFeeTx → cosmos-tx pipeline
//     with the dynamic-fee check enabled.
//   - default (no extension or unknown) → moca's cosmos-tx pipeline.
//
// The legacy ethermint Web3Tx EIP712 handler that the pre-migration ante
// supported is removed: cosmos/evm v0.6.0 no longer ships a Web3Tx
// extension and the corresponding type URL is no longer emitted by any
// supported signing flow.
func NewAnteHandler(options HandlerOptions) sdk.AnteHandler {
	return func(
		ctx sdk.Context, tx sdk.Tx, sim bool,
	) (newCtx sdk.Context, err error) {
		var anteHandler sdk.AnteHandler

		txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
		if ok {
			opts := txWithExtensions.GetExtensionOptions()
			if len(opts) > 0 {
				switch typeURL := opts[0].GetTypeUrl(); typeURL {
				case "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx":
					anteHandler = newEVMAnteHandler(ctx, options)
				case "/cosmos.evm.ante.v1.ExtensionOptionDynamicFeeTx":
					anteHandler = newCosmosAnteHandler(ctx, options)
				default:
					return ctx, errorsmod.Wrapf(
						errortypes.ErrUnknownExtensionOptions,
						"rejecting tx with unsupported extension option: %s", typeURL,
					)
				}

				return anteHandler(ctx, tx, sim)
			}
		}

		// Plain cosmos-tx with no extension option.
		switch tx.(type) {
		case sdk.Tx:
			anteHandler = newCosmosAnteHandler(ctx, options)
		default:
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid transaction type: %T", tx)
		}

		return anteHandler(ctx, tx, sim)
	}
}
