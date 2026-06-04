package erc20

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	erc20types "github.com/mocachain/moca/v2/x/erc20/types"
	evmtypes "github.com/mocachain/moca/v2/x/evm/types"
)

type Contract struct {
	ctx         sdk.Context
	tokenPair   erc20types.TokenPair
	bankKeeper  evmtypes.BankKeeper
	erc20Keeper evmtypes.ERC20Keeper
}

func NewPrecompiledContract(
	ctx sdk.Context,
	tokenPair erc20types.TokenPair,
	bankKeeper evmtypes.BankKeeper,
	erc20Keeper evmtypes.ERC20Keeper,
) *Contract {
	return &Contract{
		ctx:         ctx,
		tokenPair:   tokenPair,
		bankKeeper:  bankKeeper,
		erc20Keeper: erc20Keeper,
	}
}

func (c *Contract) Address() common.Address {
	return c.tokenPair.GetERC20Contract()
}

func (c *Contract) RequiredGas(input []byte) uint64 {
	method, err := GetMethodByID(input)
	if err != nil {
		return 0
	}

	switch method.Name {
	case TransferMethod:
		return GasTransfer
	case TransferFromMethod:
		return GasTransferFrom
	case ApproveMethod:
		return GasApprove
	case IncreaseAllowanceMethod:
		return GasIncreaseAllowance
	case DecreaseAllowanceMethod:
		return GasDecreaseAllowance
	case NameMethod:
		return GasName
	case SymbolMethod:
		return GasSymbol
	case DecimalsMethod:
		return GasDecimals
	case TotalSupplyMethod:
		return GasTotalSupply
	case BalanceOfMethod:
		return GasBalanceOf
	case AllowanceMethod:
		return GasAllowance
	default:
		return 0
	}
}

func (c *Contract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if contract.Value().Sign() > 0 {
		return evmtypes.PackRetError(fmt.Sprintf("cannot receive funds, received: %s", contract.Value()))
	}
	if len(contract.Input) < 4 {
		return evmtypes.PackRetError("invalid input")
	}

	ctx, commit := c.ctx.CacheContext()
	snapshot := evm.StateDB.Snapshot()
	method, err := GetMethodByID(contract.Input)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		return evmtypes.PackRetError(err.Error())
	}
	if readonly && isTransaction(method.Name) {
		evm.StateDB.RevertToSnapshot(snapshot)
		return nil, evmtypes.ErrReadOnly
	}

	ret, err := c.handleMethod(ctx, evm, contract, method)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		return nil, err
	}

	commit()
	return ret, nil
}

func isTransaction(method string) bool {
	switch method {
	case TransferMethod, TransferFromMethod, ApproveMethod, IncreaseAllowanceMethod, DecreaseAllowanceMethod:
		return true
	default:
		return false
	}
}

func (c *Contract) handleMethod(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method abi.Method) ([]byte, error) {
	switch method.Name {
	case TransferMethod:
		var args transferArgs
		if err := evmtypes.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
			return nil, err
		}
		return c.transfer(ctx, evm, method, contract.Caller(), args.To, args.Amount)
	case TransferFromMethod:
		var args transferFromArgs
		if err := evmtypes.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
			return nil, err
		}
		return c.transferFrom(ctx, evm, method, contract.Caller(), args.From, args.To, args.Amount)
	case ApproveMethod:
		var args approveArgs
		if err := evmtypes.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
			return nil, err
		}
		return c.approve(ctx, evm, method, contract.Caller(), args.Spender, args.Amount)
	case IncreaseAllowanceMethod:
		var args approveArgs
		if err := evmtypes.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
			return nil, err
		}
		return c.increaseAllowance(ctx, evm, method, contract.Caller(), args.Spender, args.Amount)
	case DecreaseAllowanceMethod:
		var args approveArgs
		if err := evmtypes.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
			return nil, err
		}
		return c.decreaseAllowance(ctx, evm, method, contract.Caller(), args.Spender, args.Amount)
	case NameMethod:
		return method.Outputs.Pack(c.name(ctx))
	case SymbolMethod:
		return method.Outputs.Pack(c.symbol(ctx))
	case DecimalsMethod:
		decimals, err := c.decimals(ctx)
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(decimals)
	case TotalSupplyMethod:
		return method.Outputs.Pack(c.bankKeeper.GetSupply(ctx, c.tokenPair.Denom).Amount.BigInt())
	case BalanceOfMethod:
		var args balanceOfArgs
		if err := evmtypes.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
			return nil, err
		}
		return method.Outputs.Pack(c.bankKeeper.GetBalance(ctx, args.Account.Bytes(), c.tokenPair.Denom).Amount.BigInt())
	case AllowanceMethod:
		var args allowanceArgs
		if err := evmtypes.ParseMethodArgs(method, &args, contract.Input[4:]); err != nil {
			return nil, err
		}
		allowance, err := c.erc20Keeper.GetAllowance(ctx, c.Address(), args.Owner, args.Spender)
		if err != nil {
			allowance = common.Big0
		}
		return method.Outputs.Pack(allowance)
	default:
		return nil, fmt.Errorf("method %s is not handled", method.Name)
	}
}

