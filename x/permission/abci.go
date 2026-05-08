package permission

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/mocachain/moca/v2/x/permission/keeper"
)

func EndBlocker(ctx sdk.Context, k keeper.Keeper) error {
	k.RemoveExpiredPolicies(ctx)
	return nil
}
