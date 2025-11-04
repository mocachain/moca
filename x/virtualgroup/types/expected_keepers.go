package types

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	sptypes "github.com/evmos/evmos/v12/x/sp/types"
)

type SpKeeper interface {
	GetStorageProvider(ctx sdk.Context, id uint32) (sp *sptypes.StorageProvider, found bool)
	GetStorageProviderByOperatorAddr(ctx sdk.Context, addr sdk.AccAddress) (sp *sptypes.StorageProvider, found bool)
	GetStorageProviderByFundingAddr(ctx sdk.Context, sealAddr sdk.AccAddress) (sp *sptypes.StorageProvider, found bool)
	SetStorageProvider(ctx sdk.Context, sp *sptypes.StorageProvider)
	Exit(ctx sdk.Context, sp *sptypes.StorageProvider) error
	DepositDenomForSP(ctx sdk.Context) (res string)
	GetAllStorageProviders(ctx sdk.Context) (sps []sptypes.StorageProvider)
}

// AccountKeeper defines the expected account keeper used for simulations (noalias)
// Methods imported from account should be defined here
type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	IterateAccounts(ctx context.Context, process func(sdk.AccountI) bool)
	GetModuleAddress(name string) sdk.AccAddress
	GetModuleAccount(ctx context.Context, moduleName string) sdk.ModuleAccountI
	SetModuleAccount(ctx context.Context, macc sdk.ModuleAccountI)
}

// BankKeeper defines the expected interface needed to retrieve account balances.
// Methods imported from bank should be defined here
type BankKeeper interface {
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	LockedCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
}

type PaymentKeeper interface {
	QueryDynamicBalance(ctx sdk.Context, addr sdk.AccAddress) (amount sdkmath.Int, err error)
	Withdraw(ctx sdk.Context, fromAddr, toAddr sdk.AccAddress, amount sdkmath.Int) error
	IsEmptyNetFlow(ctx sdk.Context, account sdk.AccAddress) bool
}

type StorageKeeper interface {
	GetExpectSecondarySPNumForECObject(ctx sdk.Context, time int64) (res uint32)
}
