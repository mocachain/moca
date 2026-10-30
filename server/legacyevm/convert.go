// Package legacyevm lets the node's query services read the Ethereum transactions
// recorded before the v2.0.0 upgrade.
//
// Before v2.0.0 moca ran the in-tree ethermint EVM module. It recorded an Ethereum
// transaction as /ethermint.evm.v1.MsgEthereumTx, whose fields were a LegacyTx,
// AccessListTx or DynamicFeeTx in an Any, the transaction hash and the sender.
// cosmos/evm's /cosmos.evm.vm.v1.MsgEthereumTx reserves those field numbers and
// carries the signed transaction as raw bytes, so the old messages cannot be read by
// mapping the type URL alone. ConvertTx rebuilds the signed transaction from the old
// fields, checks that it hashes to the recorded hash, and re-encodes the message in the
// cosmos/evm format. The old /ethermint.evm.v1.MsgEthereumTxResponse is wire
// compatible with the cosmos/evm response, so ConvertTxMsgData only changes its type
// URL.
//
// Only the query side uses this (JSON-RPC, the EVM indexer, the gRPC/REST tx service
// and the CLI). The transaction decoder of the state machine stays strict, so a new
// transaction in the old format is still rejected.
package legacyevm

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"google.golang.org/protobuf/encoding/protowire"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const (
	legacyMsgEthereumTxURL         = "/ethermint.evm.v1.MsgEthereumTx"
	legacyMsgEthereumTxResponseURL = "/ethermint.evm.v1.MsgEthereumTxResponse"
	legacyExtensionOptionURL       = "/ethermint.evm.v1.ExtensionOptionsEthereumTx"
	legacyLegacyTxURL              = "/ethermint.evm.v1.LegacyTx"
	legacyAccessListTxURL          = "/ethermint.evm.v1.AccessListTx"
	legacyDynamicFeeTxURL          = "/ethermint.evm.v1.DynamicFeeTx"
)

var (
	msgEthereumTxURL         = "/" + proto.MessageName(&evmtypes.MsgEthereumTx{})
	msgEthereumTxResponseURL = "/" + proto.MessageName(&evmtypes.MsgEthereumTxResponse{})
	extensionOptionURL       = "/" + proto.MessageName(&evmtypes.ExtensionOptionsEthereumTx{})

	// legacyMarker is a substring of every legacy type URL. Bytes without it are
	// returned as they are, without parsing.
	legacyMarker         = []byte("ethermint.evm.v1.")
	legacyResponseMarker = []byte(legacyMsgEthereumTxResponseURL)
)

// Field numbers of the messages ConvertTx walks through.
const (
	txRawBodyBytes               protowire.Number = 1
	txBodyMessages               protowire.Number = 1
	txBodyExtensionOptions       protowire.Number = 1023
	anyTypeURL                   protowire.Number = 1
	anyValue                     protowire.Number = 2
	txMsgDataMsgResponses        protowire.Number = 2
	legacyMsgEthereumTxData      protowire.Number = 1
	legacyMsgEthereumTxHash      protowire.Number = 3
	legacyMsgEthereumTxFrom      protowire.Number = 4
	legacyAccessTupleAddress     protowire.Number = 1
	legacyAccessTupleStorageKeys protowire.Number = 2
)

// ConvertTx returns txBytes, an encoded TxRaw, with every pre-v2 Ethereum message and
// extension option rewritten to the cosmos/evm format, and reports whether it changed
// anything. All other fields keep their bytes.
func ConvertTx(txBytes []byte) ([]byte, bool, error) {
	if !bytes.Contains(txBytes, legacyMarker) {
		return txBytes, false, nil
	}
	return rewriteBytesFields(txBytes, convertTxBody, txRawBodyBytes)
}

