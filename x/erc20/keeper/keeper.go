package keeper

import (
	"fmt"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"github.com/mocachain/moca/v2/x/erc20/types"
	evmkeeper "github.com/mocachain/moca/v2/x/evm/keeper"
)

type Keeper struct {
	storeKey storetypes.StoreKey
	cdc      codec.BinaryCodec

	authority sdk.AccAddress

	accountKeeper types.AccountKeeper
	bankKeeper    bankkeeper.Keeper
	evmKeeper     *evmkeeper.Keeper
}

func NewKeeper(
	storeKey storetypes.StoreKey,
	cdc codec.BinaryCodec,
	authority sdk.AccAddress,
	accountKeeper types.AccountKeeper,
	bankKeeper bankkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
) Keeper {
	if err := sdk.VerifyAddressFormat(authority); err != nil {
		panic(err)
	}

	return Keeper{
		authority:     authority,
		storeKey:      storeKey,
		cdc:           cdc,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		evmKeeper:     evmKeeper,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
