package erc20

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	mocatypes "github.com/mocachain/moca/v2/types"
)

const (
	TransferMethod          = "transfer"
	TransferFromMethod      = "transferFrom"
	ApproveMethod           = "approve"
	IncreaseAllowanceMethod = "increaseAllowance"
	DecreaseAllowanceMethod = "decreaseAllowance"
	NameMethod              = "name"
	SymbolMethod            = "symbol"
	DecimalsMethod          = "decimals"
	TotalSupplyMethod       = "totalSupply"
	BalanceOfMethod         = "balanceOf"
	AllowanceMethod         = "allowance"

	TransferEvent = "Transfer"
	ApprovalEvent = "Approval"
)

const (
	GasTransfer          = 9_000
	GasTransferFrom      = 30_500
	GasApprove           = 8_100
	GasIncreaseAllowance = 8_580
	GasDecreaseAllowance = 3_620
	GasName              = 3_421
	GasSymbol            = 3_464
	GasDecimals          = 427
	GasTotalSupply       = 2_480
	GasBalanceOf         = 2_870
	GasAllowance         = 3_225
)

var erc20ABI = mocatypes.MustABIJson(abiJSON)

func GetMethodByID(input []byte) (abi.Method, error) {
	if len(input) < 4 {
		return abi.Method{}, fmt.Errorf("input length %d is too short", len(input))
	}
	for _, method := range erc20ABI.Methods {
		if bytes.Equal(input[:4], method.ID) {
			return method, nil
		}
	}
	return abi.Method{}, fmt.Errorf("method id %x does not exist", input[:4])
}

func MustMethod(name string) abi.Method {
	method, ok := erc20ABI.Methods[name]
	if !ok {
		panic(fmt.Errorf("method %s does not exist", name))
	}
	return method
}

func MustEvent(name string) abi.Event {
	event, ok := erc20ABI.Events[name]
	if !ok {
		panic(fmt.Errorf("event %s does not exist", name))
	}
	return event
}

type transferArgs struct {
	To     common.Address `abi:"to"`
	Amount *big.Int       `abi:"amount"`
}

func (a *transferArgs) Validate() error {
	return validateAmount(a.Amount)
}

type transferFromArgs struct {
	From   common.Address `abi:"from"`
	To     common.Address `abi:"to"`
	Amount *big.Int       `abi:"amount"`
}

func (a *transferFromArgs) Validate() error {
	return validateAmount(a.Amount)
}

type approveArgs struct {
	Spender common.Address `abi:"spender"`
	Amount  *big.Int       `abi:"amount"`
}

func (a *approveArgs) Validate() error {
	return validateAmount(a.Amount)
}

type balanceOfArgs struct {
	Account common.Address `abi:"account"`
}

func (a *balanceOfArgs) Validate() error {
	return nil
}

type allowanceArgs struct {
	Owner   common.Address `abi:"owner"`
	Spender common.Address `abi:"spender"`
}

func (a *allowanceArgs) Validate() error {
	return nil
}

func validateAmount(amount *big.Int) error {
	if amount == nil {
		return fmt.Errorf("amount is nil")
	}
	if amount.Sign() < 0 {
		return fmt.Errorf("amount %s is negative", amount)
	}
	if amount.BitLen() > 256 {
		return fmt.Errorf("amount %s is greater than max value of uint256", amount)
	}
	return nil
}
