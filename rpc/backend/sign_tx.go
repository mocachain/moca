package backend

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// SendTransaction is temporarily disabled during the cosmos/evm v0.6.0
// migration. The pre-migration path depended on moca's old MsgEthereumTx
// helpers (Sign / ValidateBasic / BuildTx); cosmos/evm v0.6.0 no longer
// exposes those methods because TransactionArgs.ToTransaction returns a
// raw *ethtypes.Transaction. Re-implementing the flow requires switching
// to cosmos/evm's keystore-driven signing path (mirror of
// cosmos/evm@v0.6.0 rpc/backend/sign_tx.go::SendTransaction).
//
// TODO(cosmos-evm migration): port that flow. Until then,
// eth_sendRawTransaction remains the supported submission path.
func (b *Backend) SendTransaction(args evmtypes.TransactionArgs) (common.Hash, error) {
	_ = args
	return common.Hash{}, errors.New("eth_sendTransaction is temporarily disabled during cosmos/evm v0.6.0 migration; use eth_sendRawTransaction with client-side signing")
}

// Sign signs the provided data using the private key of address via Geth's signature standard.
func (b *Backend) Sign(address common.Address, data hexutil.Bytes) (hexutil.Bytes, error) {
	from := sdk.AccAddress(address.Bytes())

	_, err := b.clientCtx.Keyring.KeyByAddress(from)
	if err != nil {
		b.logger.Error("failed to find key in keyring", "address", address.String())
		return nil, fmt.Errorf("%s; %s", keystore.ErrNoMatch, err.Error())
	}

	// Sign the requested hash with the wallet
	signature, _, err := b.clientCtx.Keyring.SignByAddress(from, data, signingtypes.SignMode_SIGN_MODE_TEXTUAL)
	if err != nil {
		b.logger.Error("keyring.SignByAddress failed", "address", address.Hex())
		return nil, err
	}

	signature[crypto.RecoveryIDOffset] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	return signature, nil
}

// SignTypedData signs EIP-712 conformant typed data
func (b *Backend) SignTypedData(address common.Address, typedData apitypes.TypedData) (hexutil.Bytes, error) {
	from := sdk.AccAddress(address.Bytes())

	_, err := b.clientCtx.Keyring.KeyByAddress(from)
	if err != nil {
		b.logger.Error("failed to find key in keyring", "address", address.String())
		return nil, fmt.Errorf("%s; %s", keystore.ErrNoMatch, err.Error())
	}

	sigHash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, err
	}

	// Sign the requested hash with the wallet
	signature, _, err := b.clientCtx.Keyring.SignByAddress(from, sigHash, signingtypes.SignMode_SIGN_MODE_TEXTUAL)
	if err != nil {
		b.logger.Error("keyring.SignByAddress failed", "address", address.Hex())
		return nil, err
	}

	signature[crypto.RecoveryIDOffset] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	return signature, nil
}
