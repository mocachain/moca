package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func (k Keeper) SettleAndDistributeGVGFamily(ctx sdk.Context, sp *sptypes.StorageProvider, family *types.GlobalVirtualGroupFamily) error {
	paymentAddress := sdk.MustAccAddressFromHex(family.GetVirtualPaymentAddress())
	totalBalance, err := k.paymentKeeper.QueryDynamicBalance(ctx, paymentAddress)
	if err != nil {
		return fmt.Errorf("fail to query balance: %s, err: %s", paymentAddress.String(), err.Error())
	}
	if !totalBalance.IsPositive() {
		return nil
	}

	err = k.paymentKeeper.Withdraw(ctx, paymentAddress, sdk.MustAccAddressFromHex(sp.FundingAddress), totalBalance)
	if err != nil {
		return fmt.Errorf("fail to send coins: %s %s", paymentAddress, sp.FundingAddress)
	}

	err = ctx.EventManager().EmitTypedEvent(&types.EventSettleGlobalVirtualGroupFamily{
		Id:               family.Id,
		SpId:             sp.Id,
		SpFundingAddress: sp.FundingAddress,
		Amount:           totalBalance,
	})
	if err != nil {
		ctx.Logger().Error("fail to send event for settlement", "vfg", family.Id, "err", err)
	}

	return nil
}

func (k Keeper) SettleAndDistributeGVG(ctx sdk.Context, gvg *types.GlobalVirtualGroup) error {
	paymentAddress := sdk.MustAccAddressFromHex(gvg.GetVirtualPaymentAddress())
	totalBalance, err := k.paymentKeeper.QueryDynamicBalance(ctx, paymentAddress)
	if err != nil {
		return fmt.Errorf("fail to query balance: %s, err: %s", paymentAddress.String(), err.Error())
	}

	n := int64(len(gvg.SecondarySpIds))
	// A negative dynamic balance on an income account is an invariant violation;
	// surface it instead of silently skipping distribution.
	if totalBalance.IsNegative() {
		return fmt.Errorf("gvg %d has a negative virtual payment balance: %s", gvg.Id, totalBalance.String())
	}
	if totalBalance.IsZero() {
		return nil
	}
	// A positive balance with no secondary SPs cannot be distributed; erroring
	// prevents it from being stranded on delete.
	if n == 0 {
		return fmt.Errorf("gvg %d has balance %s but no secondary sp to distribute to", gvg.Id, totalBalance.String())
	}

	// Distribute the entire balance, spreading the division remainder round-robin
	// across the first SPs, so nothing is left stranded in the virtual payment
	// account (a plain QuoRaw would leave `totalBalance mod n` behind).
	base := totalBalance.QuoRaw(n)
	remainder := totalBalance.Sub(base.MulRaw(n)).Int64()

	fundingAddresses := make([]string, 0)
	for i, spID := range gvg.SecondarySpIds {
		sp, found := k.spKeeper.GetStorageProvider(ctx, spID)
		if !found {
			return fmt.Errorf("fail to find secondary sp: %d", spID)
		}
		amount := base
		if int64(i) < remainder {
			amount = amount.AddRaw(1) // round-robin distribution of dust (total mod n)
		}
		if amount.IsPositive() {
			err = k.paymentKeeper.Withdraw(ctx, paymentAddress, sdk.MustAccAddressFromHex(sp.FundingAddress), amount)
			if err != nil {
				return fmt.Errorf("fail to send coins: %s %s", paymentAddress, sp.FundingAddress)
			}
		}

		fundingAddresses = append(fundingAddresses, sp.FundingAddress)
	}

	err = ctx.EventManager().EmitTypedEvent(&types.EventSettleGlobalVirtualGroup{
		Id:                 gvg.Id,
		SpIds:              gvg.SecondarySpIds,
		SpFundingAddresses: fundingAddresses,
		Amount:             base,
	})
	if err != nil {
		ctx.Logger().Error("fail to send event for settlement", "gvg", gvg.Id, "err", err)
	}

	return nil
}