func (c *Contract) transfer(
	ctx sdk.Context,
	evm *vm.EVM,
	method abi.Method,
	from common.Address,
	to common.Address,
	amount *big.Int,
) ([]byte, error) {
	if err := c.bankSend(ctx, from, to, amount); err != nil {
		return nil, err
	}
	if err := c.emitTransferEvent(ctx, evm, from, to, amount); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(true)
}

func (c *Contract) transferFrom(
	ctx sdk.Context,
	evm *vm.EVM,
	method abi.Method,
	spender common.Address,
	from common.Address,
	to common.Address,
	amount *big.Int,
) ([]byte, error) {
	allowance, err := c.erc20Keeper.GetAllowance(ctx, c.Address(), from, spender)
	if err != nil {
		return nil, err
	}
	newAllowance := new(big.Int).Sub(allowance, amount)
	if newAllowance.Sign() < 0 {
		return nil, fmt.Errorf("ERC20: insufficient allowance")
	}

	if err := c.bankSend(ctx, from, to, amount); err != nil {
		return nil, err
	}
	if err := c.setAllowance(ctx, from, spender, newAllowance); err != nil {
		return nil, err
	}
	if err := c.emitTransferEvent(ctx, evm, from, to, amount); err != nil {
		return nil, err
	}
	if err := c.emitApprovalEvent(ctx, evm, from, spender, newAllowance); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(true)
}

func (c *Contract) approve(
	ctx sdk.Context,
	evm *vm.EVM,
	method abi.Method,
	owner common.Address,
	spender common.Address,
	amount *big.Int,
) ([]byte, error) {
	if err := c.setAllowance(ctx, owner, spender, amount); err != nil {
		return nil, err
	}
	if err := c.emitApprovalEvent(ctx, evm, owner, spender, amount); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(true)
}

func (c *Contract) increaseAllowance(
	ctx sdk.Context,
	evm *vm.EVM,
	method abi.Method,
	owner common.Address,
	spender common.Address,
	addedValue *big.Int,
) ([]byte, error) {
	if addedValue.Sign() <= 0 {
		return nil, fmt.Errorf("cannot increase allowance with non-positive values")
	}
	allowance, err := c.erc20Keeper.GetAllowance(ctx, c.Address(), owner, spender)
	if err != nil {
		return nil, err
	}
	amount := new(big.Int).Add(allowance, addedValue)
	if amount.BitLen() > 256 {
		return nil, fmt.Errorf("amount %s causes integer overflow", amount)
	}
	if err := c.setAllowance(ctx, owner, spender, amount); err != nil {
		return nil, err
	}
	if err := c.emitApprovalEvent(ctx, evm, owner, spender, amount); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(true)
}

func (c *Contract) decreaseAllowance(
	ctx sdk.Context,
	evm *vm.EVM,
	method abi.Method,
	owner common.Address,
	spender common.Address,
	subtractedValue *big.Int,
) ([]byte, error) {
	if subtractedValue.Sign() <= 0 {
		return nil, fmt.Errorf("cannot decrease allowance with non-positive values")
	}
	allowance, err := c.erc20Keeper.GetAllowance(ctx, c.Address(), owner, spender)
	if err != nil {
		return nil, err
	}
	amount := new(big.Int).Sub(allowance, subtractedValue)
	if amount.Sign() < 0 {
		return nil, fmt.Errorf("ERC20: decreased allowance below zero")
	}
	if err := c.setAllowance(ctx, owner, spender, amount); err != nil {
		return nil, err
	}
	if err := c.emitApprovalEvent(ctx, evm, owner, spender, amount); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(true)
}

