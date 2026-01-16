package client

import (
	"context"
	"encoding/hex"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/votepool"
)

func (c *MocaClient) ABCIInfo(ctx context.Context) (*ctypes.ResultABCIInfo, error) {
	return c.tendermintClient.ABCIInfo(ctx)
}

// GetBlock by height, gets the latest block if height is nil
func (c *MocaClient) GetBlock(ctx context.Context, height *int64) (*ctypes.ResultBlock, error) {
	return c.tendermintClient.Block(ctx, height)
}

// Tx gets a tx by detail by the tx hash
func (c *MocaClient) Tx(ctx context.Context, txHash string) (*ctypes.ResultTx, error) {
	hash, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, err
	}
	return c.tendermintClient.Tx(ctx, hash, true)
}

// GetBlockResults by height, gets the latest block result if height is nil
func (c *MocaClient) GetBlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error) {
	return c.tendermintClient.BlockResults(ctx, height)
}

// GetValidators by height, gets the latest validators if height is nil
func (c *MocaClient) GetValidators(ctx context.Context, height *int64) (*ctypes.ResultValidators, error) {
	return c.tendermintClient.Validators(ctx, height, nil, nil)
}

// GetHeader by height, gets the latest block header if height is nil
func (c *MocaClient) GetHeader(ctx context.Context, height *int64) (*ctypes.ResultHeader, error) {
	return c.tendermintClient.Header(ctx, height)
}

// GetUnconfirmedTxs by height, gets the latest block header if height is nil
func (c *MocaClient) GetUnconfirmedTxs(ctx context.Context, limit *int) (*ctypes.ResultUnconfirmedTxs, error) {
	return c.tendermintClient.UnconfirmedTxs(ctx, limit)
}

func (c *MocaClient) GetCommit(ctx context.Context, height int64) (*ctypes.ResultCommit, error) {
	return c.tendermintClient.Commit(ctx, &height)
}

func (c *MocaClient) GetStatus(ctx context.Context) (*ctypes.ResultStatus, error) {
	return c.tendermintClient.Status(ctx)
}

func (c *MocaClient) BroadcastVote(ctx context.Context, vote votepool.Vote) error {
	_, err := c.tendermintClient.BroadcastVote(ctx, vote)
	return err
}

func (c *MocaClient) QueryVote(ctx context.Context, eventType int, eventHash []byte) (*ctypes.ResultQueryVote, error) {
	return c.tendermintClient.QueryVote(ctx, eventType, eventHash)
}
