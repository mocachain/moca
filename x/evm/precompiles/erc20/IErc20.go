// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package erc20

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// Coin is an auto generated low-level Go binding around an user-defined struct.
type Coin struct {
	Denom  string
	Amount *big.Int
}

// PageRequest is an auto generated low-level Go binding around an user-defined struct.
type PageRequest struct {
	Key        []byte
	Offset     uint64
	Limit      uint64
	CountTotal bool
	Reverse    bool
}

// PageResponse is an auto generated low-level Go binding around an user-defined struct.
type PageResponse struct {
	NextKey []byte
	Total   uint64
}

// Params is an auto generated low-level Go binding around an user-defined struct.
type Params struct {
	EnableErc20   bool
	EnableEvmHook bool
}

// TokenPair is an auto generated low-level Go binding around an user-defined struct.
type TokenPair struct {
	Erc20Address  common.Address
	Denom         string
	Enabled       bool
	ContractOwner uint8
}

// IErc20MetaData contains all meta data concerning the IErc20 contract.
var IErc20MetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"ConvertCoin\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"contractAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"ConvertERC20\",\"type\":\"event\"},{\"inputs\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"internalType\":\"structCoin\",\"name\":\"coin\",\"type\":\"tuple\"},{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"}],\"name\":\"convertCoin\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"contractAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"}],\"name\":\"convertERC20\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"params\",\"outputs\":[{\"components\":[{\"internalType\":\"bool\",\"name\":\"enableErc20\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"enableEvmHook\",\"type\":\"bool\"}],\"internalType\":\"structParams\",\"name\":\"params\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"}],\"name\":\"tokenPair\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"erc20Address\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"enabled\",\"type\":\"bool\"},{\"internalType\":\"enumOwner\",\"name\":\"contractOwner\",\"type\":\"uint8\"}],\"internalType\":\"structTokenPair\",\"name\":\"tokenPair\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes\",\"name\":\"key\",\"type\":\"bytes\"},{\"internalType\":\"uint64\",\"name\":\"offset\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"limit\",\"type\":\"uint64\"},{\"internalType\":\"bool\",\"name\":\"countTotal\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"reverse\",\"type\":\"bool\"}],\"internalType\":\"structPageRequest\",\"name\":\"pagination\",\"type\":\"tuple\"}],\"name\":\"tokenPairs\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"erc20Address\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"denom\",\"type\":\"string\"},{\"internalType\":\"bool\",\"name\":\"enabled\",\"type\":\"bool\"},{\"internalType\":\"enumOwner\",\"name\":\"contractOwner\",\"type\":\"uint8\"}],\"internalType\":\"structTokenPair[]\",\"name\":\"tokenPairs\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"bytes\",\"name\":\"nextKey\",\"type\":\"bytes\"},{\"internalType\":\"uint64\",\"name\":\"total\",\"type\":\"uint64\"}],\"internalType\":\"structPageResponse\",\"name\":\"pageResponse\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// IErc20ABI is the input ABI used to generate the binding from.
// Deprecated: Use IErc20MetaData.ABI instead.
var IErc20ABI = IErc20MetaData.ABI

// IErc20 is an auto generated Go binding around an Ethereum contract.
type IErc20 struct {
	IErc20Caller     // Read-only binding to the contract
	IErc20Transactor // Write-only binding to the contract
	IErc20Filterer   // Log filterer for contract events
}

// IErc20Caller is an auto generated read-only Go binding around an Ethereum contract.
type IErc20Caller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IErc20Transactor is an auto generated write-only Go binding around an Ethereum contract.
type IErc20Transactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IErc20Filterer is an auto generated log filtering Go binding around an Ethereum contract events.
type IErc20Filterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IErc20Session is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type IErc20Session struct {
	Contract     *IErc20           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IErc20CallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type IErc20CallerSession struct {
	Contract *IErc20Caller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// IErc20TransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type IErc20TransactorSession struct {
	Contract     *IErc20Transactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IErc20Raw is an auto generated low-level Go binding around an Ethereum contract.
type IErc20Raw struct {
	Contract *IErc20 // Generic contract binding to access the raw methods on
}

// IErc20CallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type IErc20CallerRaw struct {
	Contract *IErc20Caller // Generic read-only contract binding to access the raw methods on
}

// IErc20TransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type IErc20TransactorRaw struct {
	Contract *IErc20Transactor // Generic write-only contract binding to access the raw methods on
}

