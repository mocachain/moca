package app

import (
	"encoding/json"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/stretchr/testify/require"

	legacyevmtypes "github.com/mocachain/moca/v2/x/legacy/evm/types"
)

// TestGovQueriesDecodePreV2Proposals stores the proposals that carry the pre-v2 EVM
// MsgUpdateParams, byte for byte as the live networks hold them, and checks that the
// gov queries return them with the messages v1.4.3 returned.
func TestGovQueriesDecodePreV2Proposals(t *testing.T) {
	bz, err := os.ReadFile("testdata/pre_v2_gov_proposals.json")
	require.NoError(t, err)
	var fixtures []struct {
		ChainID  string          `json:"chain_id"`
		ID       uint64          `json:"id"`
		Value    []byte          `json:"value"`
		Messages json.RawMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(bz, &fixtures))
	require.NotEmpty(t, fixtures)

	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)
	store := ctx.KVStore(mocaApp.GetKey(govtypes.StoreKey))
	for _, f := range fixtures {
		store.Set(append(govtypes.ProposalsKeyPrefix.Bytes(), sdk.Uint64ToBigEndian(f.ID)...), f.Value)
	}

	queryServer := govkeeper.NewQueryServer(&mocaApp.GovKeeper)
	legacyQueryServer := govkeeper.NewLegacyQueryServer(&mocaApp.GovKeeper)
	all, err := queryServer.Proposals(ctx, &govv1.QueryProposalsRequest{})
	require.NoError(t, err)
	require.Len(t, all.Proposals, len(fixtures))

	for _, f := range fixtures {
		_, err := legacyQueryServer.Proposal(ctx, &govv1beta1.QueryProposalRequest{ProposalId: f.ID})
		require.NoError(t, err, "%s proposal %d", f.ChainID, f.ID)
		res, err := queryServer.Proposal(ctx, &govv1.QueryProposalRequest{ProposalId: f.ID})
		require.NoError(t, err, "%s proposal %d", f.ChainID, f.ID)

		// Decoding loses nothing the stored proposal holds.
		bz, err := mocaApp.AppCodec().Marshal(res.Proposal)
		require.NoError(t, err)
		require.Equal(t, f.Value, bz, "%s proposal %d", f.ChainID, f.ID)

		bz, err = mocaApp.AppCodec().MarshalJSON(res.Proposal)
		require.NoError(t, err)
		var got struct {
			Messages json.RawMessage `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(bz, &got))
		require.JSONEq(t, string(f.Messages), string(got.Messages), "%s proposal %d", f.ChainID, f.ID)
	}
}

// TestPreV2MsgUpdateParamsFailsInTxs checks that the app decodes the pre-v2 EVM
// MsgUpdateParams but runs no transaction that carries it.
func TestPreV2MsgUpdateParamsFailsInTxs(t *testing.T) {
	mocaApp := EthSetup(false, nil)
	ctx := mocaApp.NewContext(false)
	gov := authtypes.NewModuleAddress(govtypes.ModuleName)
	msg := &legacyevmtypes.MsgUpdateParams{Authority: gov.String()} //nolint:staticcheck

	builder := mocaApp.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	bz, err := mocaApp.GetTxConfig().TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	_, _, err = mocaApp.Simulate(bz)
	require.ErrorIs(t, err, sdkerrors.ErrUnknownRequest)

	_, err = mocaApp.GovKeeper.SubmitProposal(ctx, []sdk.Msg{msg}, "", "title", "summary", gov, false)
	require.ErrorIs(t, err, govtypes.ErrUnroutableProposalMsg)
}
