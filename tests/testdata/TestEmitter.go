// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package testdata

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

// TestEmitterMetaData contains all meta data concerning the TestEmitter contract.
var TestEmitterMetaData = &bind.MetaData{
	ABI: "[{\"constant\":false,\"inputs\":[{\"name\":\"id\",\"type\":\"uint256\"},{\"name\":\"data\",\"type\":\"string\"}],\"name\":\"emitEvent\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"startId\",\"type\":\"uint256\"},{\"name\":\"count\",\"type\":\"uint256\"},{\"name\":\"data\",\"type\":\"string\"}],\"name\":\"emitMultipleEvents\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":true,\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"data\",\"type\":\"string\"}],\"name\":\"TestEvent\",\"type\":\"event\"}]",
	Bin: "0x608060405234801561001057600080fd5b5061038a806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806352925e461461003b5780635447e6a014610100575b600080fd5b6100fe6004803603604081101561005157600080fd5b81019080803590602001909291908035906020019064010000000081111561007857600080fd5b82018360208201111561008a57600080fd5b803590602001918460018302840111640100000000831117156100ac57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192905050506101cf565b005b6101cd6004803603606081101561011657600080fd5b8101908080359060200190929190803590602001909291908035906020019064010000000081111561014757600080fd5b82018360208201111561015957600080fd5b8035906020019184600183028401116401000000008311171561017b57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290505050610287565b005b3373ffffffffffffffffffffffffffffffffffffffff16827f09f09c482a293eae240f90f0a4c7ae23ba44da9a1c7965aa0a3e30472cbca237836040518080602001828103825283818151815260200191508051906020019080838360005b8381101561024957808201518184015260208101905061022e565b50505050905090810190601f1680156102765780820380516001836020036101000a031916815260200191505b509250505060405180910390a35050565b60008090505b82811015610358573373ffffffffffffffffffffffffffffffffffffffff168185017f09f09c482a293eae240f90f0a4c7ae23ba44da9a1c7965aa0a3e30472cbca237846040518080602001828103825283818151815260200191508051906020019080838360005b838110156103115780820151818401526020810190506102f6565b50505050905090810190601f16801561033e5780820380516001836020036101000a031916815260200191505b509250505060405180910390a3808060010191505061028d565b5050505056fea165627a7a723058203eeb6001009d4cc5b3da2241b5bf6732bc46719768401a85afc7772c2eeadb540029",
}

// TestEmitterABI is the input ABI used to generate the binding from.
// Deprecated: Use TestEmitterMetaData.ABI instead.
var TestEmitterABI = TestEmitterMetaData.ABI

// TestEmitterBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use TestEmitterMetaData.Bin instead.
var TestEmitterBin = TestEmitterMetaData.Bin

