package types

import errorsmod "cosmossdk.io/errors"

var (
	ErrERC20Disabled          = errorsmod.Register(ModuleName, 2, "erc20 module is disabled")
	ErrTokenPairNotFound      = errorsmod.Register(ModuleName, 4, "token pair not found")
	ErrTokenPairAlreadyExists = errorsmod.Register(ModuleName, 5, "token pair already exists")
	ErrInvalidAllowance       = errorsmod.Register(ModuleName, 18, "invalid allowance")
)
