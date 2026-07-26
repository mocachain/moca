package testutil

import (
	"fmt"
	"time"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/testutil/tx"
)

// Commit commits a block at a given time. Reminder: At the end of each
// Tendermint Consensus round the following methods are run
//  1. BeginBlock
//  2. DeliverTx
//  3. EndBlock
//  4. Commit
func Commit(ctx sdk.Context, app *app.Moca, t time.Duration, vs *cmttypes.ValidatorSet) (sdk.Context, error) {
	header, err := commit(ctx, app, t, vs)
	if err != nil {
		return ctx, err
	}

	return ctx.WithBlockHeader(header), nil
}

// DeliverTx delivers a cosmos tx for a given set of msgs
func DeliverTx(
	ctx sdk.Context,
	appMoca *app.Moca,
	priv cryptotypes.PrivKey,
	gasPrice *sdkmath.Int,
	msgs ...sdk.Msg,
) (abci.ExecTxResult, error) {
	txConfig := appMoca.GetTxConfig()
	tx, err := tx.PrepareCosmosTx(
		ctx,
		appMoca,
		tx.CosmosTxArgs{
			TxCfg:    txConfig,
			Priv:     priv,
			ChainID:  ctx.ChainID(),
			Gas:      10_000_000,
			GasPrice: gasPrice,
			Msgs:     msgs,
		},
	)
	if err != nil {
		return abci.ExecTxResult{}, err
	}
	return BroadcastTxBytes(appMoca, txConfig.TxEncoder(), tx, ctx.BlockHeader().ProposerAddress)
}

// DeliverEthTx generates and broadcasts a Cosmos Tx populated with MsgEthereumTx messages.
// ctx carries the proposer used by FinalizeBlock; EVM coinbase lookup needs it.
func DeliverEthTx(
	ctx sdk.Context,
	appMoca *app.Moca,
	priv cryptotypes.PrivKey,
	msgs ...sdk.Msg,
) (abci.ExecTxResult, error) {
	txConfig := appMoca.GetTxConfig()

	tx, err := tx.PrepareEthTx(txConfig, appMoca, priv, msgs...)
	if err != nil {
		return abci.ExecTxResult{}, err
	}
	return BroadcastTxBytes(appMoca, txConfig.TxEncoder(), tx, ctx.BlockHeader().ProposerAddress)
}

// CheckTx checks a cosmos tx for a given set of msgs
func CheckTx(
	ctx sdk.Context,
	appMoca *app.Moca,
	priv cryptotypes.PrivKey,
	gasPrice *sdkmath.Int,
	msgs ...sdk.Msg,
) (abci.ResponseCheckTx, error) {
	txConfig := appMoca.GetTxConfig()

	tx, err := tx.PrepareCosmosTx(
		ctx,
		appMoca,
		tx.CosmosTxArgs{
			TxCfg:    txConfig,
			Priv:     priv,
			ChainID:  ctx.ChainID(),
			GasPrice: gasPrice,
			Gas:      10_000_000,
			Msgs:     msgs,
		},
	)
	if err != nil {
		return abci.ResponseCheckTx{}, err
	}
	return checkTxBytes(appMoca, txConfig.TxEncoder(), tx)
}

// CheckEthTx checks a Ethereum tx for a given set of msgs
func CheckEthTx(
	appMoca *app.Moca,
	priv cryptotypes.PrivKey,
	msgs ...sdk.Msg,
) (abci.ResponseCheckTx, error) {
	txConfig := appMoca.GetTxConfig()

	tx, err := tx.PrepareEthTx(txConfig, appMoca, priv, msgs...)
	if err != nil {
		return abci.ResponseCheckTx{}, err
	}
	return checkTxBytes(appMoca, txConfig.TxEncoder(), tx)
}

// BroadcastTxBytes encodes a transaction and calls DeliverTx on the app.
// proposerAddr is required by EVM's coinbase lookup (must match a bonded validator in CMS).
func BroadcastTxBytes(app *app.Moca, txEncoder sdk.TxEncoder, tx sdk.Tx, proposerAddr []byte) (abci.ExecTxResult, error) {
	bz, err := txEncoder(tx)
	if err != nil {
		return abci.ExecTxResult{}, err
	}

	req := abci.RequestFinalizeBlock{
		Height:          app.LastBlockHeight() + 1,
		ProposerAddress: proposerAddr,
		Txs:             [][]byte{bz},
	}

	res, err := app.BaseApp.FinalizeBlock(&req)
	if err != nil {
		return abci.ExecTxResult{}, err
	}
	if len(res.TxResults) != 1 {
		return abci.ExecTxResult{}, fmt.Errorf("unexpected transaction results. Expected 1, got: %d", len(res.TxResults))
	}
	txRes := res.TxResults[0]
	if txRes.Code != 0 {
		return abci.ExecTxResult{}, errorsmod.Wrapf(errortypes.ErrInvalidRequest, "log: %s", txRes.Log)
	}

	return *txRes, nil
}

