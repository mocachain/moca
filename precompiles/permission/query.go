package permission

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"

	permissiontypes "github.com/mocachain/moca/v2/x/permission/types"
)

const (
	// ParamsMethodName is the ABI name for the Params query.
	ParamsMethodName = "params"
)

// Params queries the parameters of the x/permission module.
func (p Precompile) Params(ctx sdk.Context, method *abi.Method, _ []interface{}) ([]byte, error) {
	res, err := p.permissionQuerier.Params(ctx, &permissiontypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(Params{
		MaximumStatementsNum:                  res.Params.MaximumStatementsNum,
		MaximumGroupNum:                       res.Params.MaximumGroupNum,
		MaximumRemoveExpiredPoliciesIteration: res.Params.MaximumRemoveExpiredPoliciesIteration,
	})
}
