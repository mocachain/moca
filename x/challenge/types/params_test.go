package types

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func Test_validateParams(t *testing.T) {
	params := DefaultParams()

	// default params have no error
	require.NoError(t, params.Validate())

	// validate challenge keep alive period
	params.ChallengeKeepAlivePeriod = 0
	require.Error(t, params.Validate())
	params.ChallengeKeepAlivePeriod = DefaultChallengeKeepAlivePeriod

	// validate slash amount size rate
	params.SlashAmountSizeRate = math.LegacyDec{}
	require.Error(t, params.Validate())

	params.SlashAmountSizeRate = math.LegacyNewDec(-1)
	require.Error(t, params.Validate())
	params.SlashAmountSizeRate = DefaultSlashAmountSizeRate

	// validate slash amount min
	params.SlashAmountMin = math.Int{}
	require.Error(t, params.Validate())

	params.SlashAmountMin = math.NewInt(-1)
	require.Error(t, params.Validate())

	// validate slash amount max
	params.SlashAmountMin = math.NewInt(1)
	params.SlashAmountMax = math.Int{}
	require.Error(t, params.Validate())

	params.SlashAmountMax = math.NewInt(-1)
	require.Error(t, params.Validate())

	params.SlashAmountMin = math.NewInt(10)
	params.SlashAmountMax = math.NewInt(1)
	require.Error(t, params.Validate())

	params.SlashAmountMin = math.NewInt(1)
	params.SlashAmountMax = math.NewInt(10)
	require.NoError(t, params.Validate())

	// validate reward validator ratio
	params.RewardValidatorRatio = math.LegacyDec{}
	require.Error(t, params.Validate())

	params.RewardValidatorRatio = math.LegacyNewDec(-1)
	require.Error(t, params.Validate())

	// validate reward submitter ratio
	params.RewardValidatorRatio = math.LegacyNewDecWithPrec(5, 1)
	params.RewardSubmitterRatio = math.LegacyDec{}
	require.Error(t, params.Validate())

	params.RewardSubmitterRatio = math.LegacyNewDec(-1)
	require.Error(t, params.Validate())

	params.RewardValidatorRatio = math.LegacyNewDecWithPrec(8, 1)
	params.RewardSubmitterRatio = math.LegacyNewDecWithPrec(7, 1)
	require.Error(t, params.Validate())

	// validate submitter reward threshold
	params.RewardValidatorRatio = math.LegacyNewDecWithPrec(5, 1)
	params.RewardSubmitterRatio = math.LegacyNewDecWithPrec(4, 1)
	params.RewardSubmitterThreshold = math.Int{}
	require.Error(t, params.Validate())

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
	params.AttestationKeptCount = 100

	// validate storage provider slash max amount
	params.SpSlashMaxAmount = math.Int{}
	require.Error(t, params.Validate())

	params.SpSlashMaxAmount = math.NewInt(-1)
	require.Error(t, params.Validate())
	params.SpSlashMaxAmount = DefaultSpSlashMaxAmount

	// validate storage provider slash counting window
	params.SpSlashCountingWindow = 0
	require.Error(t, params.Validate())
	params.SpSlashCountingWindow = DefaultSpSlashCountingWindow

	// no error
	require.NoError(t, params.Validate())
}

// Test_validateParams_invalidType exercises the "wrong Go type" guard inside
// every validateXxx function. Params.Validate() can never trigger this branch
// itself (each field is always the declared Go type by construction, so the
// type assertion always succeeds) -- it is only reachable by calling the
// unexported validators directly with a value of the wrong type.
func Test_validateParams_invalidType(t *testing.T) {
	validators := []func(interface{}) error{
		validateChallengeCountPerBlock,
		validateChallengeKeepAlivePeriod,
		validateSlashCoolingOffPeriod,
		validateSlashAmountSizeRate,
		validateSlashAmountMin,
		validateSlashAmountMax,
		validateRewardValidatorRatio,
		validateRewardSubmitterRatio,
		validateRewardSubmitterThreshold,
		validateHeartbeatInterval,
		validateAttestationInturnInterval,
		validateAttestationKeptCount,
		validateSpSlashMaxAmount,
		validateSpSlashCountingWindow,
	}
	for _, validate := range validators {
		require.Error(t, validate("not-the-right-type"))
	}
}

func TestParams_String(t *testing.T) {
	str := DefaultParams().String()
	require.NotEmpty(t, str)
	require.Contains(t, str, "challenge_count_per_block")
}

// Each parameter needs its own key. They are one copy-paste apart from each other,
// and a duplicate would silently make two parameters share a slot.
func TestParamKeysAreUnique(t *testing.T) {
	keys := map[string]string{
		"ChallengeCountPerBlock":    string(KeyChallengeCountPerBlock),
		"ChallengeKeepAlivePeriod":  string(KeyChallengeKeepAlivePeriod),
		"SlashCoolingOffPeriod":     string(KeySlashCoolingOffPeriod),
		"SlashAmountSizeRate":       string(KeySlashAmountSizeRate),
		"SlashAmountMin":            string(KeySlashAmountMin),
		"SlashAmountMax":            string(KeySlashAmountMax),
		"RewardValidatorRatio":      string(KeyRewardValidatorRatio),
		"RewardSubmitterRatio":      string(KeyRewardSubmitterRatio),
		"RewardSubmitterThreshold":  string(KeyRewardSubmitterThreshold),
		"HeartbeatInterval":         string(KeyHeartbeatInterval),
		"AttestationInturnInterval": string(KeyAttestationInturnInterval),
		"AttestationKeptCount":      string(KeyAttestationKeptCount),
		"SpSlashMaxAmount":          string(KeySpSlashMaxAmount),
		"SpSlashCountingWindow":     string(KeySpSlashCountingWindow),
	}

	seen := make(map[string]string, len(keys))
	for name, key := range keys {
		if other, dup := seen[key]; dup {
			t.Errorf("%s and %s share the parameter key %q", name, other, key)
		}
		seen[key] = name
	}
}
