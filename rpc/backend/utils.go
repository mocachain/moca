package backend

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/mocachain/moca/v2/rpc/types"
	mocatypes "github.com/mocachain/moca/v2/types"
)

type txGasAndReward struct {
	gasUsed uint64
	reward  *big.Int
}

type sortGasAndReward []txGasAndReward

func (s sortGasAndReward) Len() int { return len(s) }
func (s sortGasAndReward) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortGasAndReward) Less(i, j int) bool {
	return s[i].reward.Cmp(s[j].reward) < 0
}

// getAccountNonce returns the account nonce for the given account address.
// If the pending value is true, it will iterate over the mempool (pending)
// txs in order to compute and return the pending tx sequence.
// Todo: include the ability to specify a blockNumber
func (b *Backend) getAccountNonce(accAddr common.Address, pending bool, height int64, logger log.Logger) (uint64, error) {
	queryClient := authtypes.NewQueryClient(b.clientCtx)
	adr := sdk.AccAddress(accAddr.Bytes()).String()
	ctx := types.ContextWithHeight(height)
	res, err := queryClient.Account(ctx, &authtypes.QueryAccountRequest{Address: adr})
	if err != nil {
		st, ok := status.FromError(err)
		// treat as account doesn't exist yet
		if ok && st.Code() == codes.NotFound {
			return 0, nil
		}
		return 0, err
	}
	var acc sdk.AccountI
	if err := b.clientCtx.InterfaceRegistry.UnpackAny(res.Account, &acc); err != nil {
		return 0, err
	}

	nonce := acc.GetSequence()

	if !pending {
		return nonce, nil
	}

	// the account retriever doesn't include the uncommitted transactions on the nonce so we need to
	// to manually add them.
	pendingTxs, err := b.PendingTransactions()
	if err != nil {
		logger.Error("failed to fetch pending transactions", "error", err.Error())
		return nonce, nil
	}

	// use precomputed chainID; b.ChainID() RPCs and fails before EIP-155.
	signer := ethtypes.LatestSignerForChainID(b.chainID)

	// add the uncommitted txs to the nonce counter
	// only supports `MsgEthereumTx` style tx
	for _, tx := range pendingTxs {
		for _, msg := range (*tx).GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				// not ethereum tx
				break
			}

			// pending txs have empty From; recover sender from signature.
			sender, err := ethMsg.GetSenderLegacy(signer)
			if err != nil {
				continue
			}
			if sender == accAddr {
				// saturate - never overflow beyond 2^64-1 when counting pending txs
				if nonce < math.MaxUint64 {
					nonce++
				}
			}
		}
	}

	return nonce, nil
}

