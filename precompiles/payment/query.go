package payment

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"

	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

const (
	// PaymentAccountsByOwnerMethodName is the ABI name for the paymentAccountsByOwner query.
	PaymentAccountsByOwnerMethodName = "paymentAccountsByOwner"
	// PaymentAccountMethodName is the ABI name for the paymentAccount query.
	PaymentAccountMethodName = "paymentAccount"
	// ParamsMethodName is the ABI name for the params query.
	ParamsMethodName = "params"
	// ParamsByTimestampMethodName is the ABI name for the paramsByTimestamp query.
	ParamsByTimestampMethodName = "paramsByTimestamp"
	// OutFlowsMethodName is the ABI name for the outFlows query.
	OutFlowsMethodName = "outFlows"
	// StreamRecordMethodName is the ABI name for the streamRecord query.
	StreamRecordMethodName = "streamRecord"
	// StreamRecordsMethodName is the ABI name for the streamRecords query.
	StreamRecordsMethodName = "streamRecords"
	// PaymentAccountCountMethodName is the ABI name for the paymentAccountCount query.
	PaymentAccountCountMethodName = "paymentAccountCount"
	// PaymentAccountCountsMethodName is the ABI name for the paymentAccountCounts query.
	PaymentAccountCountsMethodName = "paymentAccountCounts"
	// PaymentAccountsMethodName is the ABI name for the paymentAccounts query.
	PaymentAccountsMethodName = "paymentAccounts"
	// DynamicBalanceMethodName is the ABI name for the dynamicBalance query.
	DynamicBalanceMethodName = "dynamicBalance"
	// AutoSettleRecordsMethodName is the ABI name for the autoSettleRecords query.
	AutoSettleRecordsMethodName = "autoSettleRecords"
	// DelayedWithdrawalMethodName is the ABI name for the delayedWithdrawal query.
	DelayedWithdrawalMethodName = "delayedWithdrawal"
)

