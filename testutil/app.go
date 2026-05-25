package testutil

import (
	"encoding/hex"
	"encoding/json"
	"io"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/encoding"
	"github.com/mocachain/moca/v2/sdk/client/test"
	servercfg "github.com/mocachain/moca/v2/server/config"
	mocatypes "github.com/mocachain/moca/v2/types"
)

func NewTestApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	chainID string,
	options ...func(baseApp *baseapp.BaseApp),
) (*app.Moca, sdktestutil.TestEncodingConfig, error) {
	// create public key
	privVal := mock.NewPV()
	pubKey, _ := privVal.GetPubKey()

	// create validator set with single validator
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})

	// generate genesis account
	bz, _ := hex.DecodeString(test.TestPublicKey)
	faucetPubKey := &ethsecp256k1.PubKey{Key: bz}

	acc := authtypes.NewBaseAccount(faucetPubKey.Address().Bytes(), faucetPubKey, 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(test.TestTokenName, sdkmath.NewInt(100000000000000))),
	}

	encCfg := encoding.MakeConfig()
	options = append(options, baseapp.SetChainID(chainID))
	nApp := app.NewMoca(
		logger,
		db,
		traceStore,
		loadLatest,
		map[int64]bool{},
		app.DefaultNodeHome,
		servercfg.NewDefaultAppConfig(mocatypes.AttoMoca),
		simtestutil.EmptyAppOptions{},
		options...,
	)

	genesisState := app.NewDefaultGenesisState()
	genesisState, _ = simtestutil.GenesisStateWithValSet(nApp.AppCodec(), genesisState, valSet, []authtypes.GenesisAccount{acc}, balance)

	stateBytes, _ := json.MarshalIndent(genesisState, "", "  ")

	// Initialize the chain
	if _, err := nApp.InitChain(
		&abci.RequestInitChain{
			ChainId:       chainID,
			Validators:    []abci.ValidatorUpdate{},
			AppStateBytes: stateBytes,
		},
	); err != nil {
		panic(err)
	}
	nApp.Commit()

	return nApp, encCfg, nil
}
