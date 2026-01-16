package cli

import (
	"context"
	"fmt"
	"strings"

	cmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/spf13/cobra"

	evmostypes "github.com/evmos/evmos/v12/types"

	"github.com/evmos/evmos/v12/x/storage/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	// Group storage queries under a subcommand
	storageQueryCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	storageQueryCmd.AddCommand(
		CmdQueryParams(),
		CmdHeadBucket(),
		CmdHeadObject(),
		CmdListBuckets(),
		CmdListObjects(),
		CmdVerifyPermission(),
		CmdHeadGroup(),
		CmdListGroups(),
		CmdHeadGroupMember(),
		CmdQueryAccountPolicy(),
		CmdQueryGroupPolicy(),
	)

	return storageQueryCmd
}

func CmdHeadBucket() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "head-bucket [bucket-name]",
		Short: "Query bucket by bucket name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqBucketName := args[0]

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.HeadBucket(context.Background(), &types.QueryHeadBucketRequest{
				BucketName: reqBucketName,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdHeadObject() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "head-object [bucket-name] [object-name]",
		Short: "Query object by bucket-name and object-name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqBucketName := args[0]
			reqObjectName := args[1]

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.HeadObject(context.Background(), &types.QueryHeadObjectRequest{
				BucketName: reqBucketName,
				ObjectName: reqObjectName,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdListBuckets() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-buckets",
		Short: "Query all list buckets",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			clientCtx := client.GetClientContextFromCmd(cmd)
			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.ListBuckets(context.Background(), &types.QueryListBucketsRequest{
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdListObjects() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-objects [bucket-name]",
		Short: "Query list objects of the bucket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqBucketName := args[0]

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.ListObjects(context.Background(), &types.QueryListObjectsRequest{
				BucketName: reqBucketName,
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdVerifyPermission() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify-permission [operator] [bucket-name] [object-name] [action-type]",
		Short: "Query verify if the operator has permission for the bucket/object's action",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqOperator := args[0]
			reqBucketName := args[1]
			reqObjectName := args[2]
			reqActionType := args[3]

			actionType, err := GetActionType(reqActionType)
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.VerifyPermission(context.Background(), &types.QueryVerifyPermissionRequest{
				Operator:   reqOperator,
				BucketName: reqBucketName,
				ObjectName: reqObjectName,
				ActionType: actionType,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdHeadGroup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "head-group [group-owner] [group-name]",
		Short: "Query the group info",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqGroupOwner := args[0]
			reqGroupName := args[1]

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.HeadGroup(context.Background(), &types.QueryHeadGroupRequest{
				GroupOwner: reqGroupOwner,
				GroupName:  reqGroupName,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdListGroups() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-groups [group-owner]",
		Short: "Query list groups of owner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqGroupOwner := args[0]

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			pageReq, err := client.ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.ListGroups(context.Background(), &types.QueryListGroupsRequest{
				GroupOwner: reqGroupOwner,
				Pagination: pageReq,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdHeadGroupMember() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "head-group-member [group-owner] [group-name] [group-member]",
		Short: "Query the group member info",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			reqGroupOwner := args[0]
			reqGroupName := args[1]
			reqGroupMember := args[2]

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.HeadGroupMember(context.Background(), &types.QueryHeadGroupMemberRequest{
				Member:     reqGroupMember,
				GroupOwner: reqGroupOwner,
				GroupName:  reqGroupName,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdQueryAccountPolicy() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account-policy [grn] [principle-address]",
		Short: "Query the policy for a account that enforced on the resource",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query the polciy that a account has on the resource.

Examples:
 $ %s query %s account-policy grn:o::bucketName/objectName 0x....
	`, version.AppName, types.ModuleName),
		),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			grnStr := args[0]
			var grn evmostypes.GRN
			err = grn.ParseFromString(grnStr, false)
			if err != nil {
				return err
			}
			principalAcc, err := sdk.AccAddressFromHexUnsafe(args[1])
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.QueryPolicyForAccount(context.Background(), &types.QueryPolicyForAccountRequest{
				Resource:         grn.String(),
				PrincipalAddress: principalAcc.String(),
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func CmdQueryGroupPolicy() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-policy [grn] [principle-group-id]",
		Short: "Query the policy for a group that enforced on the resource",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Query the policy for a group that enforced on the resource

Examples:
 $ %s query %s group-policy grn:o::bucketName/objectName 1
	`, version.AppName, types.ModuleName),
		),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			grnStr := args[0]
			var grn evmostypes.GRN
			err = grn.ParseFromString(grnStr, false)
			if err != nil {
				return err
			}
			groupID, ok := cmath.NewIntFromString(args[1])
			if !ok {
				return fmt.Errorf("failed to convert group id")
			}
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.QueryPolicyForGroup(context.Background(), &types.QueryPolicyForGroupRequest{
				Resource:         grn.String(),
				PrincipalGroupId: groupID.String(),
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
