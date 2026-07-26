package types

import (
	"math/big"

	sdkmath "cosmossdk.io/math"
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
)

// PowerReduction defines the default power reduction value for staking
var PowerReduction = sdkmath.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(BaseDenomUnit), nil))
