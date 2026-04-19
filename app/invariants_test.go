// Copyright 2026 Moca Authors
// This file is part of the Moca packages.
//
// Moca is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Moca packages are distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Moca packages. If not, see <https://www.gnu.org/licenses/>.

package app

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func newInvariantTestCtx(t *testing.T) sdk.Context {
	t.Helper()
	return sdktestutil.DefaultContextWithDB(
		t,
		storetypes.NewKVStoreKey("inv_test"),
		storetypes.NewTransientStoreKey("inv_test_t"),
	).Ctx
}

// TestExportInvariantRegistry_RegisterRouteCollectsAll guards against future
// edits dropping or reordering the append in RegisterRoute, which would cause
// some module invariants to silently never run on export.
func TestExportInvariantRegistry_RegisterRouteCollectsAll(t *testing.T) {
	r := &exportInvariantRegistry{}
	r.RegisterRoute("bank", "supply", func(sdk.Context) (string, bool) { return "", false })
	r.RegisterRoute("staking", "shares", func(sdk.Context) (string, bool) { return "", false })
	r.RegisterRoute("distribution", "module-account", func(sdk.Context) (string, bool) { return "", false })

	require.Len(t, r.routes, 3, "RegisterRoute must append every invariant")
}

// TestExportInvariantRegistry_AssertAllPassesWhenHealthy is the happy-path
// regression test: a clean state must let every invariant report broken=false
// and AssertAll must return without panicking. Real chain exports rely on this
// so the regression test ensures we never flip the broken/healthy semantics.
func TestExportInvariantRegistry_AssertAllPassesWhenHealthy(t *testing.T) {
	ctx := newInvariantTestCtx(t)

	r := &exportInvariantRegistry{}
	r.RegisterRoute("bank", "supply", func(sdk.Context) (string, bool) { return "", false })
	r.RegisterRoute("staking", "shares", func(sdk.Context) (string, bool) { return "", false })

	require.NotPanics(t, func() { r.AssertAll(ctx) })
}

// TestExportInvariantRegistry_AssertAllPanicsAndStopsOnFirstBroken pins the
// safety net behaviour: as soon as any invariant returns broken=true AssertAll
// must panic with that invariant's message and must not run any later
// invariant. If a future refactor swaps panic for return or break for continue,
// this test will catch the regression.
func TestExportInvariantRegistry_AssertAllPanicsAndStopsOnFirstBroken(t *testing.T) {
	ctx := newInvariantTestCtx(t)

	var visited int
	r := &exportInvariantRegistry{}
	r.RegisterRoute("bank", "supply", func(sdk.Context) (string, bool) {
		visited++
		return "", false
	})
	r.RegisterRoute("staking", "shares", func(sdk.Context) (string, bool) {
		visited++
		return "delegator-shares broken", true
	})
	r.RegisterRoute("distribution", "module-account", func(sdk.Context) (string, bool) {
		visited++
		return "", false
	})

	require.PanicsWithValue(t, "delegator-shares broken", func() { r.AssertAll(ctx) })
	require.Equal(t, 2, visited, "AssertAll must short-circuit on the first broken invariant")
}

// TestExportInvariantRegistry_AssertAllUsesCacheContext guards the CacheContext
// isolation: an invariant that accidentally writes to its ctx (which is against
// convention) must not pollute the parent ctx. Removing the CacheContext call
// in AssertAll would make this test fail.
func TestExportInvariantRegistry_AssertAllUsesCacheContext(t *testing.T) {
	parentKey := storetypes.NewKVStoreKey("parent")
	parentCtx := sdktestutil.DefaultContextWithDB(
		t,
		parentKey,
		storetypes.NewTransientStoreKey("parent_t"),
	).Ctx

	r := &exportInvariantRegistry{}
	r.RegisterRoute("rogue", "writer", func(c sdk.Context) (string, bool) {
		c.KVStore(parentKey).Set([]byte("k"), []byte("v"))
		return "", false
	})

	r.AssertAll(parentCtx)

	require.Nil(
		t,
		parentCtx.KVStore(parentKey).Get([]byte("k")),
		"writes from inside an invariant must stay inside CacheContext",
	)
}
