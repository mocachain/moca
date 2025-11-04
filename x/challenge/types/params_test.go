package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/x/challenge/types"
)

func Test_validateParams(t *testing.T) {
	params := types.DefaultParams()

	// default params have no error
	require.NoError(t, params.Validate())

	// validate slash amount min
	params.SlashAmountMin = math.NewInt(-1)
	require.Error(t, params.Validate())

	// validate slash amount max
	params.SlashAmountMin = math.NewInt(1)
	params.SlashAmountMax = math.NewInt(-1)
	require.Error(t, params.Validate())

	params.SlashAmountMin = math.NewInt(10)
	params.SlashAmountMax = math.NewInt(1)
	require.Error(t, params.Validate())

	params.SlashAmountMin = math.NewInt(1)
	params.SlashAmountMax = math.NewInt(10)
	require.NoError(t, params.Validate())

	// validate reward validator ratio
	params.RewardValidatorRatio = math.LegacyNewDec(-1)
	require.Error(t, params.Validate())

	// validate reward submitter ratio
	params.RewardValidatorRatio = math.LegacyNewDecWithPrec(5, 1)
	params.RewardSubmitterRatio = math.LegacyNewDec(-1)
	require.Error(t, params.Validate())

	params.RewardValidatorRatio = math.LegacyNewDecWithPrec(8, 1)
	params.RewardSubmitterRatio = math.LegacyNewDecWithPrec(7, 1)
	require.Error(t, params.Validate())

	// validate submitter reward threshold
	params.RewardValidatorRatio = math.LegacyNewDecWithPrec(5, 1)
	params.RewardSubmitterRatio = math.LegacyNewDecWithPrec(4, 1)
	params.RewardSubmitterThreshold = math.NewInt(-1)
	require.Error(t, params.Validate())

	// validate heartbeat interval
	params.RewardSubmitterThreshold = math.NewInt(100)
	params.HeartbeatInterval = 0
	require.Error(t, params.Validate())

	// validate attestation inturn interval
	params.HeartbeatInterval = 100
	params.AttestationInturnInterval = 0
	require.Error(t, params.Validate())

	// validate attestation kept count
	params.AttestationInturnInterval = 120
	params.AttestationKeptCount = 0
	require.Error(t, params.Validate())

	// no error
	params.AttestationKeptCount = 100
	require.NoError(t, params.Validate())
}
