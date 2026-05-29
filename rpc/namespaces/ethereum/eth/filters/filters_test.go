package filters

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/log"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethfilters "github.com/ethereum/go-ethereum/eth/filters"

	"github.com/evmos/evmos/v12/rpc/types"
)

type stubBackend struct{}

func (stubBackend) GetBlockByNumber(types.BlockNumber, bool) (map[string]interface{}, error) {
	panic("unexpected call")
}

func (stubBackend) HeaderByNumber(types.BlockNumber) (*ethtypes.Header, error) {
	panic("unexpected call")
}

func (stubBackend) HeaderByHash(common.Hash) (*ethtypes.Header, error) {
	panic("unexpected call")
}

func (stubBackend) TendermintBlockByHash(common.Hash) (*tmrpctypes.ResultBlock, error) {
	return nil, nil
}

func (stubBackend) TendermintBlockResultByNumber(*int64) (*tmrpctypes.ResultBlockResults, error) {
	panic("unexpected call")
}

func (stubBackend) GetLogs(common.Hash) ([][]*ethtypes.Log, error) {
	panic("unexpected call")
}

func (stubBackend) GetLogsByHeight(*int64) ([][]*ethtypes.Log, error) {
	panic("unexpected call")
}

func (stubBackend) BlockBloom(*tmrpctypes.ResultBlockResults) (ethtypes.Bloom, error) {
	panic("unexpected call")
}

func (stubBackend) BloomStatus() (uint64, uint64)  { panic("unexpected call") }
func (stubBackend) RPCFilterCap() int32            { panic("unexpected call") }
func (stubBackend) RPCLogsCap() int32              { panic("unexpected call") }
func (stubBackend) RPCBlockRangeCap() int32        { panic("unexpected call") }
func (stubBackend) RPCQueryTimeout() time.Duration { panic("unexpected call") }
func (stubBackend) RPCGetLogsRateLimit() int       { panic("unexpected call") }
func (stubBackend) RPCGetLogsBurstLimit() int      { panic("unexpected call") }

func TestBlockFilterLogsReturnsEmptyForUnknownBlockHash(t *testing.T) {
	blockHash := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	filter := NewBlockFilter(
		log.NewNopLogger(),
		stubBackend{},
		gethfilters.FilterCriteria{BlockHash: &blockHash},
	)

	logs, err := filter.Logs(context.Background(), 100, 100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected empty logs, got %d entries", len(logs))
	}
}