// ConvertTxMsgData returns data, an encoded TxMsgData, with the type URL of every
// pre-v2 Ethereum transaction response replaced by the cosmos/evm one. Data that holds
// no such response, or cannot be parsed, is returned unchanged.
func ConvertTxMsgData(data []byte) []byte {
	if !bytes.Contains(data, legacyResponseMarker) {
		return data
	}
	out, changed, err := rewriteBytesFields(data, func(anyBz []byte) ([]byte, bool, error) {
		typeURL, value, err := parseAny(anyBz)
		if err != nil || typeURL != legacyMsgEthereumTxResponseURL {
			return anyBz, false, err
		}
		return encodeAny(msgEthereumTxResponseURL, value), true, nil
	}, txMsgDataMsgResponses)
	if err != nil || !changed {
		return data
	}
	return out
}

func convertTxBody(body []byte) ([]byte, bool, error) {
	return rewriteBytesFields(body, convertAny, txBodyMessages, txBodyExtensionOptions)
}

func convertAny(anyBz []byte) ([]byte, bool, error) {
	typeURL, value, err := parseAny(anyBz)
	if err != nil {
		return nil, false, err
	}
	switch typeURL {
	case legacyMsgEthereumTxURL:
		msg, err := convertMsgEthereumTx(value)
		if err != nil {
			return nil, false, err
		}
		return encodeAny(msgEthereumTxURL, msg), true, nil
	case legacyExtensionOptionURL:
		// Both extension options are empty messages.
		return encodeAny(extensionOptionURL, value), true, nil
	default:
		return anyBz, false, nil
	}
}