// NewIErc20 creates a new instance of IErc20, bound to a specific deployed contract.
func NewIErc20(address common.Address, backend bind.ContractBackend) (*IErc20, error) {
	contract, err := bindIErc20(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &IErc20{IErc20Caller: IErc20Caller{contract: contract}, IErc20Transactor: IErc20Transactor{contract: contract}, IErc20Filterer: IErc20Filterer{contract: contract}}, nil
}

// NewIErc20Caller creates a new read-only instance of IErc20, bound to a specific deployed contract.
func NewIErc20Caller(address common.Address, caller bind.ContractCaller) (*IErc20Caller, error) {
	contract, err := bindIErc20(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &IErc20Caller{contract: contract}, nil
}

// NewIErc20Transactor creates a new write-only instance of IErc20, bound to a specific deployed contract.
func NewIErc20Transactor(address common.Address, transactor bind.ContractTransactor) (*IErc20Transactor, error) {
	contract, err := bindIErc20(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &IErc20Transactor{contract: contract}, nil
}

// NewIErc20Filterer creates a new log filterer instance of IErc20, bound to a specific deployed contract.
func NewIErc20Filterer(address common.Address, filterer bind.ContractFilterer) (*IErc20Filterer, error) {
	contract, err := bindIErc20(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &IErc20Filterer{contract: contract}, nil
}

// bindIErc20 binds a generic wrapper to an already deployed contract.
func bindIErc20(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := IErc20MetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IErc20 *IErc20Raw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IErc20.Contract.IErc20Caller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IErc20 *IErc20Raw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IErc20.Contract.IErc20Transactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IErc20 *IErc20Raw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IErc20.Contract.IErc20Transactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IErc20 *IErc20CallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IErc20.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IErc20 *IErc20TransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IErc20.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IErc20 *IErc20TransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IErc20.Contract.contract.Transact(opts, method, params...)
}

// Params is a free data retrieval call binding the contract method 0xcff0ab96.
//
// Solidity: function params() view returns((bool,bool) params)
func (_IErc20 *IErc20Caller) Params(opts *bind.CallOpts) (Params, error) {
	var out []interface{}
	err := _IErc20.contract.Call(opts, &out, "params")

	if err != nil {
		return *new(Params), err
	}

	out0 := *abi.ConvertType(out[0], new(Params)).(*Params)

	return out0, err

}

// Params is a free data retrieval call binding the contract method 0xcff0ab96.
//
// Solidity: function params() view returns((bool,bool) params)
func (_IErc20 *IErc20Session) Params() (Params, error) {
	return _IErc20.Contract.Params(&_IErc20.CallOpts)
}

// Params is a free data retrieval call binding the contract method 0xcff0ab96.
//
// Solidity: function params() view returns((bool,bool) params)
func (_IErc20 *IErc20CallerSession) Params() (Params, error) {
	return _IErc20.Contract.Params(&_IErc20.CallOpts)
}

// TokenPair is a free data retrieval call binding the contract method 0x1cc6391e.
//
// Solidity: function tokenPair(address token) view returns((address,string,bool,uint8) tokenPair)
func (_IErc20 *IErc20Caller) TokenPair(opts *bind.CallOpts, token common.Address) (TokenPair, error) {
	var out []interface{}
	err := _IErc20.contract.Call(opts, &out, "tokenPair", token)

	if err != nil {
		return *new(TokenPair), err
	}

	out0 := *abi.ConvertType(out[0], new(TokenPair)).(*TokenPair)

	return out0, err

}

// TokenPair is a free data retrieval call binding the contract method 0x1cc6391e.
//
// Solidity: function tokenPair(address token) view returns((address,string,bool,uint8) tokenPair)
func (_IErc20 *IErc20Session) TokenPair(token common.Address) (TokenPair, error) {
	return _IErc20.Contract.TokenPair(&_IErc20.CallOpts, token)
}

// TokenPair is a free data retrieval call binding the contract method 0x1cc6391e.
//
// Solidity: function tokenPair(address token) view returns((address,string,bool,uint8) tokenPair)
func (_IErc20 *IErc20CallerSession) TokenPair(token common.Address) (TokenPair, error) {
	return _IErc20.Contract.TokenPair(&_IErc20.CallOpts, token)
}

// TokenPairs is a free data retrieval call binding the contract method 0xff90f0c1.
//
// Solidity: function tokenPairs((bytes,uint64,uint64,bool,bool) pagination) view returns((address,string,bool,uint8)[] tokenPairs, (bytes,uint64) pageResponse)
func (_IErc20 *IErc20Caller) TokenPairs(opts *bind.CallOpts, pagination PageRequest) (struct {
	TokenPairs   []TokenPair
	PageResponse PageResponse
}, error) {
	var out []interface{}
	err := _IErc20.contract.Call(opts, &out, "tokenPairs", pagination)

	outstruct := new(struct {
		TokenPairs   []TokenPair
		PageResponse PageResponse
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.TokenPairs = *abi.ConvertType(out[0], new([]TokenPair)).(*[]TokenPair)
	outstruct.PageResponse = *abi.ConvertType(out[1], new(PageResponse)).(*PageResponse)

	return *outstruct, err

}

// TokenPairs is a free data retrieval call binding the contract method 0xff90f0c1.
//
// Solidity: function tokenPairs((bytes,uint64,uint64,bool,bool) pagination) view returns((address,string,bool,uint8)[] tokenPairs, (bytes,uint64) pageResponse)
func (_IErc20 *IErc20Session) TokenPairs(pagination PageRequest) (struct {
	TokenPairs   []TokenPair
	PageResponse PageResponse
}, error) {
	return _IErc20.Contract.TokenPairs(&_IErc20.CallOpts, pagination)
}

// TokenPairs is a free data retrieval call binding the contract method 0xff90f0c1.
//
// Solidity: function tokenPairs((bytes,uint64,uint64,bool,bool) pagination) view returns((address,string,bool,uint8)[] tokenPairs, (bytes,uint64) pageResponse)
func (_IErc20 *IErc20CallerSession) TokenPairs(pagination PageRequest) (struct {
	TokenPairs   []TokenPair
	PageResponse PageResponse
}, error) {
	return _IErc20.Contract.TokenPairs(&_IErc20.CallOpts, pagination)
}

// ConvertCoin is a paid mutator transaction binding the contract method 0x77eb729f.
//
// Solidity: function convertCoin((string,uint256) coin, address receiver) returns(bool)
func (_IErc20 *IErc20Transactor) ConvertCoin(opts *bind.TransactOpts, coin Coin, receiver common.Address) (*types.Transaction, error) {
	return _IErc20.contract.Transact(opts, "convertCoin", coin, receiver)
}

// ConvertCoin is a paid mutator transaction binding the contract method 0x77eb729f.
//
// Solidity: function convertCoin((string,uint256) coin, address receiver) returns(bool)
func (_IErc20 *IErc20Session) ConvertCoin(coin Coin, receiver common.Address) (*types.Transaction, error) {
	return _IErc20.Contract.ConvertCoin(&_IErc20.TransactOpts, coin, receiver)
}

// ConvertCoin is a paid mutator transaction binding the contract method 0x77eb729f.
//
// Solidity: function convertCoin((string,uint256) coin, address receiver) returns(bool)
func (_IErc20 *IErc20TransactorSession) ConvertCoin(coin Coin, receiver common.Address) (*types.Transaction, error) {
	return _IErc20.Contract.ConvertCoin(&_IErc20.TransactOpts, coin, receiver)
}

// ConvertERC20 is a paid mutator transaction binding the contract method 0xffcfa891.
//
// Solidity: function convertERC20(address contractAddress, uint256 amount, address receiver) returns(bool)
func (_IErc20 *IErc20Transactor) ConvertERC20(opts *bind.TransactOpts, contractAddress common.Address, amount *big.Int, receiver common.Address) (*types.Transaction, error) {
	return _IErc20.contract.Transact(opts, "convertERC20", contractAddress, amount, receiver)
}

// ConvertERC20 is a paid mutator transaction binding the contract method 0xffcfa891.
//
// Solidity: function convertERC20(address contractAddress, uint256 amount, address receiver) returns(bool)
func (_IErc20 *IErc20Session) ConvertERC20(contractAddress common.Address, amount *big.Int, receiver common.Address) (*types.Transaction, error) {
	return _IErc20.Contract.ConvertERC20(&_IErc20.TransactOpts, contractAddress, amount, receiver)
}

// ConvertERC20 is a paid mutator transaction binding the contract method 0xffcfa891.
//
// Solidity: function convertERC20(address contractAddress, uint256 amount, address receiver) returns(bool)
func (_IErc20 *IErc20TransactorSession) ConvertERC20(contractAddress common.Address, amount *big.Int, receiver common.Address) (*types.Transaction, error) {
	return _IErc20.Contract.ConvertERC20(&_IErc20.TransactOpts, contractAddress, amount, receiver)
}

// IErc20ConvertCoinIterator is returned from FilterConvertCoin and is used to iterate over the raw logs and unpacked data for ConvertCoin events raised by the IErc20 contract.
type IErc20ConvertCoinIterator struct {
	Event *IErc20ConvertCoin // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IErc20ConvertCoinIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IErc20ConvertCoin)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IErc20ConvertCoin)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IErc20ConvertCoinIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IErc20ConvertCoinIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IErc20ConvertCoin represents a ConvertCoin event raised by the IErc20 contract.
type IErc20ConvertCoin struct {
	Receiver common.Address
	Sender   common.Address
	Denom    string
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterConvertCoin is a free log retrieval operation binding the contract event 0x544c5d08d597686f469b81ae54adb72e99e3481537693ac64429b8311fb86627.
//
// Solidity: event ConvertCoin(address indexed receiver, address indexed sender, string denom, uint256 amount)
func (_IErc20 *IErc20Filterer) FilterConvertCoin(opts *bind.FilterOpts, receiver []common.Address, sender []common.Address) (*IErc20ConvertCoinIterator, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IErc20.contract.FilterLogs(opts, "ConvertCoin", receiverRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &IErc20ConvertCoinIterator{contract: _IErc20.contract, event: "ConvertCoin", logs: logs, sub: sub}, nil
}

// WatchConvertCoin is a free log subscription operation binding the contract event 0x544c5d08d597686f469b81ae54adb72e99e3481537693ac64429b8311fb86627.
//
// Solidity: event ConvertCoin(address indexed receiver, address indexed sender, string denom, uint256 amount)
func (_IErc20 *IErc20Filterer) WatchConvertCoin(opts *bind.WatchOpts, sink chan<- *IErc20ConvertCoin, receiver []common.Address, sender []common.Address) (event.Subscription, error) {

	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IErc20.contract.WatchLogs(opts, "ConvertCoin", receiverRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IErc20ConvertCoin)
				if err := _IErc20.contract.UnpackLog(event, "ConvertCoin", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseConvertCoin is a log parse operation binding the contract event 0x544c5d08d597686f469b81ae54adb72e99e3481537693ac64429b8311fb86627.
//
// Solidity: event ConvertCoin(address indexed receiver, address indexed sender, string denom, uint256 amount)
func (_IErc20 *IErc20Filterer) ParseConvertCoin(log types.Log) (*IErc20ConvertCoin, error) {
	event := new(IErc20ConvertCoin)
	if err := _IErc20.contract.UnpackLog(event, "ConvertCoin", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IErc20ConvertERC20Iterator is returned from FilterConvertERC20 and is used to iterate over the raw logs and unpacked data for ConvertERC20 events raised by the IErc20 contract.
type IErc20ConvertERC20Iterator struct {
	Event *IErc20ConvertERC20 // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *IErc20ConvertERC20Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IErc20ConvertERC20)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(IErc20ConvertERC20)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *IErc20ConvertERC20Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IErc20ConvertERC20Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IErc20ConvertERC20 represents a ConvertERC20 event raised by the IErc20 contract.
type IErc20ConvertERC20 struct {
	ContractAddress common.Address
	Receiver        common.Address
	Sender          common.Address
	Amount          *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterConvertERC20 is a free log retrieval operation binding the contract event 0xf2726fac06f94139129291aa2e9aabaf965bbc3c64503ca29ce3163f44cd0daa.
//
// Solidity: event ConvertERC20(address indexed contractAddress, address indexed receiver, address indexed sender, uint256 amount)
func (_IErc20 *IErc20Filterer) FilterConvertERC20(opts *bind.FilterOpts, contractAddress []common.Address, receiver []common.Address, sender []common.Address) (*IErc20ConvertERC20Iterator, error) {

	var contractAddressRule []interface{}
	for _, contractAddressItem := range contractAddress {
		contractAddressRule = append(contractAddressRule, contractAddressItem)
	}
	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IErc20.contract.FilterLogs(opts, "ConvertERC20", contractAddressRule, receiverRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &IErc20ConvertERC20Iterator{contract: _IErc20.contract, event: "ConvertERC20", logs: logs, sub: sub}, nil
}

// WatchConvertERC20 is a free log subscription operation binding the contract event 0xf2726fac06f94139129291aa2e9aabaf965bbc3c64503ca29ce3163f44cd0daa.
//
// Solidity: event ConvertERC20(address indexed contractAddress, address indexed receiver, address indexed sender, uint256 amount)
func (_IErc20 *IErc20Filterer) WatchConvertERC20(opts *bind.WatchOpts, sink chan<- *IErc20ConvertERC20, contractAddress []common.Address, receiver []common.Address, sender []common.Address) (event.Subscription, error) {

	var contractAddressRule []interface{}
	for _, contractAddressItem := range contractAddress {
		contractAddressRule = append(contractAddressRule, contractAddressItem)
	}
	var receiverRule []interface{}
	for _, receiverItem := range receiver {
		receiverRule = append(receiverRule, receiverItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _IErc20.contract.WatchLogs(opts, "ConvertERC20", contractAddressRule, receiverRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IErc20ConvertERC20)
				if err := _IErc20.contract.UnpackLog(event, "ConvertERC20", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseConvertERC20 is a log parse operation binding the contract event 0xf2726fac06f94139129291aa2e9aabaf965bbc3c64503ca29ce3163f44cd0daa.
//
// Solidity: event ConvertERC20(address indexed contractAddress, address indexed receiver, address indexed sender, uint256 amount)
func (_IErc20 *IErc20Filterer) ParseConvertERC20(log types.Log) (*IErc20ConvertERC20, error) {
	event := new(IErc20ConvertERC20)
	if err := _IErc20.contract.UnpackLog(event, "ConvertERC20", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