// PaymentAccountsByOwner queries all payment accounts by an owner.
func (p Precompile) PaymentAccountsByOwner(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input PaymentAccountsByOwnerArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.PaymentAccountsByOwner(ctx, &paymenttypes.QueryPaymentAccountsByOwnerRequest{
		Owner: input.Owner,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.PaymentAccounts)
}

// PaymentAccount queries a payment account by payment account address.
func (p Precompile) PaymentAccount(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input PaymentAccountArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.PaymentAccount(ctx, &paymenttypes.QueryPaymentAccountRequest{
		Addr: input.Addr,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(PaymentAccount{
		Addr:       res.PaymentAccount.Addr,
		Owner:      res.PaymentAccount.Owner,
		Refundable: res.PaymentAccount.Refundable,
	})
}

// Params queries the parameters of the payment module.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.paymentKeeper.Params(ctx, &paymenttypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputsParams(res.Params))
}

// ParamsByTimestamp queries the parameters of the payment module by timestamp.
func (p Precompile) ParamsByTimestamp(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input ParamsByTimestampArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.ParamsByTimestamp(ctx, &paymenttypes.QueryParamsByTimestampRequest{
		Timestamp: input.Timestamp,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputsParams(res.Params))
}

// OutFlows queries out flows by account.
func (p Precompile) OutFlows(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input OutFlowsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.OutFlows(ctx, &paymenttypes.QueryOutFlowsRequest{
		Account: input.Account,
	})
	if err != nil {
		return nil, err
	}

	outFlows := make([]OutFlow, 0)
	for _, outFlow := range res.OutFlows {
		outFlows = append(outFlows, OutFlow{ToAddress: outFlow.ToAddress, Rate: outFlow.Rate.BigInt(), Status: int32(outFlow.Status)})
	}

	return method.Outputs.Pack(outFlows)
}

// StreamRecord queries a stream record by account.
func (p Precompile) StreamRecord(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input StreamRecordArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.StreamRecord(ctx, &paymenttypes.QueryGetStreamRecordRequest{
		Account: input.Account,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputsStreamRecord(res.StreamRecord))
}

// StreamRecords queries all stream records.
func (p Precompile) StreamRecords(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input StreamRecordsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.StreamRecords(ctx, &paymenttypes.QueryStreamRecordsRequest{
		Pagination: pageRequest(input.Pagination),
	})
	if err != nil {
		return nil, err
	}

	streamRecords := make([]StreamRecord, 0, len(res.StreamRecords))
	for _, streamRecord := range res.StreamRecords {
		streamRecords = append(streamRecords, outputsStreamRecord(streamRecord))
	}

	return method.Outputs.Pack(streamRecords, pageResponse(res.Pagination))
}

// PaymentAccountCount queries the count of payment accounts by owner.
func (p Precompile) PaymentAccountCount(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input PaymentAccountCountArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.PaymentAccountCount(ctx, &paymenttypes.QueryPaymentAccountCountRequest{
		Owner: input.Owner,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(PaymentAccountCount{
		Owner: res.PaymentAccountCount.Owner,
		Count: res.PaymentAccountCount.Count,
	})
}

// PaymentAccountCounts queries all counts of payment accounts for all owners.
func (p Precompile) PaymentAccountCounts(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input PaymentAccountCountsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.PaymentAccountCounts(ctx, &paymenttypes.QueryPaymentAccountCountsRequest{
		Pagination: pageRequest(input.Pagination),
	})
	if err != nil {
		return nil, err
	}

	paymentAccountCounts := make([]PaymentAccountCount, 0, len(res.PaymentAccountCounts))
	for _, paymentAccountCount := range res.PaymentAccountCounts {
		paymentAccountCounts = append(paymentAccountCounts, PaymentAccountCount{
			Owner: paymentAccountCount.Owner,
			Count: paymentAccountCount.Count,
		})
	}

	return method.Outputs.Pack(paymentAccountCounts, pageResponse(res.Pagination))
}

// PaymentAccounts queries all payment accounts.
func (p Precompile) PaymentAccounts(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input PaymentAccountsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.PaymentAccounts(ctx, &paymenttypes.QueryPaymentAccountsRequest{
		Pagination: pageRequest(input.Pagination),
	})
	if err != nil {
		return nil, err
	}

	paymentAccounts := make([]PaymentAccount, 0, len(res.PaymentAccounts))
	for _, paymentAccount := range res.PaymentAccounts {
		paymentAccounts = append(paymentAccounts, PaymentAccount{
			Addr:       paymentAccount.Addr,
			Owner:      paymentAccount.Owner,
			Refundable: paymentAccount.Refundable,
		})
	}

	return method.Outputs.Pack(paymentAccounts, pageResponse(res.Pagination))
}

// DynamicBalance queries the dynamic balance of a payment account.
func (p Precompile) DynamicBalance(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DynamicBalanceArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.DynamicBalance(ctx, &paymenttypes.QueryDynamicBalanceRequest{
		Account: input.Account,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.DynamicBalance.BigInt())
}

// AutoSettleRecords queries all auto settle records.
func (p Precompile) AutoSettleRecords(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input AutoSettleRecordsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.AutoSettleRecords(ctx, &paymenttypes.QueryAutoSettleRecordsRequest{
		Pagination: pageRequest(input.Pagination),
	})
	if err != nil {
		return nil, err
	}

	autoSettleRecords := make([]AutoSettleRecord, 0, len(res.AutoSettleRecords))
	for _, autoSettleRecord := range res.AutoSettleRecords {
		autoSettleRecords = append(autoSettleRecords, AutoSettleRecord{
			Timestamp: autoSettleRecord.Timestamp,
			Addr:      autoSettleRecord.Addr,
		})
	}

	return method.Outputs.Pack(autoSettleRecords, pageResponse(res.Pagination))
}

// DelayedWithdrawal queries the delayed withdrawal of an account.
func (p Precompile) DelayedWithdrawal(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DelayedWithdrawalArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.paymentKeeper.DelayedWithdrawal(ctx, &paymenttypes.QueryDelayedWithdrawalRequest{
		Account: input.Account,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(DelayedWithdrawalRecord{
		Addr:            res.DelayedWithdrawal.Addr,
		Amount:          res.DelayedWithdrawal.Amount.BigInt(),
		From:            res.DelayedWithdrawal.From,
		UnlockTimestamp: res.DelayedWithdrawal.UnlockTimestamp,
	})
}

// outputsParams maps payment module params into the ABI tuple.
func outputsParams(params paymenttypes.Params) Params {
	return Params{
		VersionedParams: VersionedParams{
			ReserveTime:      params.VersionedParams.ReserveTime,
			ValidatorTaxRate: params.VersionedParams.ValidatorTaxRate.BigInt(),
		},
		PaymentAccountCountLimit:  params.PaymentAccountCountLimit,
		ForcedSettleTime:          params.ForcedSettleTime,
		MaxAutoSettleFlowCount:    params.MaxAutoSettleFlowCount,
		MaxAutoResumeFlowCount:    params.MaxAutoResumeFlowCount,
		FeeDenom:                  params.FeeDenom,
		WithdrawTimeLockThreshold: params.WithdrawTimeLockThreshold.BigInt(),
		WithdrawTimeLockDuration:  params.WithdrawTimeLockDuration,
	}
}

// outputsStreamRecord maps a payment stream record into the ABI tuple.
func outputsStreamRecord(streamRecord paymenttypes.StreamRecord) StreamRecord {
	return StreamRecord{
		Account:           streamRecord.Account,
		CrudTimestamp:     streamRecord.CrudTimestamp,
		NetflowRate:       streamRecord.NetflowRate.BigInt(),
		StaticBalance:     streamRecord.StaticBalance.BigInt(),
		BufferBalance:     streamRecord.BufferBalance.BigInt(),
		LockBalance:       streamRecord.LockBalance.BigInt(),
		Status:            int32(streamRecord.Status),
		SettleTimestamp:   streamRecord.SettleTimestamp,
		OutFlowCount:      streamRecord.OutFlowCount,
		FrozenNetflowRate: streamRecord.FrozenNetflowRate.BigInt(),
	}
}
