package ante

import (
	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"

	cosmosevmante "github.com/cosmos/evm/ante"
	cosmosevmevm "github.com/cosmos/evm/ante/evm"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	cosmosante "github.com/mocachain/moca/v2/app/ante/cosmos"
	anteutils "github.com/mocachain/moca/v2/app/ante/utils"
)

// HandlerOptions defines the list of module keepers required to run the moca
// AnteHandler decorators. The EVM-tx path is the cosmos/evm v0.6.0 Mono
// decorator; the Cosmos-tx path keeps moca's custom DeductFee + MinGasPrice
// decorators so moca-specific behavior (staking-rewards top-up on insufficient
// balance, moca's per-denom min-gas rules) is preserved.
type HandlerOptions struct {
	Cdc                    codec.BinaryCodec
	AccountKeeper          anteinterfaces.AccountKeeper
	BankKeeper             anteinterfaces.BankKeeper
	DistributionKeeper     anteutils.DistributionKeeper
	FeeMarketKeeper        anteinterfaces.FeeMarketKeeper
	EvmKeeper              anteinterfaces.EVMKeeper
	FeegrantKeeper         ante.FeegrantKeeper
	ExtensionOptionChecker ante.ExtensionOptionChecker
	SignModeHandler        *txsigning.HandlerMap
	SigGasConsumer         func(meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params) error
	MaxTxGasWanted         uint64
	// TxFeeChecker, when non-nil, replaces the default fee check inside moca's
	// custom DeductFeeDecorator on the cosmos-tx path. Leaving it nil falls
	// back to checkTxFeeWithValidatorMinGasPrices.
	TxFeeChecker anteutils.TxFeeChecker
	// PendingTxListener, when non-nil, appends cosmos/evm's tx-listener
	// decorator to the EVM-tx chain so JSON-RPC newPendingTransactions
	// subscriptions can fire from CheckTx. Optional.
	PendingTxListener cosmosevmante.PendingTxListener
}

// Validate checks if the keepers are defined.
func (options HandlerOptions) Validate() error {
	if options.Cdc == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "codec is required for AnteHandler")
	}
	if options.AccountKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "account keeper is required for AnteHandler")
	}
	if options.BankKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "bank keeper is required for AnteHandler")
	}
	if options.FeeMarketKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "fee market keeper is required for AnteHandler")
	}
	if options.EvmKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "evm keeper is required for AnteHandler")
	}
	if options.SigGasConsumer == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "signature gas consumer is required for AnteHandler")
	}
	if options.SignModeHandler == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "sign mode handler is required for AnteHandler")
	}
	if options.DistributionKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "distribution keeper is required for AnteHandler")
	}
	return nil
}

// newEVMAnteHandler builds the EVM-transaction AnteHandler using cosmos/evm
// v0.6.0's Mono decorator. evmParams/feemarketParams are loaded once per tx
// at handler construction time (NewAnteHandler calls newEVMAnteHandler with
// the live ctx) so the Mono decorator sees the same params throughout its
// run.
func newEVMAnteHandler(ctx sdk.Context, options HandlerOptions) sdk.AnteHandler {
	evmParams := options.EvmKeeper.GetParams(ctx)
	feemarketParams := options.FeeMarketKeeper.GetParams(ctx)
	decorators := []sdk.AnteDecorator{
		cosmosevmevm.NewEVMMonoDecorator(
			options.AccountKeeper,
			options.FeeMarketKeeper,
			options.EvmKeeper,
			options.MaxTxGasWanted,
			&evmParams,
			&feemarketParams,
		),
	}
	if options.PendingTxListener != nil {
		decorators = append(decorators, cosmosevmante.NewTxListenerDecorator(options.PendingTxListener))
	}
	return sdk.ChainAnteDecorators(decorators...)
}

// newCosmosAnteHandler builds the Cosmos-transaction AnteHandler. It mostly
// mirrors cosmos/evm v0.6.0's cosmos.go but swaps in moca's bespoke
// DeductFeeDecorator (which can settle fees out of unclaimed staking
// rewards) and moca's MinGasPriceDecorator (which checks against moca's
// evm-denom min gas price). The IBC redundant-relay decorator is dropped
// since moca does not run IBC.
func newCosmosAnteHandler(ctx sdk.Context, options HandlerOptions) sdk.AnteHandler {
	feemarketParams := options.FeeMarketKeeper.GetParams(ctx)
	// Cosmos txs are fee-checked with the EIP-1559 feemarket logic (effective
	// gas price vs the feemarket base fee), matching cosmos/evm's own cosmos
	// ante and moca's pre-migration behavior. Without this, NewDeductFeeDecorator
	// falls back to the node's app.toml min-gas-prices, which rejects txs that
	// satisfy the (lower) consensus base fee — breaking cross-node tx inclusion.
	txFeeChecker := options.TxFeeChecker
	if txFeeChecker == nil {
		// cosmos/evm's NewDynamicFeeChecker is an authante.TxFeeChecker
		// (func(ctx, sdk.Tx)); moca's DeductFeeDecorator wants an
		// anteutils.TxFeeChecker (func(ctx, sdk.FeeTx)). Adapt — an sdk.FeeTx
		// is an sdk.Tx.
		dynamicFeeChecker := cosmosevmevm.NewDynamicFeeChecker(&feemarketParams)
		txFeeChecker = func(ctx sdk.Context, feeTx sdk.FeeTx) (sdk.Coins, int64, error) {
			return dynamicFeeChecker(ctx, feeTx)
		}
	}
	return sdk.ChainAnteDecorators(
		cosmosante.RejectMessagesDecorator{}, // reject MsgEthereumTxs on cosmos path
		cosmosante.NewAuthzLimiterDecorator( // disallow these Msg types inside authz.MsgExec
			sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
			sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
		),
		ante.NewSetUpContextDecorator(),
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		cosmosante.NewMinGasPriceDecorator(options.FeeMarketKeeper, options.EvmKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		cosmosante.NewDeductFeeDecorator(
			options.AccountKeeper,
			options.BankKeeper,
			options.DistributionKeeper,
			options.FeegrantKeeper,
			txFeeChecker,
		),
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
		cosmosevmevm.NewGasWantedDecorator(options.EvmKeeper, options.FeeMarketKeeper, &feemarketParams),
	)
}