// output: targetOneFeeHistory
func (b *Backend) processBlock(
	tendermintBlock *tmrpctypes.ResultBlock,
	ethBlock *map[string]interface{},
	rewardPercentiles []float64,
	tendermintBlockResult *tmrpctypes.ResultBlockResults,
	targetOneFeeHistory *types.OneFeeHistory,
) error {
	blockHeight := tendermintBlock.Block.Height
	blockBaseFee, err := b.BaseFee(tendermintBlockResult)
	if err != nil {
		return err
	}

	// set basefee
	targetOneFeeHistory.BaseFee = blockBaseFee
	cfg := b.ChainConfig()
	if cfg.IsLondon(big.NewInt(blockHeight + 1)) {
		header, err := b.CurrentHeader()
		if err != nil {
			return err
		}
		targetOneFeeHistory.NextBaseFee = eip1559.CalcBaseFee(cfg, header)
	} else {
		targetOneFeeHistory.NextBaseFee = new(big.Int)
	}
	// set gas used ratio
	gasLimitUint64, ok := (*ethBlock)["gasLimit"].(hexutil.Uint64)
	if !ok {
		return fmt.Errorf("invalid gas limit type: %T", (*ethBlock)["gasLimit"])
	}

	gasUsedBig, ok := (*ethBlock)["gasUsed"].(*hexutil.Big)
	if !ok {
		return fmt.Errorf("invalid gas used type: %T", (*ethBlock)["gasUsed"])
	}

	gasusedfloat, _ := new(big.Float).SetInt(gasUsedBig.ToInt()).Float64()

	if gasLimitUint64 <= 0 {
		return fmt.Errorf("gasLimit of block height %d should be bigger than 0 , current gaslimit %d", blockHeight, gasLimitUint64)
	}

	gasUsedRatio := gasusedfloat / float64(gasLimitUint64)
	blockGasUsed := gasusedfloat
	targetOneFeeHistory.GasUsedRatio = gasUsedRatio

	rewardCount := len(rewardPercentiles)
	targetOneFeeHistory.Reward = make([]*big.Int, rewardCount)
	for i := 0; i < rewardCount; i++ {
		targetOneFeeHistory.Reward[i] = big.NewInt(0)
	}

	// check tendermintTxs
	tendermintTxs := tendermintBlock.Block.Txs
	tendermintTxResults := tendermintBlockResult.TxsResults
	tendermintTxCount := len(tendermintTxs)

	var sorter sortGasAndReward

	for i := 0; i < tendermintTxCount; i++ {
		eachTendermintTx := tendermintTxs[i]
		eachTendermintTxResult := tendermintTxResults[i]

		tx, err := b.clientCtx.TxConfig.TxDecoder()(eachTendermintTx)
		if err != nil {
			b.logger.Debug("failed to decode transaction in block", "height", blockHeight, "error", err.Error())
			continue
		}
		txGasUsed := uint64(eachTendermintTxResult.GasUsed) // #nosec G701
		for _, msg := range tx.GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				continue
			}
			tx := ethMsg.AsTransaction()
			// underpriced error treated as zero reward.
			reward, _ := tx.EffectiveGasTip(blockBaseFee)
			if reward == nil {
				reward = big.NewInt(0)
			}
			sorter = append(sorter, txGasAndReward{gasUsed: txGasUsed, reward: reward})
		}
	}

	// return an all zero row if there are no transactions to gather data from
	ethTxCount := len(sorter)
	if ethTxCount == 0 {
		return nil
	}

	sort.Sort(sorter)

	var txIndex int
	sumGasUsed := sorter[0].gasUsed

	for i, p := range rewardPercentiles {
		thresholdGasUsed := uint64(blockGasUsed * p / 100) // #nosec G701
		for sumGasUsed < thresholdGasUsed && txIndex < ethTxCount-1 {
			txIndex++
			sumGasUsed += sorter[txIndex].gasUsed
		}
		targetOneFeeHistory.Reward[i] = sorter[txIndex].reward
	}

	return nil
}

// ShouldIgnoreGasUsed returns true if the gasUsed in result should be ignored
// workaround for issue: https://github.com/cosmos/cosmos-sdk/issues/10832
func ShouldIgnoreGasUsed(res *abci.ExecTxResult) bool {
	return res.GetCode() == 11 && strings.Contains(res.GetLog(), "no block gas left to run tx: out of gas")
}

// GetLogsFromBlockResults decodes EVM logs from tx response Data for each tx in the block result.
func GetLogsFromBlockResults(blockRes *tmrpctypes.ResultBlockResults) ([][]*ethtypes.Log, error) {
	height, err := mocatypes.SafeUint64(blockRes.Height)
	if err != nil {
		return nil, err
	}
	blockLogs := [][]*ethtypes.Log{}
	for _, txResult := range blockRes.TxsResults {
		logs, err := evmtypes.DecodeTxLogs(txResult.Data, height)
		if err != nil {
			return nil, err
		}

		blockLogs = append(blockLogs, logs)
	}
	return blockLogs, nil
}

// GetHexProofs returns list of hex data of proof op
func GetHexProofs(proof *crypto.ProofOps) []string {
	if proof == nil {
		return []string{""}
	}
	proofs := []string{}
	// check for proof
	for _, p := range proof.Ops {
		proof := ""
		if len(p.Data) > 0 {
			proof = hexutil.Encode(p.Data)
		}
		proofs = append(proofs, proof)
	}
	return proofs
}
