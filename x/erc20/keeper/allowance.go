package keeper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/mocachain/moca/v2/x/erc20/types"
)

func (k Keeper) GetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) (*big.Int, error) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAllowance)
	bz := store.Get(types.AllowanceKey(erc20, owner, spender))
	if len(bz) == 0 {
		return common.Big0, nil
	}

	var allowance types.Allowance
	k.cdc.MustUnmarshal(bz, &allowance)
	return allowance.Value.BigInt(), nil
}

func (k Keeper) SetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
	value *big.Int,
) error {
	return k.setAllowance(ctx, erc20, owner, spender, value, false)
}

func (k Keeper) DeleteAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) error {
	return k.setAllowance(ctx, erc20, owner, spender, common.Big0, false)
}

func (k Keeper) UnsafeSetAllowance(ctx sdk.Context, erc20, owner, spender common.Address, value *big.Int) error {
	return k.setAllowance(ctx, erc20, owner, spender, value, true)
}

func (k Keeper) setAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
	value *big.Int,
	allowDisabledTokenPair bool,
) error {
	tokenPairID := k.GetERC20Map(ctx, erc20)
	tokenPair, found := k.GetTokenPair(ctx, tokenPairID)
	if !found {
		return errorsmod.Wrapf(types.ErrTokenPairNotFound, "token pair for address '%s' not registered", erc20)
	}
	if !allowDisabledTokenPair && !tokenPair.Enabled {
		return errorsmod.Wrapf(types.ErrERC20Disabled, "token pair for address '%s' is disabled", erc20)
	}
	if (owner == common.Address{}) {
		return errorsmod.Wrap(errortypes.ErrInvalidAddress, "owner address is empty")
	}
	if (spender == common.Address{}) {
		return errorsmod.Wrap(errortypes.ErrInvalidAddress, "spender address is empty")
	}
	if value == nil {
		return errorsmod.Wrap(types.ErrInvalidAllowance, "value is nil")
	}
	if value.Sign() < 0 {
		return errorsmod.Wrapf(types.ErrInvalidAllowance, "value '%s' is less than zero", value)
	}
	if value.BitLen() > 256 {
		return errorsmod.Wrapf(types.ErrInvalidAllowance, "value '%s' is greater than max value of uint256", value)
	}

	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAllowance)
	key := types.AllowanceKey(erc20, owner, spender)
	if value.Sign() == 0 {
		store.Delete(key)
		return nil
	}

	allowance := types.NewAllowance(erc20, owner, spender, value)
	if err := allowance.Validate(); err != nil {
		return err
	}

	store.Set(key, k.cdc.MustMarshal(&allowance))
	return nil
}

func (k Keeper) GetAllowances(ctx sdk.Context) []types.Allowance {
	allowances := []types.Allowance{}
	k.IterateAllowances(ctx, func(allowance types.Allowance) bool {
		allowances = append(allowances, allowance)
		return false
	})
	return allowances
}

func (k Keeper) IterateAllowances(ctx sdk.Context, cb func(allowance types.Allowance) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	iterator := storetypes.KVStorePrefixIterator(store, types.KeyPrefixAllowance)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var allowance types.Allowance
		k.cdc.MustUnmarshal(iterator.Value(), &allowance)
		if cb(allowance) {
			break
		}
	}
}

func (k Keeper) deleteAllowances(ctx sdk.Context, erc20 common.Address) {
	store := ctx.KVStore(k.storeKey)
	iterator := storetypes.KVStorePrefixIterator(store, append(types.KeyPrefixAllowance, erc20.Bytes()...))
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		store.Delete(iterator.Key())
	}
}
