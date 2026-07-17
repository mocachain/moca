package bank

import (
	"bytes"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/mocachain/moca/v2/utils"
)

const (
	// BalanceMethodName is the ABI name for the Balance query.
	BalanceMethodName = "balance"
	// AllBalancesMethodName is the ABI name for the AllBalances query.
	AllBalancesMethodName = "allBalances"
	// TotalSupplyMethodName is the ABI name for the TotalSupply query.
	TotalSupplyMethodName = "totalSupply"
	// SpendableBalancesMethodName is the ABI name for the SpendableBalances query.
	SpendableBalancesMethodName = "spendableBalances"
	// SpendableBalanceByDenomMethodName is the ABI name for the SpendableBalanceByDenom query.
	SpendableBalanceByDenomMethodName = "spendableBalanceByDenom"
	// SupplyOfMethodName is the ABI name for the SupplyOf query.
	SupplyOfMethodName = "supplyOf"
	// ParamsMethodName is the ABI name for the Params query.
	ParamsMethodName = "params"
	// DenomMetadataMethodName is the ABI name for the DenomMetadata query.
	DenomMetadataMethodName = "denomMetadata"
	// DenomsMetadataMethodName is the ABI name for the DenomsMetadata query.
	DenomsMetadataMethodName = "denomsMetadata"
	// DenomOwnersMethodName is the ABI name for the DenomOwners query.
	DenomOwnersMethodName = "denomOwners"
	// SendEnabledMethodName is the ABI name for the SendEnabled query.
	SendEnabledMethodName = "sendEnabled"
)

// Balance queries the balance of a single coin for a single account.
func (p Precompile) Balance(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input BalanceArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: input.AccountAddress.String(),
		Denom:   input.Denom,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(Coin{
		Denom:  res.Balance.Denom,
		Amount: res.Balance.Amount.BigInt(),
	})
}

// AllBalances queries the balance of all coins for a single account.
func (p Precompile) AllBalances(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input AllBalancesArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.AllBalances(ctx, &banktypes.QueryAllBalancesRequest{
		Address:    input.AccountAddress.String(),
		Pagination: pageRequest(input.PageRequest),
	})
	if err != nil {
		return nil, err
	}

	balances := make([]Coin, 0, len(res.Balances))
	for _, balance := range res.Balances {
		balances = append(balances, Coin{Denom: balance.Denom, Amount: balance.Amount.BigInt()})
	}

	return method.Outputs.Pack(balances, pageResponse(res.Pagination))
}

// TotalSupply queries the total supply of all coins.
func (p Precompile) TotalSupply(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input TotalSupplyArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.TotalSupply(ctx, &banktypes.QueryTotalSupplyRequest{
		Pagination: pageRequest(input.PageRequest),
	})
	if err != nil {
		return nil, err
	}

	balances := make([]Coin, 0, len(res.Supply))
	for _, balance := range res.Supply {
		balances = append(balances, Coin{Denom: balance.Denom, Amount: balance.Amount.BigInt()})
	}

	return method.Outputs.Pack(balances, pageResponse(res.Pagination))
}

// SpendableBalanceByDenom queries an account's spendable balance for a specific denom.
func (p Precompile) SpendableBalanceByDenom(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input SpendableBalanceByDenomArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.SpendableBalanceByDenom(ctx, &banktypes.QuerySpendableBalanceByDenomRequest{
		Address: input.AccountAddress.String(),
		Denom:   input.Denom,
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(Coin{
		Denom:  res.Balance.Denom,
		Amount: res.Balance.Amount.BigInt(),
	})
}

// SpendableBalances queries the spendable balance of all coins for a single account.
func (p Precompile) SpendableBalances(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input SpendableBalancesArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.SpendableBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
		Address:    input.AccountAddress.String(),
		Pagination: pageRequest(input.PageRequest),
	})
	if err != nil {
		return nil, err
	}

	balances := make([]Coin, 0, len(res.Balances))
	for _, balance := range res.Balances {
		balances = append(balances, Coin{Denom: balance.Denom, Amount: balance.Amount.BigInt()})
	}

	return method.Outputs.Pack(balances, pageResponse(res.Pagination))
}

// SupplyOf queries the supply of a single coin.
func (p Precompile) SupplyOf(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input SupplyOfArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.SupplyOf(ctx, &banktypes.QuerySupplyOfRequest{Denom: input.Denom})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(Coin{
		Denom:  res.Amount.Denom,
		Amount: res.Amount.Amount.BigInt(),
	})
}

