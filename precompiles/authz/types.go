package authz

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/mocachain/moca/v2/types"
)

var (
	authzAddress = common.HexToAddress(types.AuthzAddress)
	authzABI     = types.MustABIJson(IAuthzMetaData.ABI)
)

// GetAddress returns the authz precompile's fixed hex address.
func GetAddress() common.Address {
	return authzAddress
}

// GetMethod resolves an ABI method by name.
func GetMethod(name string) (abi.Method, error) {
	method := authzABI.Methods[name]
	if method.ID == nil {
		return abi.Method{}, fmt.Errorf("method %s is not exist", name)
	}
	return method, nil
}

// MustMethod resolves an ABI method by name and panics if it does not exist.
func MustMethod(name string) abi.Method {
	method, err := GetMethod(name)
	if err != nil {
		panic(err)
	}
	return method
}

// GetEvent resolves an ABI event by name.
func GetEvent(name string) (abi.Event, error) {
	event := authzABI.Events[name]
	if event.ID == (common.Hash{}) {
		return abi.Event{}, fmt.Errorf("event %s is not exist", name)
	}
	return event, nil
}

// MustEvent resolves an ABI event by name and panics if it does not exist.
func MustEvent(name string) abi.Event {
	event, err := GetEvent(name)
	if err != nil {
		panic(err)
	}
	return event
}

// The arg structs below are decode targets for cmn.SetupABI's positional args via
// abi.Arguments.Copy; their fields carry the ABI names (and hex address types).

type GrantArgs struct {
	Grantee       common.Address `abi:"grantee"`
	AuthzType     string         `abi:"authzType"`
	Authorization string         `abi:"authorization"`
	Limit         []Coin         `abi:"limit"`
	Expiration    int64          `abi:"expiration"`
}

// StakingParams parses the allowed/denied validator list from the staking
// authorization string (e.g. "allowed:0x..,0x.." or "denied:0x..").
func (args *GrantArgs) StakingParams() (allowed []sdk.AccAddress, denied []sdk.AccAddress, err error) {
	err = fmt.Errorf("authorization input example allowed:0x00000004e1E16f249E2b71c2dc66545215FE9d84,0x1111102Dd32160B064F2A512CDEf74bFdB6a9F96 or denied:0x00000004e1E16f249E2b71c2dc66545215FE9d84, but you input is %s", args.Authorization)

	switch args.AuthzType {
	case AuthzTypeDelegate, AuthzTypeUnbond, AuthzTypeRedelegate:
		// Authorization input example
		// allowed:0x00000004e1E16f249E2b71c2dc66545215FE9d84,0x1111102Dd32160B064F2A512CDEf74bFdB6a9F96
		// or
		// denied:0x00000004e1E16f249E2b71c2dc66545215FE9d84

		authorizationArr := strings.Split(args.Authorization, ":")
		if len(authorizationArr) != 2 {
			return nil, nil, err
		}

		authorizationType := authorizationArr[0]
		authorizationData := authorizationArr[1]

		validatorList := strings.Split(authorizationData, ",")
		if len(validatorList) < 1 {
			return nil, nil, err
		}

		var validators []sdk.AccAddress
		for _, validatorStr := range validatorList {
			validators = append(validators, common.HexToAddress(validatorStr).Bytes())
		}

		if authorizationType == "allowed" {
			return validators, nil, nil
		} else if authorizationType == "denied" {
			return nil, validators, nil
		} else {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("auth type %s not need staking params", args.AuthzType)
	}
}

// SendParams parses the allowed recipient list from the send authorization
// string (e.g. "allowed:0x..,0x..").
func (args *GrantArgs) SendParams() (allowed []sdk.AccAddress, err error) {
	err = fmt.Errorf("authorization input example allowed:0x00000004e1E16f249E2b71c2dc66545215FE9d84,0x1111102Dd32160B064F2A512CDEf74bFdB6a9F96 but you input is %s", args.Authorization)

	switch args.AuthzType {
	case AuthzTypeSend:
		// Authorization input example
		// allowed:0x00000004e1E16f249E2b71c2dc66545215FE9d84,0x1111102Dd32160B064F2A512CDEf74bFdB6a9F96

		authorizationArr := strings.Split(args.Authorization, ":")
		if len(authorizationArr) != 2 {
			return nil, err
		}

		authorizationType := authorizationArr[0]
		authorizationData := authorizationArr[1]

		validatorList := strings.Split(authorizationData, ",")
		if len(validatorList) < 1 {
			return nil, err
		}

		for _, validatorStr := range validatorList {
			allowed = append(allowed, common.HexToAddress(validatorStr).Bytes())
		}

		if authorizationType == "allowed" {
			return allowed, nil
		}
		return nil, err
	default:
		return nil, fmt.Errorf("auth type %s not need staking params", args.AuthzType)
	}
}

type RevokeArgs struct {
	Grantee    common.Address `abi:"grantee"`
	MsgTypeURL string         `abi:"msgTypeUrl"`
}

type ExecArgs struct {
	Msgs []string `abi:"msgs"`
}

type GrantsArgs struct {
	Granter    common.Address `abi:"granter"`
	Grantee    common.Address `abi:"grantee"`
	MsgTypeURL string         `abi:"msgTypeUrl"`
	Pagination PageRequest    `abi:"pagination"`
}

type GranterGrantsArgs struct {
	Granter    common.Address `abi:"granter"`
	Pagination PageRequest    `abi:"pagination"`
}

type GranteeGrantsArgs struct {
	Grantee    common.Address `abi:"grantee"`
	Pagination PageRequest    `abi:"pagination"`
}