func (c *Contract) bankSend(ctx sdk.Context, from, to common.Address, amount *big.Int) error {
	coins := sdk.NewCoins(sdk.NewCoin(c.tokenPair.Denom, sdkmath.NewIntFromBigInt(amount)))
	msg := banktypes.NewMsgSend(from.Bytes(), to.Bytes(), coins)
	if err := msg.Amount.Validate(); err != nil {
		return err
	}

	return c.bankKeeper.SendCoins(ctx, from.Bytes(), to.Bytes(), coins)
}

func (c *Contract) setAllowance(ctx sdk.Context, owner, spender common.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return c.erc20Keeper.DeleteAllowance(ctx, c.Address(), owner, spender)
	}
	return c.erc20Keeper.SetAllowance(ctx, c.Address(), owner, spender, amount)
}

func (c *Contract) name(ctx sdk.Context) string {
	metadata, found := c.bankKeeper.GetDenomMetaData(ctx, c.tokenPair.Denom)
	if found && metadata.Name != "" {
		return metadata.Name
	}
	return fallbackName(c.tokenPair.Denom)
}

func (c *Contract) symbol(ctx sdk.Context) string {
	metadata, found := c.bankKeeper.GetDenomMetaData(ctx, c.tokenPair.Denom)
	if found && metadata.Symbol != "" {
		return metadata.Symbol
	}
	return strings.ToUpper(fallbackName(c.tokenPair.Denom))
}

func (c *Contract) decimals(ctx sdk.Context) (uint8, error) {
	metadata, found := c.bankKeeper.GetDenomMetaData(ctx, c.tokenPair.Denom)
	if !found {
		return 0, nil
	}

	for i := len(metadata.DenomUnits) - 1; i >= 0; i-- {
		if metadata.DenomUnits[i].Denom != metadata.Display {
			continue
		}
		if metadata.DenomUnits[i].Exponent > math.MaxUint8 {
			return 0, fmt.Errorf("uint8 overflow: invalid decimals: %d", metadata.DenomUnits[i].Exponent)
		}
		return uint8(metadata.DenomUnits[i].Exponent), nil
	}
	return 0, nil
}

func fallbackName(denom string) string {
	parts := strings.FieldsFunc(denom, func(r rune) bool {
		return r == '/' || r == ':' || r == '-' || r == '_' || r == '.'
	})
	if len(parts) == 0 {
		return denom
	}
	return parts[len(parts)-1]
}

func (c *Contract) emitTransferEvent(ctx sdk.Context, evm *vm.EVM, from, to common.Address, value *big.Int) error {
	return c.addLog(
		ctx,
		evm,
		MustEvent(TransferEvent),
		[]common.Hash{common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())},
		value,
	)
}

func (c *Contract) emitApprovalEvent(ctx sdk.Context, evm *vm.EVM, owner, spender common.Address, value *big.Int) error {
	return c.addLog(
		ctx,
		evm,
		MustEvent(ApprovalEvent),
		[]common.Hash{common.BytesToHash(owner.Bytes()), common.BytesToHash(spender.Bytes())},
		value,
	)
}

func (c *Contract) addLog(ctx sdk.Context, evm *vm.EVM, event abi.Event, topics []common.Hash, args ...interface{}) error {
	data, newTopics, err := evmtypes.PackTopicData(event, topics, args...)
	if err != nil {
		return err
	}
	blockHeight := ctx.BlockHeight()
	if blockHeight < 0 {
		return fmt.Errorf("block height %d is negative", blockHeight)
	}
	evm.StateDB.AddLog(&ethtypes.Log{
		Address:     c.Address(),
		Topics:      newTopics,
		Data:        data,
		BlockNumber: uint64(blockHeight),
	})
	return nil
}
