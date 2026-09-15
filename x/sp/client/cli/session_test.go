package cli_test

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/sp/client/cli"
)

// TestCreateTxOpts_InvalidPrivateKey covers CreateTxOpts's first, purely local
// error branch: crypto.HexToECDSA rejecting a malformed hex private key. This
// returns before the function ever touches the (here nil) *ethclient.Client,
// so it needs no live or stubbed EVM RPC.
func TestCreateTxOpts_InvalidPrivateKey(t *testing.T) {
	opts, err := cli.CreateTxOpts(context.Background(), nil, "not-a-valid-hex-key", nil, 100000, 0)
	require.Error(t, err)
	require.Nil(t, opts)
}

// TestCreateSpSession covers session construction: binding an
// IStorageProviderSession never dials out (the contract binding only parses
// the static ABI and stores the address/backend), so a nil *ethclient.Client
// is safe here.
func TestCreateSpSession(t *testing.T) {
	txOpts := bind.TransactOpts{
		From: common.HexToAddress("0x1111111111111111111111111111111111111a"),
	}

	session, err := cli.CreateSpSession(nil, txOpts, "0x0000000000000000000000000000000000002002")
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, session.Contract)
	require.Equal(t, txOpts, session.TransactOpts)
	require.False(t, session.CallOpts.Pending)
}
