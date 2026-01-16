package keeper

import (
	"fmt"

	"cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"

	"github.com/evmos/evmos/v12/x/payment/types"
)

type (
	Keeper struct {
		cdc      codec.BinaryCodec
		storeKey storetypes.StoreKey

		bankKeeper    types.BankKeeper
		accountKeeper types.AccountKeeper
		authority     string
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
	authority string,
) *Keeper {
	return &Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
		authority:     authority,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) QueryDynamicBalance(ctx sdk.Context, addr sdk.AccAddress) (amount sdkmath.Int, err error) {
	streamRecord, found := k.GetStreamRecord(ctx, addr)
	if !found {
		return sdkmath.ZeroInt(), nil
	}
	change := types.NewDefaultStreamRecordChangeWithAddr(addr)
	err = k.UpdateStreamRecord(ctx, streamRecord, change)
	if err != nil {
		return sdkmath.ZeroInt(), errors.Wrapf(err, "update stream record failed")
	}
	return streamRecord.StaticBalance, nil
}

func (k Keeper) Withdraw(ctx sdk.Context, fromAddr, toAddr sdk.AccAddress, amount sdkmath.Int) error {
	streamRecord, found := k.GetStreamRecord(ctx, fromAddr)
	if !found {
		return errors.Wrapf(types.ErrStreamRecordNotFound, "stream record not found %s", fromAddr.String())
	}
	change := types.NewDefaultStreamRecordChangeWithAddr(fromAddr).WithStaticBalanceChange(amount.Neg())
	err := k.UpdateStreamRecord(ctx, streamRecord, change)
	if err != nil {
		return errors.Wrapf(err, "update stream record failed %s", fromAddr.String())
	}
	k.SetStreamRecord(ctx, streamRecord)

	coins := sdk.NewCoins(sdk.NewCoin(k.GetParams(ctx).FeeDenom, amount))

	// Check if recipient is the distribution module account
	distModuleAcc := authtypes.NewModuleAddress(distrtypes.ModuleName)
	if toAddr.Equals(distModuleAcc) {
		// Transfer to distribution module using SendCoinsFromModuleToModule (bypasses blocklist)
		err = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, distrtypes.ModuleName, coins)
		if err != nil {
			return errors.Wrapf(err, "send coins from module to module failed, recipient module: %s", distrtypes.ModuleName)
		}
	} else {
		// Transfer to regular account using SendCoinsFromModuleToAccount
		err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, toAddr, coins)
		if err != nil {
			return errors.Wrapf(err, "send coins from module to account failed %s", toAddr.String())
		}
	}

	return nil
}

func (k Keeper) GetAuthority() string {
	return k.authority
}
