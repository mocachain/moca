package types

import (
	"fmt"
	"math/big"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

var _ codectypes.UnpackInterfacesMessage = (*MsgEthereumTx)(nil)

// TxData is the Ethereum transaction a MsgEthereumTx carries.
type TxData interface {
	proto.Message
	ethTxData() (ethtypes.TxData, error)
}

// UnpackInterfaces implements codectypes.UnpackInterfacesMessage.
func (m *MsgEthereumTx) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	var data TxData
	return unpacker.UnpackAny(m.Data, &data)
}

// ToMsgEthereumTx returns m as a cosmos/evm message. It rebuilds the signed
// transaction from m's fields and checks that it hashes to the recorded hash.
func (m *MsgEthereumTx) ToMsgEthereumTx() (*evmtypes.MsgEthereumTx, error) {
	data, ok := m.Data.GetCachedValue().(TxData)
	if !ok {
		return nil, fmt.Errorf("transaction %s: data is not unpacked", m.Hash)
	}
	inner, err := data.ethTxData()
	if err != nil {
		return nil, fmt.Errorf("transaction %s: %w", m.Hash, err)
	}
	tx := ethtypes.NewTx(inner)
	if tx.Hash() != common.HexToHash(m.Hash) {
		return nil, fmt.Errorf("transaction %s: rebuilt transaction hashes to %s", m.Hash, tx.Hash())
	}

	var from common.Address
	if m.From != "" {
		if !common.IsHexAddress(m.From) {
			return nil, fmt.Errorf("transaction %s: invalid sender %q", m.Hash, m.From)
		}
		from = common.HexToAddress(m.From)
	} else if from, err = ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), tx); err != nil {
		return nil, fmt.Errorf("transaction %s: %w", m.Hash, err)
	}
	return &evmtypes.MsgEthereumTx{From: from.Bytes(), Raw: evmtypes.EthereumTx{Transaction: tx}}, nil
}

func (tx *LegacyTx) ethTxData() (ethtypes.TxData, error) {
	ints, err := parseInts(tx.GasPrice, tx.Value)
	if err != nil {
		return nil, err
	}
	to, err := parseTo(tx.To)
	if err != nil {
		return nil, err
	}
	return &ethtypes.LegacyTx{
		Nonce:    tx.Nonce,
		GasPrice: ints[0],
		Gas:      tx.Gas,
		To:       to,
		Value:    ints[1],
		Data:     tx.Data,
		V:        new(big.Int).SetBytes(tx.V),
		R:        new(big.Int).SetBytes(tx.R),
		S:        new(big.Int).SetBytes(tx.S),
	}, nil
}

func (tx *AccessListTx) ethTxData() (ethtypes.TxData, error) {
	ints, err := parseInts(tx.ChainId, tx.GasPrice, tx.Value)
	if err != nil {
		return nil, err
	}
	to, err := parseTo(tx.To)
	if err != nil {
		return nil, err
	}
	return &ethtypes.AccessListTx{
		ChainID:    ints[0],
		Nonce:      tx.Nonce,
		GasPrice:   ints[1],
		Gas:        tx.Gas,
		To:         to,
		Value:      ints[2],
		Data:       tx.Data,
		AccessList: accessList(tx.Accesses),
		V:          new(big.Int).SetBytes(tx.V),
		R:          new(big.Int).SetBytes(tx.R),
		S:          new(big.Int).SetBytes(tx.S),
	}, nil
}

func (tx *DynamicFeeTx) ethTxData() (ethtypes.TxData, error) {
	ints, err := parseInts(tx.ChainId, tx.GasTipCap, tx.GasFeeCap, tx.Value)
	if err != nil {
		return nil, err
	}
	to, err := parseTo(tx.To)
	if err != nil {
		return nil, err
	}
	return &ethtypes.DynamicFeeTx{
		ChainID:    ints[0],
		Nonce:      tx.Nonce,
		GasTipCap:  ints[1],
		GasFeeCap:  ints[2],
		Gas:        tx.Gas,
		To:         to,
		Value:      ints[3],
		Data:       tx.Data,
		AccessList: accessList(tx.Accesses),
		V:          new(big.Int).SetBytes(tx.V),
		R:          new(big.Int).SetBytes(tx.R),
		S:          new(big.Int).SetBytes(tx.S),
	}, nil
}

// parseInts parses decimal integers; an empty string is zero.
func parseInts(values ...string) ([]*big.Int, error) {
	ints := make([]*big.Int, len(values))
	for i, v := range values {
		ints[i] = new(big.Int)
		if v == "" {
			continue
		}
		if _, ok := ints[i].SetString(v, 10); !ok {
			return nil, fmt.Errorf("invalid integer %q", v)
		}
	}
	return ints, nil
}

// parseTo parses a recipient address; an empty string is a contract creation.
func parseTo(to string) (*common.Address, error) {
	if to == "" {
		return nil, nil
	}
	if !common.IsHexAddress(to) {
		return nil, fmt.Errorf("invalid recipient %q", to)
	}
	addr := common.HexToAddress(to)
	return &addr, nil
}

func accessList(tuples []AccessTuple) ethtypes.AccessList {
	if len(tuples) == 0 {
		return nil
	}
	list := make(ethtypes.AccessList, len(tuples))
	for i, t := range tuples {
		keys := make([]common.Hash, len(t.StorageKeys))
		for j, k := range t.StorageKeys {
			keys[j] = common.HexToHash(k)
		}
		list[i] = ethtypes.AccessTuple{Address: common.HexToAddress(t.Address), StorageKeys: keys}
	}
	return list
}