// convertMsgEthereumTx turns an encoded /ethermint.evm.v1.MsgEthereumTx into an
// encoded /cosmos.evm.vm.v1.MsgEthereumTx carrying the same signed transaction.
func convertMsgEthereumTx(value []byte) ([]byte, error) {
	var (
		dataAny  []byte
		hash     string
		fromAddr string
	)
	err := rangeFields(value, func(num protowire.Number, typ protowire.Type, _, v []byte) error {
		if typ != protowire.BytesType {
			return nil
		}
		switch num {
		case legacyMsgEthereumTxData:
			dataAny = v
		case legacyMsgEthereumTxHash:
			hash = string(v)
		case legacyMsgEthereumTxFrom:
			fromAddr = string(v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if dataAny == nil {
		return nil, errors.New("legacy MsgEthereumTx has no transaction data")
	}
	typeURL, txData, err := parseAny(dataAny)
	if err != nil {
		return nil, err
	}
	inner, err := legacyTxData(typeURL, txData)
	if err != nil {
		return nil, err
	}
	tx := ethtypes.NewTx(inner)
	if hash != "" && tx.Hash() != common.HexToHash(hash) {
		return nil, fmt.Errorf("legacy MsgEthereumTx rebuilds to %s, recorded hash is %s", tx.Hash().Hex(), hash)
	}

	var from common.Address
	if common.IsHexAddress(fromAddr) {
		from = common.HexToAddress(fromAddr)
	} else {
		from, err = ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			return nil, fmt.Errorf("legacy MsgEthereumTx %s: recover sender: %w", tx.Hash().Hex(), err)
		}
	}

	msg := evmtypes.MsgEthereumTx{From: from.Bytes(), Raw: evmtypes.EthereumTx{Transaction: tx}}
	return msg.Marshal()
}

// legacyTxData decodes ethermint's LegacyTx, AccessListTx or DynamicFeeTx. Their
// amounts are decimal strings and their signature values big-endian bytes.
func legacyTxData(typeURL string, bz []byte) (ethtypes.TxData, error) {
	f, err := parseTxFields(bz)
	if err != nil {
		return nil, err
	}
	switch typeURL {
	case legacyLegacyTxURL:
		// nonce=1 gas_price=2 gas=3 to=4 value=5 data=6 v=7 r=8 s=9
		tx := &ethtypes.LegacyTx{Nonce: f.uint(1), Gas: f.uint(3), Data: f.bytes(6)}
		if tx.GasPrice, err = f.int(2); err != nil {
			return nil, err
		}
		if tx.Value, err = f.int(5); err != nil {
			return nil, err
		}
		if tx.To, err = f.to(4); err != nil {
			return nil, err
		}
		tx.V, tx.R, tx.S = f.sig(7), f.sig(8), f.sig(9)
		return tx, nil
	case legacyAccessListTxURL:
		// chain_id=1 nonce=2 gas_price=3 gas=4 to=5 value=6 data=7 accesses=8 v=9 r=10 s=11
		tx := &ethtypes.AccessListTx{Nonce: f.uint(2), Gas: f.uint(4), Data: f.bytes(7)}
		if tx.ChainID, err = f.int(1); err != nil {
			return nil, err
		}
		if tx.GasPrice, err = f.int(3); err != nil {
			return nil, err
		}
		if tx.Value, err = f.int(6); err != nil {
			return nil, err
		}
		if tx.To, err = f.to(5); err != nil {
			return nil, err
		}
		if tx.AccessList, err = f.accessList(8); err != nil {
			return nil, err
		}
		tx.V, tx.R, tx.S = f.sig(9), f.sig(10), f.sig(11)
		return tx, nil
	case legacyDynamicFeeTxURL:
		// chain_id=1 nonce=2 gas_tip_cap=3 gas_fee_cap=4 gas=5 to=6 value=7 data=8
		// accesses=9 v=10 r=11 s=12
		tx := &ethtypes.DynamicFeeTx{Nonce: f.uint(2), Gas: f.uint(5), Data: f.bytes(8)}
		if tx.ChainID, err = f.int(1); err != nil {
			return nil, err
		}
		if tx.GasTipCap, err = f.int(3); err != nil {
			return nil, err
		}
		if tx.GasFeeCap, err = f.int(4); err != nil {
			return nil, err
		}
		if tx.Value, err = f.int(7); err != nil {
			return nil, err
		}
		if tx.To, err = f.to(6); err != nil {
			return nil, err
		}
		if tx.AccessList, err = f.accessList(9); err != nil {
			return nil, err
		}
		tx.V, tx.R, tx.S = f.sig(10), f.sig(11), f.sig(12)
		return tx, nil
	default:
		return nil, fmt.Errorf("unsupported legacy Ethereum transaction type %q", typeURL)
	}
}

// txFields holds the fields of one legacy transaction data message: the last value of
// each varint field and every value of each length-delimited field, in order.
type txFields struct {
	varints map[protowire.Number]uint64
	bytesV  map[protowire.Number][][]byte
}

func parseTxFields(bz []byte) (txFields, error) {
	f := txFields{varints: map[protowire.Number]uint64{}, bytesV: map[protowire.Number][][]byte{}}
	err := rangeFields(bz, func(num protowire.Number, typ protowire.Type, _, value []byte) error {
		switch typ {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(value)
			if n < 0 {
				return protowire.ParseError(n)
			}
			f.varints[num] = v
		case protowire.BytesType:
			f.bytesV[num] = append(f.bytesV[num], value)
		}
		return nil
	})
	return f, err
}

func (f txFields) uint(num protowire.Number) uint64 { return f.varints[num] }

func (f txFields) bytes(num protowire.Number) []byte {
	vs := f.bytesV[num]
	if len(vs) == 0 {
		return nil
	}
	return append([]byte(nil), vs[len(vs)-1]...)
}

// int parses a decimal amount; an absent or empty amount is zero.
func (f txFields) int(num protowire.Number) (*big.Int, error) {
	s := string(f.bytes(num))
	if s == "" {
		return new(big.Int), nil
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount %q in field %d", s, num)
	}
	return v, nil
}

// to parses the recipient; an empty recipient is a contract creation.
func (f txFields) to(num protowire.Number) (*common.Address, error) {
	s := string(f.bytes(num))
	if s == "" {
		return nil, nil
	}
	if !common.IsHexAddress(s) {
		return nil, fmt.Errorf("invalid recipient %q", s)
	}
	addr := common.HexToAddress(s)
	return &addr, nil
}

func (f txFields) sig(num protowire.Number) *big.Int {
	return new(big.Int).SetBytes(f.bytes(num))
}

func (f txFields) accessList(num protowire.Number) (ethtypes.AccessList, error) {
	tuples := f.bytesV[num]
	if len(tuples) == 0 {
		return nil, nil
	}
	list := make(ethtypes.AccessList, 0, len(tuples))
	for _, tuple := range tuples {
		var t ethtypes.AccessTuple
		err := rangeFields(tuple, func(n protowire.Number, typ protowire.Type, _, v []byte) error {
			if typ != protowire.BytesType {
				return nil
			}
			switch n {
			case legacyAccessTupleAddress:
				if !common.IsHexAddress(string(v)) {
					return fmt.Errorf("invalid access list address %q", v)
				}
				t.Address = common.HexToAddress(string(v))
			case legacyAccessTupleStorageKeys:
				t.StorageKeys = append(t.StorageKeys, common.HexToHash(string(v)))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

// rewriteBytesFields applies fn to every value of the length-delimited fields nums of
// the message bz and keeps every other field as it is.
func rewriteBytesFields(bz []byte, fn func([]byte) ([]byte, bool, error), nums ...protowire.Number) ([]byte, bool, error) {
	var (
		out     = make([]byte, 0, len(bz))
		changed bool
	)
	err := rangeFields(bz, func(n protowire.Number, typ protowire.Type, field, value []byte) error {
		if typ == protowire.BytesType && slices.Contains(nums, n) {
			converted, ok, err := fn(value)
			if err != nil {
				return err
			}
			if ok {
				changed = true
				out = protowire.AppendTag(out, n, protowire.BytesType)
				out = protowire.AppendBytes(out, converted)
				return nil
			}
		}
		out = append(out, field...)
		return nil
	})
	if err != nil || !changed {
		return bz, false, err
	}
	return out, true, nil
}

// rangeFields calls fn for every field of the encoded message bz with the field's
// complete encoding and, for varint and length-delimited fields, its value.
func rangeFields(bz []byte, fn func(num protowire.Number, typ protowire.Type, field, value []byte) error) error {
	for len(bz) > 0 {
		num, typ, n := protowire.ConsumeTag(bz)
		if n < 0 {
			return protowire.ParseError(n)
		}
		m := protowire.ConsumeFieldValue(num, typ, bz[n:])
		if m < 0 {
			return protowire.ParseError(m)
		}
		value := bz[n : n+m]
		if typ == protowire.BytesType {
			v, k := protowire.ConsumeBytes(bz[n:])
			if k < 0 {
				return protowire.ParseError(k)
			}
			value = v
		}
		if err := fn(num, typ, bz[:n+m], value); err != nil {
			return err
		}
		bz = bz[n+m:]
	}
	return nil
}

func parseAny(bz []byte) (string, []byte, error) {
	var (
		typeURL string
		value   []byte
	)
	err := rangeFields(bz, func(num protowire.Number, typ protowire.Type, _, v []byte) error {
		if typ != protowire.BytesType {
			return nil
		}
		switch num {
		case anyTypeURL:
			typeURL = string(v)
		case anyValue:
			value = v
		}
		return nil
	})
	return typeURL, value, err
}

func encodeAny(typeURL string, value []byte) []byte {
	out := protowire.AppendTag(nil, anyTypeURL, protowire.BytesType)
	out = protowire.AppendString(out, typeURL)
	if len(value) > 0 {
		out = protowire.AppendTag(out, anyValue, protowire.BytesType)
		out = protowire.AppendBytes(out, value)
	}
	return out
}
