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

package mocks

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
)

type MockCometRPC struct {
	rpcclientmock.Client

	responseQuery abci.ResponseQuery
}

// NewMockCometRPC returns a mock CometRPC implementation.
// It is used for CLI testing.
func NewMockCometRPC(respQuery abci.ResponseQuery) MockCometRPC {
	return MockCometRPC{responseQuery: respQuery}
}

func (MockCometRPC) BroadcastTxSync(context.Context, cmttypes.Tx) (*coretypes.ResultBroadcastTx, error) {
	return &coretypes.ResultBroadcastTx{Code: 0}, nil
}

func (m MockCometRPC) ABCIQueryWithOptions(
	_ context.Context,
	_ string,
	_ tmbytes.HexBytes,
	_ rpcclient.ABCIQueryOptions,
) (*coretypes.ResultABCIQuery, error) {
	return &coretypes.ResultABCIQuery{Response: m.responseQuery}, nil
}
