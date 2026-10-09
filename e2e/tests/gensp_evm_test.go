package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

// genesisSPMinDepositAmoca mirrors deployment/localup/.env.example's
// SP_MIN_DEPOSIT_AMOUNT -- the floor every genesis-created SP's deposit is
// generated against.
const genesisSPMinDepositAmoca = "10000000000000000000000000"

// TestGenesisStorageProviderEvmFlow queries a genesis-seeded storage
// provider's on-chain record, exercising the same read path as the retired
// suite's TestGenStorageProvider. Unlike that test, TotalDeposit is checked
// against an independently-known floor (the audit flagged that the
// original only compared the query response to itself for that field).
func TestGenesisStorageProviderEvmFlow(t *testing.T) {
	ctx := context.Background()
	_, conn := dialChain(t)
	spClient := sptypes.NewQueryClient(conn)

	spExport := loadSPExport(t)
	sp0Export, ok := spExport["sp0"]
	require.True(t, ok, "sp_export.json missing sp0 -- run localup.sh export_sps 1 7 first")

	resp, err := spClient.StorageProviderByOperatorAddress(ctx, &sptypes.QueryStorageProviderByOperatorAddressRequest{
		OperatorAddress: sp0Export.OperatorAddress,
	})
	require.NoError(t, err)

	sp := resp.StorageProvider
	require.Equal(t, sp0Export.OperatorAddress, sp.OperatorAddress)
	require.Equal(t, sp0Export.ApprovalAddress, sp.ApprovalAddress)
	require.NotEmpty(t, sp.Endpoint)
	require.True(t, sp.TotalDeposit.GTE(sdkmath.NewIntFromBigInt(mustBigInt(t, genesisSPMinDepositAmoca))),
		"genesis SP deposit should be at least the configured floor")
}
