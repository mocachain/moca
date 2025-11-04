package cli

import (
	"fmt"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/spf13/cobra"

	"github.com/evmos/evmos/v12/x/evm/precompiles/payment"
	"github.com/evmos/evmos/v12/x/payment/types"
)

func ToPaymentPageReq(in *query.PageRequest) *payment.PageRequest {
	if in == nil {
		return nil
	}
	return &payment.PageRequest{
		Key:        in.Key,
		Offset:     in.Offset,
		Limit:      in.Limit,
		CountTotal: in.CountTotal,
		Reverse:    in.Reverse,
	}
}

func ToPageResp(p *payment.PageResponse) *query.PageResponse {
	if p == nil {
		return nil
	}
	return &query.PageResponse{
		NextKey: p.NextKey,
		Total:   p.Total,
	}
}

func ToStreamRecord(p *payment.StreamRecord) *types.StreamRecord {
	if p == nil {
		return nil
	}
	s := &types.StreamRecord{
		Account:           p.Account,
		CrudTimestamp:     p.CrudTimestamp,
		NetflowRate:       math.NewIntFromBigInt(p.NetflowRate),
		StaticBalance:     math.NewIntFromBigInt(p.StaticBalance),
		BufferBalance:     math.NewIntFromBigInt(p.BufferBalance),
		LockBalance:       math.NewIntFromBigInt(p.LockBalance),
		Status:            types.StreamAccountStatus(p.Status),
		SettleTimestamp:   p.SettleTimestamp,
		OutFlowCount:      p.OutFlowCount,
		FrozenNetflowRate: math.NewIntFromBigInt(p.FrozenNetflowRate),
	}
	return s
}

func ToPaymentAccount(p *payment.PaymentAccount) *types.PaymentAccount {
	if p == nil {
		return nil
	}
	s := &types.PaymentAccount{
		Addr:       p.Addr,
		Owner:      p.Owner,
		Refundable: p.Refundable,
	}
	return s
}

func ToPaymentAccountCount(p *payment.PaymentAccountCount) *types.PaymentAccountCount {
	if p == nil {
		return nil
	}
	s := &types.PaymentAccountCount{
		Owner: p.Owner,
		Count: p.Count,
	}
	return s
}

func ToAutoSettleRecord(p *payment.AutoSettleRecord) *types.AutoSettleRecord {
	if p == nil {
		return nil
	}
	s := &types.AutoSettleRecord{
		Timestamp: p.Timestamp,
		Addr:      p.Addr,
	}
	return s
}

// GetQueryCmd returns the cli query commands for this module
func GetEvmQueryCmd() *cobra.Command {
	// Group payment queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdEvmQueryParams())
	cmd.AddCommand(CmdEvmListStreamRecord())
	cmd.AddCommand(CmdEvmShowStreamRecord())
	cmd.AddCommand(CmdEvmListPaymentAccountCount())
	cmd.AddCommand(CmdEvmShowPaymentAccountCount())
	cmd.AddCommand(CmdEvmListPaymentAccount())
	cmd.AddCommand(CmdEvmShowPaymentAccount())
	cmd.AddCommand(CmdEvmDynamicBalance())
	cmd.AddCommand(CmdEvmGetPaymentAccountsByOwner())
	cmd.AddCommand(CmdEvmListAutoSettleRecord())

	return cmd
}

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd() *cobra.Command {
	// Group payment queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdQueryParams())
	cmd.AddCommand(CmdListStreamRecord())
	cmd.AddCommand(CmdShowStreamRecord())
	cmd.AddCommand(CmdListPaymentAccountCount())
	cmd.AddCommand(CmdShowPaymentAccountCount())
	cmd.AddCommand(CmdListPaymentAccount())
	cmd.AddCommand(CmdShowPaymentAccount())
	cmd.AddCommand(CmdDynamicBalance())
	cmd.AddCommand(CmdGetPaymentAccountsByOwner())
	cmd.AddCommand(CmdListAutoSettleRecord())

	return cmd
}
