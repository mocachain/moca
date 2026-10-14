package v2

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/x/sp/types"
)

// MigrateStore sets max_sp_price, which params stored before consensus version 2
// do not have, to its default. A value already set and the other params are kept as stored.
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey, cdc codec.BinaryCodec) error {
	store := ctx.KVStore(storeKey)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return fmt.Errorf("%s params not found", types.ModuleName)
	}

	var params types.Params
	if err := cdc.Unmarshal(bz, &params); err != nil {
		return err
	}
	if params.MaxSpPrice.IsNil() {
		params.MaxSpPrice = types.DefaultMaxSpPrice
	}
	store.Set(types.ParamsKey, cdc.MustMarshal(&params))
	return nil
}
