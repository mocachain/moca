package app_test

import (
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/evmos/evmos/v12/sdk/client/test"
	"github.com/evmos/evmos/v12/testutil"
)

func TestExportAppStateAndValidators(t *testing.T) {
	logger := log.NewNopLogger()
	db := dbm.NewMemDB()
	app, _, _ := testutil.NewTestApp(logger, db, nil, true, test.TestChainID)

	_, err := app.ExportAppStateAndValidators(false, nil, []string{banktypes.ModuleName})
	if err != nil {
		t.Fatalf("error exporting state: %s", err)
	}
}
