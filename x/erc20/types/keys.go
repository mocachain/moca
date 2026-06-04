package types

import (
	"github.com/ethereum/go-ethereum/common"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

const (
	ModuleName = "erc20"
	StoreKey   = ModuleName
	RouterKey  = ModuleName
)

var ModuleAddress common.Address

func init() {
	ModuleAddress = common.BytesToAddress(authtypes.NewModuleAddress(ModuleName).Bytes())
}

const (
	prefixTokenPair = iota + 1
	prefixTokenPairByERC20
	prefixTokenPairByDenom
	prefixSTRv2Addresses
	prefixAllowance
)

var (
	KeyPrefixTokenPair        = []byte{prefixTokenPair}
	KeyPrefixTokenPairByERC20 = []byte{prefixTokenPairByERC20}
	KeyPrefixTokenPairByDenom = []byte{prefixTokenPairByDenom}
	KeyPrefixSTRv2Addresses   = []byte{prefixSTRv2Addresses}
	KeyPrefixAllowance        = []byte{prefixAllowance}
)

func AllowanceKey(erc20, owner, spender common.Address) []byte {
	return append(append(erc20.Bytes(), owner.Bytes()...), spender.Bytes()...)
}
