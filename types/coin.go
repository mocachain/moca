package types

import (
	"math/big"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// AttoMoca defines the default coin denomination used in Moca in:
	//
	// - Staking parameters: denomination used as stake in the dPoS chain
	// - Mint parameters: denomination minted due to fee distribution rewards
	// - Governance parameters: denomination used for spam prevention in proposal deposits
	// - EVM parameters: denomination used for running EVM state transitions in Moca.
	AttoMoca string = "amoca"

	// BaseDenomUnit defines the base denomination unit for Moca.
	// 1 moca = 1x10^{BaseDenomUnit} amoca
	BaseDenomUnit = 18

	// DefaultGasPrice is default gas price for evm transactions
	DefaultGasPrice = 20
)

// PowerReduction defines the default power reduction value for staking
var PowerReduction = sdkmath.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(BaseDenomUnit), nil))

// NewMocaCoin is a utility function that returns an "amoca" coin with the given sdkmath.Int amount.
// The function will panic if the provided amount is negative.
func NewMocaCoin(amount sdkmath.Int) sdk.Coin {
	return sdk.NewCoin(AttoMoca, amount)
}

// NewMocaDecCoin is a utility function that returns an "amoca" decimal coin with the given sdkmath.Int amount.
// The function will panic if the provided amount is negative.
func NewMocaDecCoin(amount sdkmath.Int) sdk.DecCoin {
	return sdk.NewDecCoin(AttoMoca, amount)
}

// NewMocaCoinInt64 is a utility function that returns an "amoca" coin with the given int64 amount.
// The function will panic if the provided amount is negative.
func NewMocaCoinInt64(amount int64) sdk.Coin {
	return sdk.NewInt64Coin(AttoMoca, amount)
}
