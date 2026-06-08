package erc20

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"

	"github.com/mocachain/moca/v2/x/erc20/keeper"
	"github.com/mocachain/moca/v2/x/erc20/types"
)

func InitGenesis(ctx sdk.Context, k keeper.Keeper, accountKeeper authkeeper.AccountKeeper, data types.GenesisState) {
	if err := k.SetParams(ctx, data.Params); err != nil {
		panic(fmt.Errorf("error setting params %s", err))
	}

	if acc := accountKeeper.GetModuleAccount(ctx, types.ModuleName); acc == nil {
		panic("the erc20 module account has not been set")
	}

	for _, pair := range data.TokenPairs {
		k.SetToken(ctx, pair)
	}

	for _, allowance := range data.Allowances {
		erc20 := common.HexToAddress(allowance.Erc20Address)
		owner := common.HexToAddress(allowance.Owner)
		spender := common.HexToAddress(allowance.Spender)
		if err := k.UnsafeSetAllowance(ctx, erc20, owner, spender, allowance.Value.BigInt()); err != nil {
			panic(fmt.Errorf("error setting allowance %s", err))
		}
	}
}

func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	return &types.GenesisState{
		Params:     k.GetParams(ctx),
		TokenPairs: k.GetTokenPairs(ctx),
		Allowances: k.GetAllowances(ctx),
	}
}
