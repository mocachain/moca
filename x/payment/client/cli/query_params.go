package cli

import (
	"context"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	evmostypes "github.com/evmos/evmos/v12/types"
	"github.com/evmos/evmos/v12/x/evm/precompiles/payment"
	"github.com/evmos/evmos/v12/x/payment/types"
)

func CmdEvmQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "shows the parameters of the module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			contract, err := payment.NewIPayment(common.HexToAddress(evmostypes.PaymentAddress), clientCtx.EvmClient)
			if err != nil {
				return err
			}
			result, err := contract.Params(&bind.CallOpts{})
			if err != nil {
				return err
			}
			withdrawTimeLockThreshold := math.NewIntFromBigInt(result.WithdrawTimeLockThreshold)
			res := &types.QueryParamsResponse{
				Params: types.Params{
					VersionedParams: types.VersionedParams{
						ReserveTime:      result.VersionedParams.ReserveTime,
						ValidatorTaxRate: math.LegacyNewDecFromBigInt(result.VersionedParams.ValidatorTaxRate),
					},
					PaymentAccountCountLimit:  result.PaymentAccountCountLimit,
					ForcedSettleTime:          result.ForcedSettleTime,
					MaxAutoSettleFlowCount:    result.MaxAutoSettleFlowCount,
					MaxAutoResumeFlowCount:    result.MaxAutoResumeFlowCount,
					FeeDenom:                  result.FeeDenom,
					WithdrawTimeLockThreshold: &withdrawTimeLockThreshold,
					WithdrawTimeLockDuration:  result.WithdrawTimeLockDuration,
				}}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "shows the parameters of the module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(context.Background(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
