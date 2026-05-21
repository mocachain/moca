package app

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmosevmfeemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	feemarketkeeper "github.com/mocachain/moca/v2/x/feemarket/keeper"
)

// feeMarketKeeperAdapter adapts moca's x/feemarket keeper to the cosmos/evm
// x/vm types.FeeMarketKeeper interface, which the x/vm keeper requires at
// construction.
//
// moca keeps its own x/feemarket module; its keeper returns base fees as
// *big.Int and its Params carry BaseFee as math.Int, whereas cosmos/evm's
// interface expects math.LegacyDec throughout. This adapter performs the
// value-preserving conversions (an integer base fee N maps to the decimal N).
type feeMarketKeeperAdapter struct {
	keeper feemarketkeeper.Keeper
}

func newFeeMarketKeeperAdapter(k feemarketkeeper.Keeper) feeMarketKeeperAdapter {
	return feeMarketKeeperAdapter{keeper: k}
}

// GetBaseFee returns the current EIP-1559 base fee as a LegacyDec.
func (a feeMarketKeeperAdapter) GetBaseFee(ctx sdk.Context) sdkmath.LegacyDec {
	baseFee := a.keeper.GetBaseFee(ctx)
	if baseFee == nil {
		return sdkmath.LegacyZeroDec()
	}
	return sdkmath.LegacyNewDecFromBigInt(baseFee)
}

// CalculateBaseFee recomputes the base fee for the current block as a LegacyDec.
func (a feeMarketKeeperAdapter) CalculateBaseFee(ctx sdk.Context) sdkmath.LegacyDec {
	baseFee := a.keeper.CalculateBaseFee(ctx)
	if baseFee == nil {
		return sdkmath.LegacyZeroDec()
	}
	return sdkmath.LegacyNewDecFromBigInt(baseFee)
}

// GetParams converts moca's x/feemarket params into the cosmos/evm shape. The
// two structs are field-identical except for BaseFee (math.Int vs LegacyDec).
func (a feeMarketKeeperAdapter) GetParams(ctx sdk.Context) cosmosevmfeemarkettypes.Params {
	p := a.keeper.GetParams(ctx)
	return cosmosevmfeemarkettypes.Params{
		NoBaseFee:                p.NoBaseFee,
		BaseFeeChangeDenominator: p.BaseFeeChangeDenominator,
		ElasticityMultiplier:     p.ElasticityMultiplier,
		EnableHeight:             p.EnableHeight,
		BaseFee:                  sdkmath.LegacyNewDecFromInt(p.BaseFee),
		MinGasPrice:              p.MinGasPrice,
		MinGasMultiplier:         p.MinGasMultiplier,
	}
}
