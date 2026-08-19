package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/precompiles/storageprovider"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// TestStorageProviderPriceEvmFlow drives x/sp's per-SP storage pricing
// through the storageprovider precompile's updateSPPrice method, exercising
// the same functional path as the retired suite's
// TestUpdateSpStoragePrice (flagged there as the oracle-critical test) but
// over a real signed EVM transaction against a live node.
func TestStorageProviderPriceEvmFlow(t *testing.T) {
	ctx := context.Background()
	chainID := big.NewInt(evmChainIDNum)
	client, conn := dialChain(t)
	spClient := sptypes.NewQueryClient(conn)

	spExport := loadSPExport(t)
	sp0Export, ok := spExport["sp0"]
	require.True(t, ok, "sp_export.json missing sp0 -- run localup.sh export_sps 1 7 first")

	sp0OperatorKey := mustHexKey(t, sp0Export.OperatorPrivateKey)
	operatorAddr := crypto.PubkeyToAddress(sp0OperatorKey.PublicKey)
	fundMoca(t, ctx, client, chainID, operatorAddr, fundingAmountMOCA)

	before, err := spClient.QuerySpStoragePrice(ctx, &sptypes.QuerySpStoragePriceRequest{
		SpAddr: sp0Export.OperatorAddress,
	})
	require.NoError(t, err)

	// LegacyDec.BigInt() returns the value's raw 18-decimal-precision
	// internal representation, which is exactly what updateSPPrice expects
	// (it reconstructs via LegacyNewDecFromBigIntWithPrec(x, 18)) -- so
	// doubling that raw integer doubles the reconstructed Dec price.
	newReadPrice := new(big.Int).Mul(before.SpStoragePrice.ReadPrice.BigInt(), big.NewInt(2))
	newStorePrice := new(big.Int).Mul(before.SpStoragePrice.StorePrice.BigInt(), big.NewInt(2))
	newFreeReadQuota := before.SpStoragePrice.FreeReadQuota + 1024

	precompile := storageprovider.Precompile{}
	method := storageprovider.GetAbiMethod(storageprovider.UpdateSPPriceMethodName)
	packedArgs, err := method.Inputs.Pack(newReadPrice, newFreeReadQuota, newStorePrice)
	require.NoError(t, err)
	calldata := append(append([]byte{}, method.ID...), packedArgs...)
	sendPrecompileTx(t, ctx, client, chainID, sp0OperatorKey, precompile.Address(), calldata)

	after, err := spClient.QuerySpStoragePrice(ctx, &sptypes.QuerySpStoragePriceRequest{
		SpAddr: sp0Export.OperatorAddress,
	})
	require.NoError(t, err)
	require.Equal(t, newReadPrice, after.SpStoragePrice.ReadPrice.BigInt())
	require.Equal(t, newStorePrice, after.SpStoragePrice.StorePrice.BigInt())
	require.Equal(t, newFreeReadQuota, after.SpStoragePrice.FreeReadQuota)
	require.Greater(t, after.SpStoragePrice.UpdateTimeSec, before.SpStoragePrice.UpdateTimeSec)
}
