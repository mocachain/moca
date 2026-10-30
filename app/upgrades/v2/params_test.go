package v2_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/stretchr/testify/require"

	v2 "github.com/mocachain/moca/v2/app/upgrades/v2"
	"github.com/mocachain/moca/v2/encoding"
)

// TestNetworkForChainID verifies the chain-id -> network map, including the
// devnet fallback for unknown/ad-hoc ids used by local and in-process chains.
func TestNetworkForChainID(t *testing.T) {
	require.Equal(t, v2.Devnet, v2.NetworkForChainID("moca_5151-1"))
	require.Equal(t, v2.Testnet, v2.NetworkForChainID("moca_222888-1"))
	require.Equal(t, v2.Mainnet, v2.NetworkForChainID("moca_2288-1"))
	require.Equal(t, v2.Devnet, v2.NetworkForChainID("moca_99999-1"), "unknown id falls back to devnet")
	require.Equal(t, v2.Devnet, v2.NetworkForChainID(""), "empty id falls back to devnet")
}

// TestEmbeddedParamsRoundTrip proves every committed per-network snapshot is
// valid proto-JSON the app codec accepts (the comment header is stripped) and
// unmarshals into the cosmos/evm param types. The handler relies on this on the
// live upgrade.
func TestEmbeddedParamsRoundTrip(t *testing.T) {
	cdc := encoding.MakeConfig().Codec

	for _, n := range []v2.Network{v2.Devnet, v2.Testnet, v2.Mainnet} {
		evmJSON, err := v2.EVMParamsJSON(n)
		require.NoError(t, err, "%s evm json", n)
		var evmParams evmtypes.Params
		require.NoError(t, cdc.UnmarshalJSON(evmJSON, &evmParams), "%s evm unmarshal", n)
		require.NoError(t, evmParams.Validate(), "%s evm params valid", n)
		require.Equal(t, "amoca", evmParams.EvmDenom, "%s", n)
		require.Len(t, evmParams.ActiveStaticPrecompiles, 11, "%s precompiles", n)

		feeJSON, err := v2.FeeMarketParamsJSON(n)
		require.NoError(t, err, "%s feemarket json", n)
		var feeParams feemarkettypes.Params
		require.NoError(t, cdc.UnmarshalJSON(feeJSON, &feeParams), "%s feemarket unmarshal", n)
		require.NoError(t, feeParams.Validate(), "%s feemarket params valid", n)
		// The snapshots carry the values every live network runs on v1.4.3.
		require.Equal(t, []int64{3855}, evmParams.ExtraEIPs, "%s extra_eips", n)
		require.Empty(t, evmParams.EVMChannels, "%s evm_channels (IBC is removed in v2)", n)
		require.Equal(t, sdkmath.LegacyNewDec(1_000_000_000), feeParams.MinGasPrice, "%s min_gas_price", n)
		require.Equal(t, sdkmath.LegacyNewDec(1_000_000_000), feeParams.BaseFee, "%s base_fee", n)
		require.False(t, feeParams.NoBaseFee, "%s no_base_fee", n)
		require.Equal(t, uint32(8), feeParams.BaseFeeChangeDenominator, "%s base_fee_change_denominator", n)
		require.Equal(t, uint32(2), feeParams.ElasticityMultiplier, "%s elasticity_multiplier", n)
		require.Equal(t, sdkmath.LegacyNewDecWithPrec(5, 1), feeParams.MinGasMultiplier, "%s min_gas_multiplier", n)
	}
}
