package app

import (
	"strings"
	"testing"

	"cosmossdk.io/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestGovTally_LowercaseOperatorAddress_CountsVote(t *testing.T) {
	mocaApp := EthSetup(false, func(app *Moca, genesis simapp.GenesisState) simapp.GenesisState {
		var stakingGenesis stakingtypes.GenesisState
		app.AppCodec().MustUnmarshalJSON(genesis[stakingtypes.ModuleName], &stakingGenesis)
		for i := range stakingGenesis.Validators {
			stakingGenesis.Validators[i].OperatorAddress = strings.ToLower(stakingGenesis.Validators[i].OperatorAddress)
		}
		genesis[stakingtypes.ModuleName] = app.AppCodec().MustMarshalJSON(&stakingGenesis)
		return genesis
	})
	ctx := mocaApp.NewContext(false)

	validators, err := mocaApp.StakingKeeper.GetAllValidators(ctx)
	require.NoError(t, err)
	require.Len(t, validators, 1)
	validator := validators[0]
	require.Equal(t, strings.ToLower(validator.OperatorAddress), validator.OperatorAddress)
	require.True(t, validator.GetBondedTokens().IsPositive())

	voter, err := sdk.AccAddressFromHexUnsafe(validator.OperatorAddress)
	require.NoError(t, err)

	proposal, err := mocaApp.GovKeeper.SubmitProposal(ctx, nil, "", "title", "summary", voter, false)
	require.NoError(t, err)
	require.NoError(t, mocaApp.GovKeeper.ActivateVotingPeriod(ctx, proposal))
	require.NoError(t, mocaApp.GovKeeper.AddVote(ctx, proposal.Id, voter, govv1.NewNonSplitVoteOption(govv1.OptionYes), ""))

	proposal, err = mocaApp.GovKeeper.Proposals.Get(ctx, proposal.Id)
	require.NoError(t, err)
	_, _, tally, err := mocaApp.GovKeeper.Tally(ctx, proposal)
	require.NoError(t, err)
	require.Equal(t, validator.GetBondedTokens().String(), tally.YesCount)
}
