package keyring

import (
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cosmosLedger "github.com/cosmos/cosmos-sdk/crypto/ledger"
	"github.com/cosmos/cosmos-sdk/crypto/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	"github.com/mocachain/moca/v2/wallets/ledger"
)

// AppName defines the Ledger app used for signing. Moca uses the Ethereum app
const AppName = "Ethereum"

var (
	// SupportedAlgorithms defines the list of signing algorithms used on Moca:
	//  - eth_secp256k1 (Ethereum)
	//  - eth_bls (Ethereum)
	SupportedAlgorithms = keyring.SigningAlgoList{hd.EthSecp256k1, hd.EthBLS}
	// SupportedAlgorithmsLedger defines the list of signing algorithms used on Moca for the Ledger device:
	//  - eth_secp256k1 (Ethereum)
	//  - eth_bls (Ethereum)
	// The Ledger derivation function is responsible for all signing and address generation.
	SupportedAlgorithmsLedger = keyring.SigningAlgoList{hd.EthSecp256k1, hd.EthBLS}
	// LedgerDerivation defines the Moca Ledger Go derivation (Ethereum app with EIP-712 signing)
	LedgerDerivation = ledger.MocaLedgerDerivation()
	// CreatePubkey uses the ethsecp256k1 pubkey with Ethereum address generation and keccak hashing
	CreatePubkey = func(key []byte) types.PubKey { return &ethsecp256k1.PubKey{Key: key} }
	// SkipDERConversion represents whether the signed Ledger output should skip conversion from DER to BER.
	// This is set to true for signing performed by the Ledger Ethereum app.
	SkipDERConversion = true
)

// EthSecp256k1Option defines a function keys options for the ethereum Secp256k1 curve.
// It supports eth_secp256k1, eth_bls keys for accounts.
func Option() keyring.Option {
	return func(options *keyring.Options) {
		options.SupportedAlgos = SupportedAlgorithms
		options.SupportedAlgosLedger = SupportedAlgorithmsLedger
		options.LedgerDerivation = func() (cosmosLedger.SECP256K1, error) { return LedgerDerivation() }
		options.LedgerCreateKey = CreatePubkey
		options.LedgerAppName = AppName
		options.LedgerSigSkipDERConv = SkipDERConversion
	}
}
