package server

import (
	"encoding/hex"
	"testing"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/crypto"

	mocakr "github.com/mocachain/moca/v2/crypto/keyring"
	"github.com/mocachain/moca/v2/encoding"
)

// newPersonalTestAPI builds the override service against an in-memory keyring
// wired exactly like mocad's (fork eth_secp256k1 algo + moca codec).
func newPersonalTestAPI(t *testing.T) (*personalForkAPI, keyring.Keyring) {
	t.Helper()
	encCfg := encoding.MakeConfig()
	kr := keyring.NewInMemory(encCfg.Codec, mocakr.Option())
	clientCtx := client.Context{}.WithKeyring(kr).WithCodec(encCfg.Codec)
	return newPersonalForkAPI(log.NewNopLogger(), clientCtx), kr
}

// TestPersonalImportRawKey guards the regression where the personal namespace
// created keyring records with cosmos/evm's key types, which moca's fork
// codecs cannot read back: the imported record must round-trip through the
// keyring (KeyByAddress + List), and re-importing must hit the early-return
// path instead of duplicating.
func TestPersonalImportRawKey(t *testing.T) {
	api, kr := newPersonalTestAPI(t)

	priv, err := crypto.GenerateKey()
	require.NoError(t, err)
	privHex := hex.EncodeToString(crypto.FromECDSA(priv))

	addr, err := api.ImportRawKey(privHex, "password")
	require.NoError(t, err)
	require.Equal(t, crypto.PubkeyToAddress(priv.PublicKey), addr, "imported address must be the keccak eth address")

	// The record must be readable back through the fork keyring.
	rec, err := kr.KeyByAddress(sdk.AccAddress(addr.Bytes()))
	require.NoError(t, err, "imported key must round-trip through the keyring")
	pub, err := rec.GetPubKey()
	require.NoError(t, err, "record pubkey must unpack with moca's codecs")
	require.Equal(t, addr.Bytes(), pub.Address().Bytes())

	list, err := kr.List()
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Second import of the same key: early-return with the same address, no
	// duplicate record (exercises the KeyByAddress readback path).
	addr2, err := api.ImportRawKey(privHex, "password")
	require.NoError(t, err)
	require.Equal(t, addr, addr2)
	list, err = kr.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
}

// TestPersonalNewAccount guards the sibling regression: the generated record
// must be listable and its pubkey unpackable with moca's codecs (a cosmos/evm
// typed record would be silently skipped by the fork keyring's migration).
func TestPersonalNewAccount(t *testing.T) {
	api, kr := newPersonalTestAPI(t)

	addr, err := api.NewAccount("password")
	require.NoError(t, err)
	require.NotEqual(t, [20]byte{}, [20]byte(addr))

	list, err := kr.List()
	require.NoError(t, err)
	require.Len(t, list, 1, "new account must be visible to the fork keyring")

	pub, err := list[0].GetPubKey()
	require.NoError(t, err, "record pubkey must unpack with moca's codecs")
	require.Equal(t, addr.Bytes(), pub.Address().Bytes())

	rec, err := kr.KeyByAddress(sdk.AccAddress(addr.Bytes()))
	require.NoError(t, err)
	require.NotEmpty(t, rec.Name)
}