// Params queries the parameters of the x/bank module.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.bankKeeper.Params(ctx, &banktypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	sendEnableds := make([]SendEnabled, 0, len(res.Params.SendEnabled)) //nolint:staticcheck // deprecated field returned for ABI compatibility
	for _, sendEnabled := range res.Params.SendEnabled {                //nolint:staticcheck // deprecated field returned for ABI compatibility
		sendEnableds = append(sendEnableds, SendEnabled{Denom: sendEnabled.Denom, Enabled: sendEnabled.Enabled})
	}

	return method.Outputs.Pack(Params{
		SendEnabled:        sendEnableds,
		DefaultSendEnabled: res.Params.DefaultSendEnabled,
	})
}

// DenomMetadata queries the client metadata of a given coin denomination.
func (p Precompile) DenomMetadata(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DenomMetadataArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.DenomMetadata(ctx, &banktypes.QueryDenomMetadataRequest{Denom: input.Denom})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(outputsMetadata(res.Metadata))
}

// DenomsMetadata queries the client metadata for all registered coin denominations.
func (p Precompile) DenomsMetadata(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DenomsMetadataArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.DenomsMetadata(ctx, &banktypes.QueryDenomsMetadataRequest{
		Pagination: pageRequest(input.PageRequest),
	})
	if err != nil {
		return nil, err
	}

	metaDatas := make([]Metadata, 0, len(res.Metadatas))
	for _, metaData := range res.Metadatas {
		metaDatas = append(metaDatas, outputsMetadata(metaData))
	}

	return method.Outputs.Pack(metaDatas, pageResponse(res.Pagination))
}

// DenomOwners queries for all account addresses that own a particular token denomination.
func (p Precompile) DenomOwners(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input DenomOwnersArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.DenomOwners(ctx, &banktypes.QueryDenomOwnersRequest{
		Denom:      input.Denom,
		Pagination: pageRequest(input.PageRequest),
	})
	if err != nil {
		return nil, err
	}

	denomOwners := make([]DenomOwner, 0, len(res.DenomOwners))
	for _, denomOwner := range res.DenomOwners {
		denomOwners = append(denomOwners, DenomOwner{
			AccountAddress: utils.AccAddressMustToHexAddress(denomOwner.Address),
			Balance: Coin{
				Denom:  denomOwner.Balance.Denom,
				Amount: denomOwner.Balance.Amount.BigInt(),
			},
		})
	}

	return method.Outputs.Pack(denomOwners, pageResponse(res.Pagination))
}

// SendEnabled queries for SendEnabled entries.
func (p Precompile) SendEnabled(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input SendEnabledArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	res, err := p.bankKeeper.SendEnabled(ctx, &banktypes.QuerySendEnabledRequest{
		Denoms:     input.Denoms,
		Pagination: pageRequest(input.PageRequest),
	})
	if err != nil {
		return nil, err
	}

	sendEnableds := make([]SendEnabled, 0, len(res.SendEnabled))
	for _, sendEnabled := range res.SendEnabled {
		sendEnableds = append(sendEnableds, SendEnabled{Denom: sendEnabled.Denom, Enabled: sendEnabled.Enabled})
	}

	return method.Outputs.Pack(sendEnableds, pageResponse(res.Pagination))
}

// pageRequest builds a query.PageRequest from the ABI pagination tuple, treating a
// single zero byte key as empty.
func pageRequest(page PageRequest) *query.PageRequest {
	key := page.Key
	if bytes.Equal(key, []byte{0}) {
		key = nil
	}
	return &query.PageRequest{
		Key:        key,
		Offset:     page.Offset,
		Limit:      page.Limit,
		CountTotal: page.CountTotal,
		Reverse:    page.Reverse,
	}
}

func pageResponse(res *query.PageResponse) PageResponse {
	return PageResponse{NextKey: res.NextKey, Total: res.Total}
}

// outputsMetadata maps bank denom metadata into the ABI tuple.
func outputsMetadata(metaData banktypes.Metadata) Metadata {
	denomUnits := make([]DenomUnit, 0, len(metaData.DenomUnits))
	for _, denomUnit := range metaData.DenomUnits {
		denomUnits = append(denomUnits, DenomUnit{
			Denom:    denomUnit.Denom,
			Exponent: denomUnit.Exponent,
			Aliases:  denomUnit.Aliases,
		})
	}

	return Metadata{
		Description: metaData.Description,
		DenomUnits:  denomUnits,
		Base:        metaData.Base,
		Display:     metaData.Display,
		Name:        metaData.Name,
		Symbol:      metaData.Symbol,
		Uri:         metaData.URI,
		UriHash:     metaData.URIHash,
	}
}