// DeployTestEmitter deploys a new Ethereum contract, binding an instance of TestEmitter to it.
func DeployTestEmitter(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *TestEmitter, error) {
	parsed, err := TestEmitterMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(TestEmitterBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &TestEmitter{TestEmitterCaller: TestEmitterCaller{contract: contract}, TestEmitterTransactor: TestEmitterTransactor{contract: contract}, TestEmitterFilterer: TestEmitterFilterer{contract: contract}}, nil
}

// TestEmitter is an auto generated Go binding around an Ethereum contract.
type TestEmitter struct {
	TestEmitterCaller     // Read-only binding to the contract
	TestEmitterTransactor // Write-only binding to the contract
	TestEmitterFilterer   // Log filterer for contract events
}

// TestEmitterCaller is an auto generated read-only Go binding around an Ethereum contract.
type TestEmitterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestEmitterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TestEmitterTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestEmitterFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TestEmitterFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestEmitterSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TestEmitterSession struct {
	Contract     *TestEmitter      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TestEmitterCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TestEmitterCallerSession struct {
	Contract *TestEmitterCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// TestEmitterTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TestEmitterTransactorSession struct {
	Contract     *TestEmitterTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// TestEmitterRaw is an auto generated low-level Go binding around an Ethereum contract.
type TestEmitterRaw struct {
	Contract *TestEmitter // Generic contract binding to access the raw methods on
}

// TestEmitterCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TestEmitterCallerRaw struct {
	Contract *TestEmitterCaller // Generic read-only contract binding to access the raw methods on
}

// TestEmitterTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TestEmitterTransactorRaw struct {
	Contract *TestEmitterTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTestEmitter creates a new instance of TestEmitter, bound to a specific deployed contract.
func NewTestEmitter(address common.Address, backend bind.ContractBackend) (*TestEmitter, error) {
	contract, err := bindTestEmitter(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TestEmitter{TestEmitterCaller: TestEmitterCaller{contract: contract}, TestEmitterTransactor: TestEmitterTransactor{contract: contract}, TestEmitterFilterer: TestEmitterFilterer{contract: contract}}, nil
}

// NewTestEmitterCaller creates a new read-only instance of TestEmitter, bound to a specific deployed contract.
func NewTestEmitterCaller(address common.Address, caller bind.ContractCaller) (*TestEmitterCaller, error) {
	contract, err := bindTestEmitter(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TestEmitterCaller{contract: contract}, nil
}

// NewTestEmitterTransactor creates a new write-only instance of TestEmitter, bound to a specific deployed contract.
func NewTestEmitterTransactor(address common.Address, transactor bind.ContractTransactor) (*TestEmitterTransactor, error) {
	contract, err := bindTestEmitter(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TestEmitterTransactor{contract: contract}, nil
}

// NewTestEmitterFilterer creates a new log filterer instance of TestEmitter, bound to a specific deployed contract.
func NewTestEmitterFilterer(address common.Address, filterer bind.ContractFilterer) (*TestEmitterFilterer, error) {
	contract, err := bindTestEmitter(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TestEmitterFilterer{contract: contract}, nil
}

// bindTestEmitter binds a generic wrapper to an already deployed contract.
func bindTestEmitter(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := TestEmitterMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TestEmitter *TestEmitterRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TestEmitter.Contract.TestEmitterCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TestEmitter *TestEmitterRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TestEmitter.Contract.TestEmitterTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TestEmitter *TestEmitterRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TestEmitter.Contract.TestEmitterTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TestEmitter *TestEmitterCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TestEmitter.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TestEmitter *TestEmitterTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TestEmitter.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TestEmitter *TestEmitterTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TestEmitter.Contract.contract.Transact(opts, method, params...)
}

// EmitEvent is a paid mutator transaction binding the contract method 0x52925e46.
//
// Solidity: function emitEvent(uint256 id, string data) returns()
func (_TestEmitter *TestEmitterTransactor) EmitEvent(opts *bind.TransactOpts, id *big.Int, data string) (*types.Transaction, error) {
	return _TestEmitter.contract.Transact(opts, "emitEvent", id, data)
}

// EmitEvent is a paid mutator transaction binding the contract method 0x52925e46.
//
// Solidity: function emitEvent(uint256 id, string data) returns()
func (_TestEmitter *TestEmitterSession) EmitEvent(id *big.Int, data string) (*types.Transaction, error) {
	return _TestEmitter.Contract.EmitEvent(&_TestEmitter.TransactOpts, id, data)
}

// EmitEvent is a paid mutator transaction binding the contract method 0x52925e46.
//
// Solidity: function emitEvent(uint256 id, string data) returns()
func (_TestEmitter *TestEmitterTransactorSession) EmitEvent(id *big.Int, data string) (*types.Transaction, error) {
	return _TestEmitter.Contract.EmitEvent(&_TestEmitter.TransactOpts, id, data)
}

// EmitMultipleEvents is a paid mutator transaction binding the contract method 0x5447e6a0.
//
// Solidity: function emitMultipleEvents(uint256 startId, uint256 count, string data) returns()
func (_TestEmitter *TestEmitterTransactor) EmitMultipleEvents(opts *bind.TransactOpts, startId *big.Int, count *big.Int, data string) (*types.Transaction, error) {
	return _TestEmitter.contract.Transact(opts, "emitMultipleEvents", startId, count, data)
}

// EmitMultipleEvents is a paid mutator transaction binding the contract method 0x5447e6a0.
//
// Solidity: function emitMultipleEvents(uint256 startId, uint256 count, string data) returns()
func (_TestEmitter *TestEmitterSession) EmitMultipleEvents(startId *big.Int, count *big.Int, data string) (*types.Transaction, error) {
	return _TestEmitter.Contract.EmitMultipleEvents(&_TestEmitter.TransactOpts, startId, count, data)
}

// EmitMultipleEvents is a paid mutator transaction binding the contract method 0x5447e6a0.
//
// Solidity: function emitMultipleEvents(uint256 startId, uint256 count, string data) returns()
func (_TestEmitter *TestEmitterTransactorSession) EmitMultipleEvents(startId *big.Int, count *big.Int, data string) (*types.Transaction, error) {
	return _TestEmitter.Contract.EmitMultipleEvents(&_TestEmitter.TransactOpts, startId, count, data)
}

// TestEmitterTestEventIterator is returned from FilterTestEvent and is used to iterate over the raw logs and unpacked data for TestEvent events raised by the TestEmitter contract.
type TestEmitterTestEventIterator struct {
	Event *TestEmitterTestEvent // Event containing the contract specifics and raw log

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
func (it *TestEmitterTestEventIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TestEmitterTestEvent)
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
		it.Event = new(TestEmitterTestEvent)
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
func (it *TestEmitterTestEventIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TestEmitterTestEventIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TestEmitterTestEvent represents a TestEvent event raised by the TestEmitter contract.
type TestEmitterTestEvent struct {
	Id     *big.Int
	Sender common.Address
	Data   string
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterTestEvent is a free log retrieval operation binding the contract event 0x09f09c482a293eae240f90f0a4c7ae23ba44da9a1c7965aa0a3e30472cbca237.
//
// Solidity: event TestEvent(uint256 indexed id, address indexed sender, string data)
func (_TestEmitter *TestEmitterFilterer) FilterTestEvent(opts *bind.FilterOpts, id []*big.Int, sender []common.Address) (*TestEmitterTestEventIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TestEmitter.contract.FilterLogs(opts, "TestEvent", idRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &TestEmitterTestEventIterator{contract: _TestEmitter.contract, event: "TestEvent", logs: logs, sub: sub}, nil
}

// WatchTestEvent is a free log subscription operation binding the contract event 0x09f09c482a293eae240f90f0a4c7ae23ba44da9a1c7965aa0a3e30472cbca237.
//
// Solidity: event TestEvent(uint256 indexed id, address indexed sender, string data)
func (_TestEmitter *TestEmitterFilterer) WatchTestEvent(opts *bind.WatchOpts, sink chan<- *TestEmitterTestEvent, id []*big.Int, sender []common.Address) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _TestEmitter.contract.WatchLogs(opts, "TestEvent", idRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TestEmitterTestEvent)
				if err := _TestEmitter.contract.UnpackLog(event, "TestEvent", log); err != nil {
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

// ParseTestEvent is a log parse operation binding the contract event 0x09f09c482a293eae240f90f0a4c7ae23ba44da9a1c7965aa0a3e30472cbca237.
//
// Solidity: event TestEvent(uint256 indexed id, address indexed sender, string data)
func (_TestEmitter *TestEmitterFilterer) ParseTestEvent(log types.Log) (*TestEmitterTestEvent, error) {
	event := new(TestEmitterTestEvent)
	if err := _TestEmitter.contract.UnpackLog(event, "TestEvent", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
