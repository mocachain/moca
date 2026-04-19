// Copyright 2022 Evmos Foundation
// This file is part of the Evmos Network packages.
//
// Evmos is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Evmos packages are distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Evmos packages. If not, see https://github.com/evmos/evmos/blob/main/LICENSE
package app

import (
	"encoding/json"
	"time"

	"cosmossdk.io/math"
	"cosmossdk.io/simapp"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	tmtypes "github.com/cometbft/cometbft/proto/tendermint/types"
	cmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	servercfg "github.com/evmos/evmos/v12/server/config"
	evmostypes "github.com/evmos/evmos/v12/types"
	"github.com/evmos/evmos/v12/utils"
)

// EthDefaultConsensusParams defines the default Tendermint consensus params used in
// EvmosApp testing.
var EthDefaultConsensusParams = &tmtypes.ConsensusParams{
	Block: &tmtypes.BlockParams{
		MaxBytes: 200000,
		MaxGas:   -1, // no limit
	},
	Evidence: &tmtypes.EvidenceParams{
		MaxAgeNumBlocks: 302400,
		MaxAgeDuration:  504 * time.Hour, // 3 weeks is the max duration
		MaxBytes:        10000,
	},
	Validator: &tmtypes.ValidatorParams{
		PubKeyTypes: []string{
			cmtypes.ABCIPubKeyTypeEd25519,
		},
	},
}

// EthSetup initializes a new EvmosApp. A Nop logger is set in EvmosApp.
func EthSetup(isCheckTx bool, patchGenesis func(*Evmos, simapp.GenesisState) simapp.GenesisState) *Evmos {
	return EthSetupWithDB(isCheckTx, patchGenesis, dbm.NewMemDB())
}

// EthSetupWithDB initializes a new EvmosApp. A Nop logger is set in EvmosApp.
func EthSetupWithDB(isCheckTx bool, patchGenesis func(*Evmos, simapp.GenesisState) simapp.GenesisState, db dbm.DB) *Evmos {
	chainID := utils.TestnetChainID + "-1"

	appOpts := simtestutil.NewAppOptionsWithFlagHome(DefaultNodeHome)

	app := NewEvmos(log.NewNopLogger(),
		db,
		nil,
		true,
		map[int64]bool{},
		DefaultNodeHome,
		servercfg.NewDefaultAppConfig(evmostypes.AttoEvmos),
		appOpts,
		baseapp.SetChainID(chainID),
	)
	if !isCheckTx {
		// init chain must be called to stop deliverState from being nil
		genesisState := NewTestGenesisState(app)
		if patchGenesis != nil {
			genesisState = patchGenesis(app, genesisState)
		}

		stateBytes, err := json.MarshalIndent(genesisState, "", " ")
		if err != nil {
			panic(err)
		}

		// Initialize the chain
		if _, err := app.InitChain(
			&abci.RequestInitChain{
				ChainId:         chainID,
				Validators:      []abci.ValidatorUpdate{},
				ConsensusParams: DefaultConsensusParams,
				AppStateBytes:   stateBytes,
			},
		); err != nil {
			panic(err)
		}
	}

	return app
}

// NewTestGenesisState builds a single-validator genesis suitable for the EVM
// test harness. It delegates to the standard GenesisStateWithValSet helper in
// test_helpers.go so that auth/bank/staking/distribution/gov are all
// seeded consistently with the same denom (utils.BaseDenom) and module
// defaults that the production-style Setup() helper already uses. In
// particular this guarantees distribution.InitialFeePool exists, which the
// staking-rewards test helper depends on; without it
// distrKeeper.GetFeePool returns "collections: not found ... FeePool" the
// first time a staking hook fires.
//
// We deliberately start from an empty simapp.GenesisState (rather than
// app.DefaultGenesis()) so that modules whose key is absent (evm, feemarket,
// ibc, ...) keep their long-standing test behaviour of being skipped during
// InitChain. EthSetup callers that need a real evm/feemarket genesis already
// populate those keys explicitly via the patchGenesis callback.
//
// The result is returned as simapp.GenesisState (an alias of
// map[string]json.RawMessage) so that EthSetup's patchGenesis callback
// signature (and its many call sites) stays untouched.
func NewTestGenesisState(app *Evmos) simapp.GenesisState {
	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	if err != nil {
		panic(err)
	}
	// create validator set with single validator
	validator := cmtypes.NewValidator(pubKey, 1)
	valSet := cmtypes.NewValidatorSet([]*cmtypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(utils.BaseDenom, math.NewInt(100000000000000))),
	}

	genesisState := evmostypes.GenesisState{}
	genesisState = GenesisStateWithValSet(app, genesisState, valSet, []authtypes.GenesisAccount{acc}, balance)
	return simapp.GenesisState(genesisState)
}
