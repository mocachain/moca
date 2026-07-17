package authz

import (
	"encoding/json"
	"fmt"
	"time"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"

	"github.com/mocachain/moca/v2/precompiles/types"
	challengetypes "github.com/mocachain/moca/v2/x/challenge/types"
	gensptypes "github.com/mocachain/moca/v2/x/gensp/types"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	permissiontypes "github.com/mocachain/moca/v2/x/permission/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	virtualgrouptypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

const (
	// GrantMethodName is the ABI name for the Grant transaction.
	GrantMethodName = "grant"
	// RevokeMethodName is the ABI name for the Revoke transaction.
	RevokeMethodName = "revoke"
	// ExecMethodName is the ABI name for the Exec transaction.
	ExecMethodName = "exec"

	// AuthzTypeSend is the authorization type for bank send grants.
	AuthzTypeSend = "send"
	// AuthzTypeGeneric is the authorization type for generic grants.
	AuthzTypeGeneric = "generic"
	// AuthzTypeDelegate is the authorization type for staking delegate grants.
	AuthzTypeDelegate = "delegate"
	// AuthzTypeUnbond is the authorization type for staking unbond grants.
	AuthzTypeUnbond = "unbond"
	// AuthzTypeRedelegate is the authorization type for staking redelegate grants.
	AuthzTypeRedelegate = "redelegate"
	// AuthzTypeSpDeposit is the authorization type for moca sp deposit grants.
	AuthzTypeSpDeposit = "spDeposit"
)

// Grant implements the MsgServer.Grant method to create a new grant.
func (p Precompile) Grant(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input GrantArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	var limit sdk.Coins
	for _, coin := range input.Limit {
		if coin.Amount.Sign() > 0 {
			limit = limit.Add(sdk.Coin{
				Denom:  coin.Denom,
				Amount: math.NewIntFromBigInt(coin.Amount),
			})
		}
	}

	// more details see https://github.com/mocachain/moca-cosmos-sdk/blob/1ad031a3d3a4b73997d72b8012397633b3cdcae2/x/authz/client/cli/tx.go#L56-L202
	// TODO
	var authorization authz.Authorization
	switch input.AuthzType {
	case AuthzTypeSend:
		// Authorization input example
		// allowed:0x00000004e1E16f249E2b71c2dc66545215FE9d84,0x1111102Dd32160B064F2A512CDEf74bFdB6a9F96
		allowed, err := input.SendParams()
		if err != nil {
			return nil, err
		}
		authorization = banktypes.NewSendAuthorization(limit, allowed)
	case AuthzTypeGeneric:
		authorization = authz.NewGenericAuthorization(input.Authorization)
	case AuthzTypeSpDeposit:
		spAddress := sdk.MustAccAddressFromHex(input.Authorization)
		find, amount := limit.Find(sptypes.DefaultDepositDenom)
		if !find || len(limit.Denoms()) > 1 {
			return nil, fmt.Errorf("limit %s is invalid", limit.String())
		}
		authorization = sptypes.NewDepositAuthorization(spAddress, &amount)
	case AuthzTypeDelegate, AuthzTypeUnbond, AuthzTypeRedelegate:
		if limit.Len() != 1 {
			return nil, fmt.Errorf("limit length must be 1, but limit is %s", limit.String())
		}
		// Authorization input example
		// allowed:0x00000004e1E16f249E2b71c2dc66545215FE9d84,0x1111102Dd32160B064F2A512CDEf74bFdB6a9F96
		// or
		// denied:0x00000004e1E16f249E2b71c2dc66545215FE9d84
		allowed, denied, err := input.StakingParams()
		if err != nil {
			return nil, err
		}

		authzType := stakingtypes.AuthorizationType_AUTHORIZATION_TYPE_REDELEGATE
		switch input.AuthzType {
		case AuthzTypeDelegate:
			authzType = stakingtypes.AuthorizationType_AUTHORIZATION_TYPE_DELEGATE
		case AuthzTypeUnbond:
			authzType = stakingtypes.AuthorizationType_AUTHORIZATION_TYPE_UNDELEGATE
		}

		authorization, err = stakingtypes.NewStakeAuthorization(allowed, denied, authzType, &limit[0])
	default:
		return nil, fmt.Errorf("invalid authorization type %s", input.AuthzType)
	}

	var expiration *time.Time
	if input.Expiration > 0 {
		exp := time.Unix(input.Expiration, 0)
		expiration = &exp
	}
	msg, err := authz.NewMsgGrant(contract.Caller().Bytes(), input.Grantee.Bytes(), authorization, expiration)
	if err != nil {
		return nil, err
	}

	if _, err = p.authzKeeper.Grant(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitGrantEvent(evm, contract.Caller(), input.Grantee, input.AuthzType); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Revoke implements the MsgServer.Revoke method.
func (p Precompile) Revoke(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input RevokeArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &authz.MsgRevoke{
		Granter:    sdk.AccAddress(contract.Caller().Bytes()).String(),
		Grantee:    sdk.AccAddress(input.Grantee.Bytes()).String(),
		MsgTypeUrl: input.MsgTypeURL,
	}

	if _, err := p.authzKeeper.Revoke(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitRevokeEvent(evm, contract.Caller(), input.Grantee, input.MsgTypeURL); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Exec implements the MsgServer.Exec method.
func (p Precompile) Exec(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	if evm.Origin != contract.Caller() {
		return nil, types.ErrInvalidCaller
	}

	var input ExecArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	interfaceRegistry := codectypes.NewInterfaceRegistry()

	authtypes.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	distrtypes.RegisterInterfaces(interfaceRegistry)
	slashingtypes.RegisterInterfaces(interfaceRegistry)
	govv1beta1.RegisterInterfaces(interfaceRegistry)
	govv1.RegisterInterfaces(interfaceRegistry)
	// Note: upgradetypes.RegisterInterfaces is not available in v0.50, interfaces are registered via module manager
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	types.RegisterInterfaces(interfaceRegistry)

	challengetypes.RegisterInterfaces(interfaceRegistry)
	feemarkettypes.RegisterInterfaces(interfaceRegistry)
	gensptypes.RegisterInterfaces(interfaceRegistry)
	paymenttypes.RegisterInterfaces(interfaceRegistry)
	permissiontypes.RegisterInterfaces(interfaceRegistry)
	sptypes.RegisterInterfaces(interfaceRegistry)
	storagetypes.RegisterInterfaces(interfaceRegistry)
	virtualgrouptypes.RegisterInterfaces(interfaceRegistry)

	ethosCodec := codec.NewProtoCodec(interfaceRegistry)

	msgs := make([]sdk.Msg, len(input.Msgs))
	for i, message := range input.Msgs {
		var msg sdk.Msg
		var rawMessage json.RawMessage
		if err := json.Unmarshal([]byte(message), &rawMessage); err != nil {
			return nil, err
		}
		if err := ethosCodec.UnmarshalInterfaceJSON(rawMessage, &msg); err != nil {
			return nil, err
		}

		msgs[i] = msg
	}

	msg := authz.NewMsgExec(sdk.AccAddress(contract.Caller().Bytes()), msgs)
	if _, err := p.authzKeeper.Exec(ctx, &msg); err != nil {
		return nil, err
	}

	if err := p.EmitExecEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
