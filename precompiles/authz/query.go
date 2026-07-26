package authz

import (
	"bytes"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/accounts/abi"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"

	"github.com/mocachain/moca/v2/precompiles/types"
	"github.com/mocachain/moca/v2/utils"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

const (
	// GrantsMethodName is the ABI name for the Grants query.
	GrantsMethodName = "grants"
	// GranterGrantsMethodName is the ABI name for the GranterGrants query.
	GranterGrantsMethodName = "granterGrants"
	// GranteeGrantsMethodName is the ABI name for the GranteeGrants query.
	GranteeGrantsMethodName = "granteeGrants"
)

// Grants returns list of `Authorization`, granted to the grantee by the granter.
func (p Precompile) Grants(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input GrantsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}

	msg := &authztypes.QueryGrantsRequest{
		Granter:    input.Granter.String(),
		Grantee:    input.Grantee.String(),
		MsgTypeUrl: input.MsgTypeURL,
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	}

	res, err := p.authzKeeper.Grants(ctx, msg)
	if err != nil {
		return nil, err
	}

	grants := make([]GrantData, 0, len(res.Grants))
	for _, grant := range res.Grants {
		var expiration int64
		if grant.Expiration != nil {
			expiration = grant.Expiration.Unix()
		}
		grants = append(grants, GrantData{
			Authorization: OutputsAuthorization(grant.Authorization.GetCachedValue().(authztypes.Authorization)),
			Expiration:    expiration,
		})
	}

	var pageResponse PageResponse
	pageResponse.NextKey = res.Pagination.NextKey
	pageResponse.Total = res.Pagination.Total

	return method.Outputs.Pack(grants, pageResponse)
}

// GranterGrants returns list of `GrantAuthorization`, granted by granter.
func (p Precompile) GranterGrants(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input GranterGrantsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}

	msg := &authztypes.QueryGranterGrantsRequest{
		Granter: input.Granter.String(),
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	}

	res, err := p.authzKeeper.GranterGrants(ctx, msg)
	if err != nil {
		return nil, err
	}

	grants := make([]GrantAuthorization, 0, len(res.Grants))
	for _, grant := range res.Grants {
		var expiration int64
		if grant.Expiration != nil {
			expiration = grant.Expiration.Unix()
		}

		grants = append(grants, GrantAuthorization{
			Granter:       utils.AccAddressMustToHexAddress(grant.Granter),
			Grantee:       utils.AccAddressMustToHexAddress(grant.Grantee),
			Authorization: OutputsAuthorization(grant.Authorization.GetCachedValue().(authztypes.Authorization)),
			Expiration:    expiration,
		})
	}

	var pageResponse PageResponse
	pageResponse.NextKey = res.Pagination.NextKey
	pageResponse.Total = res.Pagination.Total

	return method.Outputs.Pack(grants, pageResponse)
}

// GranteeGrants returns a list of `GrantAuthorization` by grantee.
func (p Precompile) GranteeGrants(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	var input GranteeGrantsArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	if bytes.Equal(input.Pagination.Key, []byte{0}) {
		input.Pagination.Key = nil
	}

	msg := &authztypes.QueryGranteeGrantsRequest{
		Grantee: input.Grantee.String(),
		Pagination: &query.PageRequest{
			Key:        input.Pagination.Key,
			Offset:     input.Pagination.Offset,
			Limit:      input.Pagination.Limit,
			CountTotal: input.Pagination.CountTotal,
			Reverse:    input.Pagination.Reverse,
		},
	}

	res, err := p.authzKeeper.GranteeGrants(ctx, msg)
	if err != nil {
		return nil, err
	}

	grants := make([]GrantAuthorization, 0, len(res.Grants))
	for _, grant := range res.Grants {
		var expiration int64
		if grant.Expiration != nil {
			expiration = grant.Expiration.Unix()
		}

		grants = append(grants, GrantAuthorization{
			Granter:       utils.AccAddressMustToHexAddress(grant.Granter),
			Grantee:       utils.AccAddressMustToHexAddress(grant.Grantee),
			Authorization: OutputsAuthorization(grant.Authorization.GetCachedValue().(authztypes.Authorization)),
			Expiration:    expiration,
		})
	}

	var pageResponse PageResponse
	pageResponse.NextKey = res.Pagination.NextKey
	pageResponse.Total = res.Pagination.Total

	return method.Outputs.Pack(grants, pageResponse)
}

// OutputsAuthorization marshals an authorization to its JSON string form using a
// codec seeded with the moca and cosmos-sdk module interfaces, falling back to the
// authorization's String() form on error.
func OutputsAuthorization(authorization authztypes.Authorization) string {
	interfaceRegistry := codectypes.NewInterfaceRegistry()

	authtypes.RegisterInterfaces(interfaceRegistry)
	authztypes.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	distrtypes.RegisterInterfaces(interfaceRegistry)
	slashingtypes.RegisterInterfaces(interfaceRegistry)
	govv1beta1.RegisterInterfaces(interfaceRegistry)
	govv1.RegisterInterfaces(interfaceRegistry)
	types.RegisterInterfaces(interfaceRegistry)
	feemarkettypes.RegisterInterfaces(interfaceRegistry)
	// Note: upgradetypes.RegisterInterfaces is not available in v0.50, interfaces are registered via module manager
	sptypes.RegisterInterfaces(interfaceRegistry)

	mocaCodec := codec.NewProtoCodec(interfaceRegistry)

	authorizationBytes, err := mocaCodec.MarshalInterfaceJSON(authorization)
	if err == nil {
		return string(authorizationBytes)
	}

	return authorization.String()
}
