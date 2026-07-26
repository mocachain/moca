package slashing

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

const (
	// SigningInfoMethod is the ABI name for the SigningInfo query.
	SigningInfoMethod = "signingInfo"
	// SigningInfosMethod is the ABI name for the SigningInfos query.
	SigningInfosMethod = "signingInfos"
	// ParamsMethod is the ABI name for the Params query.
	ParamsMethod = "params"
)

// SigningInfo queries the signing info of a single validator by consensus address.
func (p Precompile) SigningInfo(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	req, err := NewSigningInfoRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.slashingQuerier.SigningInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(newValidatorSigningInfo(res.ValSigningInfo))
}

// SigningInfos queries the signing info of all validators, paginated.
func (p Precompile) SigningInfos(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	req, err := NewSigningInfosRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.slashingQuerier.SigningInfos(ctx, req)
	if err != nil {
		return nil, err
	}

	infos := make([]ValidatorSigningInfo, 0, len(res.Info))
	for _, info := range res.Info {
		infos = append(infos, newValidatorSigningInfo(info))
	}

	pageResponse := PageResponse{
		NextKey: res.Pagination.NextKey,
		Total:   res.Pagination.Total,
	}

	return method.Outputs.Pack(infos, pageResponse)
}

// Params queries the slashing module parameters.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.slashingQuerier.Params(ctx, &slashingtypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	params := Params{
		SignedBlocksWindow:      res.Params.SignedBlocksWindow,
		MinSignedPerWindow:      res.Params.MinSignedPerWindow.String(),
		DowntimeJailDuration:    int64(res.Params.DowntimeJailDuration.Seconds()),
		SlashFractionDoubleSign: res.Params.SlashFractionDoubleSign.String(),
		SlashFractionDowntime:   res.Params.SlashFractionDowntime.String(),
	}

	return method.Outputs.Pack(params)
}
