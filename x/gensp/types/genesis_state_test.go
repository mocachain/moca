package types_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cosmossdk.io/math"
	tmtypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/gensp/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// newTestEncodingConfig builds an encoding config with the message types this
// file's fixtures need registered: MsgCreateStorageProvider (the only valid
// GenTx message) and MsgSend (used as a "wrong message type" fixture).
func newTestEncodingConfig() moduletestutil.TestEncodingConfig {
	cfg := moduletestutil.MakeTestEncodingConfig()
	sptypes.RegisterInterfaces(cfg.InterfaceRegistry)
	banktypes.RegisterInterfaces(cfg.InterfaceRegistry)
	return cfg
}

// validCreateSPMsg returns a MsgCreateStorageProvider that passes ValidateBasic.
func validCreateSPMsg(addr sdk.AccAddress, blsKey, blsProof string) *sptypes.MsgCreateStorageProvider {
	return &sptypes.MsgCreateStorageProvider{
		Creator:            addr.String(),
		SpAddress:          addr.String(),
		FundingAddress:     addr.String(),
		SealAddress:        addr.String(),
		ApprovalAddress:    addr.String(),
		GcAddress:          addr.String(),
		MaintenanceAddress: addr.String(),
		Description:        sptypes.NewDescription("moniker", "identity", "website", "details"),
		Endpoint:           "http://127.0.0.1:9033",
		Deposit:            sdk.NewInt64Coin(sptypes.DefaultDepositDenom, 100000),
		ReadPrice:          math.LegacyZeroDec(),
		StorePrice:         math.LegacyZeroDec(),
		BlsKey:             blsKey,
		BlsProof:           blsProof,
	}
}

// mustEncodeTx wraps msgs into an (unsigned) tx and JSON-encodes it, mirroring
// how x/gensp/gentx_test.go builds gentx fixtures. ValidateAndGetGenTx never
// checks signatures (it is stateless validation), so an unsigned tx is enough.
func mustEncodeTx(t *testing.T, cfg moduletestutil.TestEncodingConfig, msgs ...sdk.Msg) json.RawMessage {
	t.Helper()
	txBuilder := cfg.TxConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msgs...))
	bz, err := cfg.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
	require.NoError(t, err)
	return bz
}

func TestNewGenesisState(t *testing.T) {
	t.Run("nil genTxs becomes a non-nil empty slice", func(t *testing.T) {
		gs := types.NewGenesisState(nil)
		require.NotNil(t, gs.GenspTxs)
		require.Empty(t, gs.GenspTxs)
	})

	t.Run("non-nil genTxs are preserved", func(t *testing.T) {
		genTxs := []json.RawMessage{[]byte(`{"a":1}`)}
		gs := types.NewGenesisState(genTxs)
		require.Equal(t, genTxs, gs.GenspTxs)
	})
}

func TestDefaultGenesisState(t *testing.T) {
	gs := types.DefaultGenesisState()
	require.NotNil(t, gs.GenspTxs)
	require.Empty(t, gs.GenspTxs)
}

func TestNewGenesisStateFromTx(t *testing.T) {
	cfg := newTestEncodingConfig()

	t.Run("encodes every tx into GenspTxs", func(t *testing.T) {
		blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()
		msg := validCreateSPMsg(sample.RandAccAddress(), blsKey, blsProof)

		txBuilder := cfg.TxConfig.NewTxBuilder()
		require.NoError(t, txBuilder.SetMsgs(msg))

		gs := types.NewGenesisStateFromTx(cfg.TxConfig.TxJSONEncoder(), []sdk.Tx{txBuilder.GetTx()})
		require.Len(t, gs.GenspTxs, 1)

		decoded, err := cfg.TxConfig.TxJSONDecoder()(gs.GenspTxs[0])
		require.NoError(t, err)
		gotMsgs := decoded.GetMsgs()
		require.Len(t, gotMsgs, 1)
		got, ok := gotMsgs[0].(*sptypes.MsgCreateStorageProvider)
		require.True(t, ok)
		require.Equal(t, msg.SpAddress, got.SpAddress)
		require.Equal(t, msg.Endpoint, got.Endpoint)
	})

	t.Run("a failing encoder panics", func(t *testing.T) {
		failEncoder := func(sdk.Tx) ([]byte, error) { return nil, errors.New("boom") }
		require.Panics(t, func() {
			types.NewGenesisStateFromTx(failEncoder, []sdk.Tx{nil})
		})
	})
}

func TestSetAndGetGenesisStateInAppState(t *testing.T) {
	cfg := newTestEncodingConfig()

	t.Run("round-trips through app state", func(t *testing.T) {
		gs := types.NewGenesisState([]json.RawMessage{[]byte(`{"foo":"bar"}`)})

		appState := types.SetGenesisStateInAppState(cfg.Codec, make(map[string]json.RawMessage), gs)
		require.NotNil(t, appState[types.ModuleName])

		got := types.GetGenesisStateFromAppState(cfg.Codec, appState)
		require.Equal(t, gs.GenspTxs, got.GenspTxs)
	})

	t.Run("missing module entry returns the zero value", func(t *testing.T) {
		got := types.GetGenesisStateFromAppState(cfg.Codec, map[string]json.RawMessage{})
		require.Empty(t, got.GenspTxs)
	})
}

