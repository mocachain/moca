package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	servertypes "github.com/cosmos/evm/server/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// mockRPCClient scripts the only four rpcclient.Client methods the indexer
// service uses. BlockResults fails until failuresLeft drains, tracking the
// attempt count — the seam for both indexer-loop regressions.
type mockRPCClient struct {
	rpcclient.Client // nil-embedded: any unscripted call panics loudly

	height       int64
	headerCh     chan coretypes.ResultEvent
	failuresLeft atomic.Int64
	attempts     atomic.Int64
}

func (m *mockRPCClient) Status(context.Context) (*coretypes.ResultStatus, error) {
	return &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{LatestBlockHeight: m.height},
	}, nil
}

func (m *mockRPCClient) Subscribe(_ context.Context, _, _ string, _ ...int) (<-chan coretypes.ResultEvent, error) {
	return m.headerCh, nil
}

func (m *mockRPCClient) Block(_ context.Context, height *int64) (*coretypes.ResultBlock, error) {
	return &coretypes.ResultBlock{
		Block: &cmttypes.Block{Header: cmttypes.Header{Height: *height}},
	}, nil
}

func (m *mockRPCClient) BlockResults(_ context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	m.attempts.Add(1)
	if m.failuresLeft.Add(-1) >= 0 {
		return nil, context.DeadlineExceeded
	}
	return &coretypes.ResultBlockResults{Height: *height}, nil
}

// mockIndexer records indexed heights; the query methods are never used by
// the service loop.
type mockIndexer struct {
	indexed chan int64
}

func (m *mockIndexer) LastIndexedBlock() (int64, error)  { return 0, nil }
func (m *mockIndexer) FirstIndexedBlock() (int64, error) { return 0, nil }
func (m *mockIndexer) IndexBlock(block *cmttypes.Block, _ []*abci.ExecTxResult) error {
	m.indexed <- block.Height
	return nil
}

func (m *mockIndexer) GetByTxHash(common.Hash) (*servertypes.TxResult, error) {
	panic("not used")
}

func (m *mockIndexer) GetByBlockAndIndex(int64, int32) (*servertypes.TxResult, error) {
	panic("not used")
}

// TestIndexerServiceRetriesAfterFetchError guards both historical failure
// modes of the indexing loop around transient Block/BlockResults errors:
//
//   - stall (upstream cosmos/evm as of v0.6.0): the fetch error is latched and
//     never cleared, so after the first failure the loop only sleeps and the
//     indexer never progresses again — this test times out on that code;
//   - busy-loop (moca's pre-fix copy): the loop hammered the failing fetch
//     with no backoff — the bounded attempt count below fails on that code.
//
// Correct behavior: back off on error, then re-attempt on the next new-block
// signal (or timeout) and recover.
func TestIndexerServiceRetriesAfterFetchError(t *testing.T) {
	client := &mockRPCClient{
		height:   1,
		headerCh: make(chan coretypes.ResultEvent, 4),
	}
	// One failure → recovery needs exactly one wake-up token. (Two needed two, but the
	// capacity-1 signal channel coalesces close headers into one → the old -race flake.)
	client.failuresLeft.Store(1)
	idxr := &mockIndexer{indexed: make(chan int64, 8)}

	svc := NewEVMIndexerService(idxr, client)
	go func() {
		_ = svc.OnStart() // loops forever; test asserts via channels
	}()

	signal := func(height int64) {
		client.headerCh <- coretypes.ResultEvent{
			Data: cmttypes.EventDataNewBlockHeader{
				Header: cmttypes.Header{Height: height},
			},
		}
	}

	// Phase 1 — busy-loop check: startup catch-up (height 1) fails, no signal sent.
	// Correct code parks; a busy-loop retries immediately and indexes without a wake-up.
	select {
	case h := <-idxr.indexed:
		t.Fatalf("indexed height %d without a wake-up: busy-loop regression", h)
	case <-time.After(1500 * time.Millisecond): // parked, as it should be
	}

	// Phase 2 — latch check: one signal = one retry token. Correct code clears the error
	// and catches up (heights 1–2); the latched variant stays parked and trips the deadline.
	signal(2)

	deadline := time.After(10 * time.Second)
	got := map[int64]bool{}
	for len(got) < 2 {
		select {
		case h := <-idxr.indexed:
			got[h] = true
		case <-deadline:
			t.Fatalf("indexer stalled after transient fetch error; indexed so far: %v (attempts=%d)",
				got, client.attempts.Load())
		}
	}
	require.True(t, got[1] && got[2], "heights 1 and 2 must be indexed after recovery, got %v", got)

	// Sanity bound: catch-up failure + signaled retry + follow-up block = 3.
	require.LessOrEqual(t, client.attempts.Load(), int64(4),
		"fetch attempts should be bounded by wake-ups")
}
