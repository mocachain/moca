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
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// exportInvariantRegistry is a minimal sdk.InvariantRegistry implementation
// used solely to keep the export-time state self-check that x/crisis used to
// provide. The crisis module itself was removed because its runtime
// BeginBlocker check and MsgVerifyInvariant governance handler are not used in
// moca; only Keeper.AssertInvariants on the export path was actually relied on.
// This 30-line registry preserves that safety net without re-introducing the
// full crisis module surface (CLI flag, store, governance handler, etc.).
//
// Concretely it is wired up exactly where app.mm.RegisterInvariants(&CrisisKeeper)
// used to live, so every standard SDK module (bank, staking, distribution, gov)
// still gets a chance to hand its invariants to a collector. The collected
// invariants are then run from app/export.go before any state is mutated for
// zero-height export.
type exportInvariantRegistry struct {
	routes []sdk.Invariant
}

// RegisterRoute satisfies sdk.InvariantRegistry. The moduleName/route arguments
// are intentionally discarded because moca no longer exposes the
// `tx crisis invariant-broken` command that needed those identifiers; the panic
// message produced by the invariant itself is sufficient to locate the failure.
func (r *exportInvariantRegistry) RegisterRoute(_, _ string, inv sdk.Invariant) {
	r.routes = append(r.routes, inv)
}

// AssertAll runs every registered invariant against a CacheContext so that an
// invariant which accidentally mutates state (invariants are read-only by
// convention) cannot leak into the real KV store. The first broken invariant
// panics with the descriptive message returned by the invariant function,
// mirroring x/crisis Keeper.AssertInvariants behaviour.
func (r *exportInvariantRegistry) AssertAll(ctx sdk.Context) {
	for _, inv := range r.routes {
		invCtx, _ := ctx.CacheContext()
		if msg, broken := inv(invCtx); broken {
			panic(msg)
		}
	}
}