func TestGenesisStateFromGenDoc(t *testing.T) {
	t.Run("valid app state", func(t *testing.T) {
		genDoc := tmtypes.GenesisDoc{AppState: json.RawMessage(`{"gensp":{}}`)}
		gs, err := types.GenesisStateFromGenDoc(genDoc)
		require.NoError(t, err)
		require.Contains(t, gs, types.ModuleName)
	})

	t.Run("malformed app state", func(t *testing.T) {
		genDoc := tmtypes.GenesisDoc{AppState: json.RawMessage(`{invalid`)}
		_, err := types.GenesisStateFromGenDoc(genDoc)
		require.Error(t, err)
	})
}

func TestGenesisStateFromGenFile(t *testing.T) {
	t.Run("file does not exist", func(t *testing.T) {
		_, _, err := types.GenesisStateFromGenFile(filepath.Join(t.TempDir(), "missing.json"))
		require.ErrorContains(t, err, "does not exist")
	})

	t.Run("valid genesis file", func(t *testing.T) {
		genFile := filepath.Join(t.TempDir(), "genesis.json")
		genDoc := tmtypes.GenesisDoc{
			ChainID:  "test-chain",
			AppState: json.RawMessage(`{"gensp":{}}`),
		}
		require.NoError(t, genDoc.SaveAs(genFile))

		gs, doc, err := types.GenesisStateFromGenFile(genFile)
		require.NoError(t, err)
		require.Equal(t, "test-chain", doc.ChainID)
		require.Contains(t, gs, types.ModuleName)
	})

	t.Run("malformed genesis file", func(t *testing.T) {
		genFile := filepath.Join(t.TempDir(), "genesis.json")
		require.NoError(t, os.WriteFile(genFile, []byte("not json"), 0o600))

		_, _, err := types.GenesisStateFromGenFile(genFile)
		require.Error(t, err)
	})
}

func TestValidateAndGetGenTx(t *testing.T) {
	cfg := newTestEncodingConfig()
	decoder := cfg.TxConfig.TxJSONDecoder()

	addr := sample.RandAccAddress()
	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()
	validMsg := validCreateSPMsg(addr, blsKey, blsProof)
	invalidMsg := validCreateSPMsg(addr, blsKey, blsProof)
	invalidMsg.Deposit = sdk.NewInt64Coin(sptypes.DefaultDepositDenom, 0)
	sendMsg := &banktypes.MsgSend{
		FromAddress: addr.String(),
		ToAddress:   addr.String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin(sptypes.DefaultDepositDenom, 1)),
	}

	tests := []struct {
		name    string
		genTx   json.RawMessage
		wantErr string
	}{
		{
			name:    "undecodable json",
			genTx:   json.RawMessage(`{not valid json`),
			wantErr: "failed to decode gentx",
		},
		{
			name:    "zero messages",
			genTx:   mustEncodeTx(t, cfg),
			wantErr: "unexpected number of GenTx messages",
		},
		{
			name:    "two messages",
			genTx:   mustEncodeTx(t, cfg, validMsg, sendMsg),
			wantErr: "unexpected number of GenTx messages",
		},
		{
			name:    "wrong message type",
			genTx:   mustEncodeTx(t, cfg, sendMsg),
			wantErr: "unexpected GenTx message type",
		},
		{
			name:    "message fails ValidateBasic",
			genTx:   mustEncodeTx(t, cfg, invalidMsg),
			wantErr: "invalid GenTx",
		},
		{
			name:  "valid",
			genTx: mustEncodeTx(t, cfg, validMsg),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := types.ValidateAndGetGenTx(tc.genTx, decoder)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, tx.GetMsgs(), 1)
		})
	}
}

func TestValidateGenesis(t *testing.T) {
	cfg := newTestEncodingConfig()
	decoder := cfg.TxConfig.TxJSONDecoder()

	blsKey, blsProof := sample.RandBlsPubKeyAndBlsProof()
	validMsg := validCreateSPMsg(sample.RandAccAddress(), blsKey, blsProof)

	t.Run("empty genesis is valid", func(t *testing.T) {
		require.NoError(t, types.ValidateGenesis(types.DefaultGenesisState(), decoder))
	})

	t.Run("genesis with a valid gentx", func(t *testing.T) {
		gs := types.NewGenesisState([]json.RawMessage{mustEncodeTx(t, cfg, validMsg)})
		require.NoError(t, types.ValidateGenesis(gs, decoder))
	})

	t.Run("propagates the first bad gentx's error", func(t *testing.T) {
		gs := types.NewGenesisState([]json.RawMessage{json.RawMessage(`{not valid json`)})
		err := types.ValidateGenesis(gs, decoder)
		require.ErrorContains(t, err, "failed to decode gentx")
	})
}