// commit is a private helper function that finalizes the current block via
// FinalizeBlock (which internally runs BeginBlocker, tx execution, and
// EndBlocker), commits the resulting state, and advances the header for the
// next block.
//
// Under Cosmos SDK 0.50 ABCI++ the call to FinalizeBlock is required for the
// cache writes produced during InitChain (or previous state mutations) to be
// flushed into the main CommitMultiStore via workingHash(). Calling
// EndBlocker + Commit directly skips that flush and causes subsequent reads of
// genesis-initialised state (e.g. distribution FeePool) to fail with
// "collections: not found".
//
// FinalizeBlock also requires Height >= 1 and Height == LastBlockHeight + 1.
// Several callers (e.g. AnteTestSuite.SetupTest) deliberately rewind the
// ctx header height before calling Commit, so we derive the target height
// from app.LastBlockHeight() instead of trusting ctx.BlockHeader().Height.
// This mirrors what BroadcastTxBytes already does.
func commit(ctx sdk.Context, app *app.Moca, t time.Duration, vs *cmttypes.ValidatorSet) (tmproto.Header, error) {
	header := ctx.BlockHeader()
	nextHeight := app.LastBlockHeight() + 1
	req := abci.RequestFinalizeBlock{
		Height:          nextHeight,
		ProposerAddress: header.ProposerAddress,
	}

	res, err := app.FinalizeBlock(&req)
	if err != nil {
		return header, err
	}

	if vs != nil {
		nextVals, err := applyValSetChanges(vs, res.ValidatorUpdates)
		if err != nil {
			return header, err
		}
		header.ValidatorsHash = vs.Hash()
		header.NextValidatorsHash = nextVals.Hash()

		// Rotate proposer like CometBFT does between blocks, so a validator
		// removed by the just-applied updates can't linger as
		// ctx.BlockHeader().ProposerAddress and break EVM's coinbase lookup
		// in subsequent DeliverEthTx calls.
		nextVals.IncrementProposerPriority(1)
		header.ProposerAddress = nextVals.Proposer.Address
	}

	if _, err := app.Commit(); err != nil {
		return header, err
	}

	header.Height = app.LastBlockHeight() + 1
	header.Time = header.Time.Add(t)
	header.AppHash = app.LastCommitID().Hash

	return header, nil
}

// checkTxBytes encodes a transaction and calls checkTx on the app.
func checkTxBytes(app *app.Moca, txEncoder sdk.TxEncoder, tx sdk.Tx) (abci.ResponseCheckTx, error) {
	bz, err := txEncoder(tx)
	if err != nil {
		return abci.ResponseCheckTx{}, err
	}

	req := abci.RequestCheckTx{Tx: bz}
	res, err := app.BaseApp.CheckTx(&req)
	if err != nil {
		return abci.ResponseCheckTx{}, err
	}
	if res.Code != 0 {
		return abci.ResponseCheckTx{}, errorsmod.Wrapf(errortypes.ErrInvalidRequest, "log: %s", res.Log)
	}

	return *res, nil
}

// applyValSetChanges takes in cmttypes.ValidatorSet and []abci.ValidatorUpdate and will return a new cmttypes.ValidatorSet which has the
// provided validator updates applied to the provided validator set.
func applyValSetChanges(valSet *cmttypes.ValidatorSet, valUpdates []abci.ValidatorUpdate) (*cmttypes.ValidatorSet, error) {
	updates, err := cmttypes.PB2TM.ValidatorUpdates(valUpdates)
	if err != nil {
		return nil, err
	}

	// must copy since validator set will mutate with UpdateWithChangeSet
	newVals := valSet.Copy()
	err = newVals.UpdateWithChangeSet(updates)
	if err != nil {
		return nil, err
	}

	return newVals, nil
}
