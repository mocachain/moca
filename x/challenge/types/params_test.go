package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/challenge/types"
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

// Each parameter needs its own key. They are one copy-paste apart from each other,
// and a duplicate would silently make two parameters share a slot.
func TestParamKeysAreUnique(t *testing.T) {
	keys := map[string]string{
		"ChallengeCountPerBlock":    string(types.KeyChallengeCountPerBlock),
		"ChallengeKeepAlivePeriod":  string(types.KeyChallengeKeepAlivePeriod),
		"SlashCoolingOffPeriod":     string(types.KeySlashCoolingOffPeriod),
		"SlashAmountSizeRate":       string(types.KeySlashAmountSizeRate),
		"SlashAmountMin":            string(types.KeySlashAmountMin),
		"SlashAmountMax":            string(types.KeySlashAmountMax),
		"RewardValidatorRatio":      string(types.KeyRewardValidatorRatio),
		"RewardSubmitterRatio":      string(types.KeyRewardSubmitterRatio),
		"RewardSubmitterThreshold":  string(types.KeyRewardSubmitterThreshold),
		"HeartbeatInterval":         string(types.KeyHeartbeatInterval),
		"AttestationInturnInterval": string(types.KeyAttestationInturnInterval),
		"AttestationKeptCount":      string(types.KeyAttestationKeptCount),
		"SpSlashMaxAmount":          string(types.KeySpSlashMaxAmount),
		"SpSlashCountingWindow":     string(types.KeySpSlashCountingWindow),
	}

	seen := make(map[string]string, len(keys))
	for name, key := range keys {
		if other, dup := seen[key]; dup {
			t.Errorf("%s and %s share the parameter key %q", name, other, key)
		}
		seen[key] = name
	}
}
