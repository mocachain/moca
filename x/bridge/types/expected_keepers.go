package types

import (
	"context"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
}

// StakingKeeper defines the expected interface needed to get staking related data
type StakingKeeper interface {
	BondDenom(ctx context.Context) (res string, err error)
}

type CrossChainKeeper interface {
	GetDestBscChainID() sdk.ChainID
	CreateRawIBCPackageWithFee(ctx context.Context, chainID sdk.ChainID, channelID sdk.ChannelID, packageType sdk.CrossChainPackageType,
		packageLoad []byte, relayerFee *big.Int, ackRelayerFee *big.Int,
	) (uint64, error)

	RegisterChannel(name string, id sdk.ChannelID, app sdk.CrossChainApplication) error
}
