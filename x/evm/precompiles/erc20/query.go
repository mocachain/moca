package erc20

import (
	"bytes"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	erc20types "github.com/evmos/evmos/v12/x/erc20/types"
	"github.com/evmos/evmos/v12/x/evm/types"
)

const (
	TokenPairsGas = 50_000
	TokenPairGas  = 30_000
	ParamsGas     = 30_000

	TokenPairsMethodName = "tokenPairs"
	TokenPairMethodName  = "tokenPair"
	ParamsMethodName     = "params"
)

// TokenPairs queries registered token pairs with pagination
func (c *Contract) TokenPairs(ctx sdk.Context, _ *vm.EVM, contract *vm.Contract, _ bool) ([]byte, error) {
	method := MustMethod(TokenPairsMethodName)

	var args TokenPairsArgs
	if err := types.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
		return nil, err
	}

	if bytes.Equal(args.PageRequest.Key, []byte{0}) {
		args.PageRequest.Key = nil
	}

	msg := &erc20types.QueryTokenPairsRequest{
		Pagination: &query.PageRequest{
			Key:        args.PageRequest.Key,
			Offset:     args.PageRequest.Offset,
			Limit:      args.PageRequest.Limit,
			CountTotal: args.PageRequest.CountTotal,
			Reverse:    args.PageRequest.Reverse,
		},
	}

	res, err := c.erc20Keeper.TokenPairs(ctx, msg)
	if err != nil {
		return nil, err
	}

	var tokenPairs []TokenPair
	for _, pair := range res.TokenPairs {
		tokenPairs = append(tokenPairs, TokenPair{
			Erc20Address:  common.HexToAddress(pair.Erc20Address),
			Denom:         pair.Denom,
			Enabled:       pair.Enabled,
			ContractOwner: uint8(pair.ContractOwner),
		})
	}

	var pageResponse PageResponse
	pageResponse.NextKey = res.Pagination.NextKey
	pageResponse.Total = res.Pagination.Total

	return method.Outputs.Pack(tokenPairs, pageResponse)
}

// TokenPair queries a specific registered token pair
func (c *Contract) TokenPair(ctx sdk.Context, _ *vm.EVM, contract *vm.Contract, _ bool) ([]byte, error) {
	method := MustMethod(TokenPairMethodName)

	var args TokenPairArgs
	if err := types.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
		return nil, err
	}

	msg := &erc20types.QueryTokenPairRequest{
		Token: args.Token.String(),
	}

	res, err := c.erc20Keeper.TokenPair(ctx, msg)
	if err != nil {
		return nil, err
	}

	tokenPair := TokenPair{
		Erc20Address:  common.HexToAddress(res.TokenPair.Erc20Address),
		Denom:         res.TokenPair.Denom,
		Enabled:       res.TokenPair.Enabled,
		ContractOwner: uint8(res.TokenPair.ContractOwner),
	}

	return method.Outputs.Pack(tokenPair)
}

// Params queries the erc20 module parameters
func (c *Contract) Params(ctx sdk.Context, _ *vm.EVM, contract *vm.Contract, _ bool) ([]byte, error) {
	method := MustMethod(ParamsMethodName)

	msg := &erc20types.QueryParamsRequest{}

	res, err := c.erc20Keeper.Params(ctx, msg)
	if err != nil {
		return nil, err
	}

	params := Params{
		EnableErc20:   res.Params.EnableErc20,
		EnableEvmHook: res.Params.EnableEVMHook,
	}

	return method.Outputs.Pack(params)
}
