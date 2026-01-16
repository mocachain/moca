package app

import (
	"encoding/json"
	"os"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"

	servercfg "github.com/evmos/evmos/v12/server/config"
	evmostypes "github.com/evmos/evmos/v12/types"
	"github.com/evmos/evmos/v12/utils"
)

func TestEvmosExport(t *testing.T) {
	// create public key
	privVal := cmttypes.NewMockPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err, "public key should be created without error")

	// create validator set with single validator
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(utils.BaseDenom, sdkmath.NewInt(100000000000000))),
	}

	db := dbm.NewMemDB()
	chainID := utils.MainnetChainID + "-1"
	app := NewEvmos(
		log.NewLogger(os.Stdout),
		db, nil, true, map[int64]bool{},
		DefaultNodeHome, 0,
		servercfg.NewDefaultAppConfig(evmostypes.AttoEvmos),
		simtestutil.NewAppOptionsWithFlagHome(DefaultNodeHome),
		baseapp.SetChainID(chainID),
	)

	genesisState := app.DefaultGenesis()
	genesisState = GenesisStateWithValSet(app, genesisState, valSet, []authtypes.GenesisAccount{acc}, balance)

	stateBytes, err := json.MarshalIndent(genesisState, "", "  ")
	require.NoError(t, err)

	// Initialize the chain
	_, err = app.InitChain(
		&abci.RequestInitChain{
			ChainId:       chainID,
			Validators:    []abci.ValidatorUpdate{},
			AppStateBytes: stateBytes,
		},
	)
	require.NoError(t, err)

	// In Cosmos SDK v0.50+, we need to call FinalizeBlock before Commit
	// to ensure that the state changes from InitGenesis are written to the underlying store.
	// FinalizeBlock calls workingHash() which in turn calls finalizeBlockState.ms.Write().
	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: 1,
	})
	require.NoError(t, err)

	// Commit to persist the state
	_, err = app.Commit()
	require.NoError(t, err)

	// Making a new app object with the db, so that initchain hasn't been called
	app2 := NewEvmos(
		log.NewLogger(os.Stdout),
		db, nil, true, map[int64]bool{},
		DefaultNodeHome, 0,
		servercfg.NewDefaultAppConfig(evmostypes.AttoEvmos),
		simtestutil.NewAppOptionsWithFlagHome(DefaultNodeHome),
		baseapp.SetChainID(chainID),
	)

	_, err = app2.ExportAppStateAndValidators(false, []string{}, []string{})
	require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
}
