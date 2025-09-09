package cli

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	evmostypes "github.com/evmos/evmos/v12/types"
	"github.com/evmos/evmos/v12/x/evm/precompiles/permission"
	"github.com/evmos/evmos/v12/x/permission/types"
)

func CmdEvmQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "shows the parameters of the module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			contract, err := permission.NewIPermission(common.HexToAddress(evmostypes.PermissionAddress), clientCtx.EvmClient)
			if err != nil {
				return err
			}
			result, err := contract.Params(&bind.CallOpts{})
			if err != nil {
				return err
			}
			res := &types.QueryParamsResponse{
				Params: types.Params{
					MaximumStatementsNum:                  result.MaximumStatementsNum,
					MaximumGroupNum:                       result.MaximumGroupNum,
					MaximumRemoveExpiredPoliciesIteration: result.MaximumRemoveExpiredPoliciesIteration,
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
