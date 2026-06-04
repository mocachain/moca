package types

import (
	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"

	mocatypes "github.com/mocachain/moca/v2/types"
)

func NewTokenPair(erc20Address common.Address, denom string, contractOwner Owner) TokenPair {
	return TokenPair{
		Erc20Address:  erc20Address.String(),
		Denom:         denom,
		Enabled:       true,
		ContractOwner: contractOwner,
	}
}

func (tp TokenPair) GetID() []byte {
	id := tp.Erc20Address + "|" + tp.Denom
	return tmhash.Sum([]byte(id))
}

func (tp TokenPair) GetERC20Contract() common.Address {
	return common.HexToAddress(tp.Erc20Address)
}

func (tp TokenPair) Validate() error {
	if err := sdk.ValidateDenom(tp.Denom); err != nil {
		return err
	}
	return mocatypes.ValidateAddress(tp.Erc20Address)
}

func (tp TokenPair) IsNativeCoin() bool {
	return tp.ContractOwner == OWNER_MODULE
}

func (tp TokenPair) IsNativeERC20() bool {
	return tp.ContractOwner == OWNER_EXTERNAL
}
