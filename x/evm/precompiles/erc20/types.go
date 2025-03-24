package erc20

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v12/types"
)

var (
	erc20Address = common.HexToAddress(types.Erc20Address)
	erc20ABI     = types.MustABIJson(IErc20MetaData.ABI)
)

func GetAddress() common.Address {
	return erc20Address
}

func GetMethod(name string) (abi.Method, error) {
	method := erc20ABI.Methods[name]
	if method.ID == nil {
		return abi.Method{}, fmt.Errorf("method %s is not exist", name)
	}
	return method, nil
}

func GetMethodByID(input []byte) (abi.Method, error) {
	if len(input) < 4 {
		return abi.Method{}, fmt.Errorf("input length %d is too short", len(input))
	}
	for _, method := range erc20ABI.Methods {
		if bytes.Equal(input[:4], method.ID) {
			return method, nil
		}
	}
	return abi.Method{}, fmt.Errorf("method id %s is not exist", string(input[:4]))
}

func MustMethod(name string) abi.Method {
	method, err := GetMethod(name)
	if err != nil {
		panic(err)
	}
	return method
}

func GetEvent(name string) (abi.Event, error) {
	event := erc20ABI.Events[name]
	if event.ID == (common.Hash{}) {
		return abi.Event{}, fmt.Errorf("event %s is not exist", name)
	}
	return event, nil
}

func MustEvent(name string) abi.Event {
	event, err := GetEvent(name)
	if err != nil {
		panic(err)
	}
	return event
}

type CoinJson = Coin

type ConvertCoinArgs struct {
	Coin     CoinJson       `abi:"coin"`
	Receiver common.Address `abi:"receiver"`
}

// Validate ConvertCoin args
func (args *ConvertCoinArgs) Validate() error {
	if args.Coin.Amount.Sign() <= 0 {
		return fmt.Errorf("convertCoin %s amount is %s, need to greater than 0", args.Coin.Denom, args.Coin.Amount.String())
	}
	return nil
}

type ConvertERC20Args struct {
	ContractAddress common.Address `abi:"contractAddress"`
	Amount          *big.Int       `abi:"amount"`
	Receiver        common.Address `abi:"receiver"`
}

// Validate ConvertERC20 args
func (args *ConvertERC20Args) Validate() error {
	if args.Amount.Sign() <= 0 {
		return fmt.Errorf("convertERC20 %s amount is %s, need to greater than 0", args.ContractAddress.String(), args.Amount.String())
	}
	return nil
}

type PageRequestJson = PageRequest

type TokenPairsArgs struct {
	PageRequest PageRequestJson `abi:"pageRequest"`
}

// Validate TokenPairs args
func (args *TokenPairsArgs) Validate() error {
	return nil
}

type TokenPairArgs struct {
	Token common.Address `abi:"token"`
}

// Validate TokenPair args
func (args *TokenPairArgs) Validate() error {
	if args.Token == (common.Address{}) {
		return fmt.Errorf("token is empty")
	}
	return nil
}
