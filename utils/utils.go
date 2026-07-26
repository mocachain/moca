package utils

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// MainnetChainID defines the Moca EIP155 chain ID for mainnet
	MainnetChainID = "moca_5252"
	// TestnetChainID defines the Moca EIP155 chain ID base for testnet
	TestnetChainID = "moca_222888"
	// BaseDenom defines the Moca base denomination
	BaseDenom = "amoca"
)

// IsMainnet returns true if the chain-id has the Moca mainnet EIP155 chain prefix.
func IsMainnet(chainID string) bool {
	return strings.HasPrefix(chainID, MainnetChainID)
}

func AccAddressMustToHexAddress(accStrAddress string) common.Address {
	ValAddress, err := sdk.AccAddressFromBech32(accStrAddress)
	var hexAddress common.Address
	if err == nil {
		hexAddress = common.BytesToAddress(ValAddress)
	}
	return hexAddress
}
