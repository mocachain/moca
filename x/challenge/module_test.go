package challenge_test

import (
	"encoding/json"
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/x/challenge"
	"github.com/mocachain/moca/v2/x/challenge/types"
)

// newTestAppModule builds a real AppModule wired to the suite's keeper/cdc,
// mirroring how app.go constructs it in production. accountKeeper is never
// invoked by any method under test in this file or module_simulation_test.go,
// so a fresh, expectation-free mock (its own controller, per Risk #3 in the
// coverage plan) is enough.
func newTestAppModule(s *TestSuite) challenge.AppModule {
	mockAccountKeeper := types.NewMockAccountKeeper(gomock.NewController(s.T()))
	return challenge.NewAppModule(s.cdc, *s.challengeKeeper, mockAccountKeeper, s.bankKeeper)
}

func (s *TestSuite) TestAppModuleBasic() {
	amb := challenge.NewAppModuleBasic(s.cdc)

	s.Require().Equal(types.ModuleName, amb.Name())

	s.Require().NotPanics(func() {
		amb.RegisterLegacyAminoCodec(codec.NewLegacyAmino())
	})
	s.Require().NotPanics(func() {
		amb.RegisterInterfaces(cdctypes.NewInterfaceRegistry())
	})

	bz := amb.DefaultGenesis(s.cdc)
	var gotGenesis types.GenesisState
	s.Require().NoError(s.cdc.UnmarshalJSON(bz, &gotGenesis))
	s.Require().Equal(*types.DefaultGenesis(), gotGenesis)

	txCmd := amb.GetTxCmd()
	s.Require().NotNil(txCmd)
	s.Require().Equal(types.ModuleName, txCmd.Use)

	queryCmd := amb.GetQueryCmd()
	s.Require().NotNil(queryCmd)
	s.Require().Equal(types.ModuleName, queryCmd.Use)

	s.Require().NotPanics(func() {
		amb.RegisterGRPCGatewayRoutes(client.Context{}, runtime.NewServeMux())
	})
}

func (s *TestSuite) TestAppModuleBasic_ValidateGenesis() {
	amb := challenge.NewAppModuleBasic(s.cdc)

	invalidGenesis := types.DefaultGenesis()
	invalidGenesis.Params.HeartbeatInterval = 0

	testCases := []struct {
		name      string
		bz        json.RawMessage
		expectErr bool
	}{
		{
			name:      "valid genesis",
			bz:        s.cdc.MustMarshalJSON(types.DefaultGenesis()),
			expectErr: false,
		},
		{
			name:      "malformed json",
			bz:        json.RawMessage(`{not-json`),
			expectErr: true,
		},
		{
			name:      "well-formed json but invalid params",
			bz:        s.cdc.MustMarshalJSON(invalidGenesis),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			err := amb.ValidateGenesis(s.cdc, nil, tc.bz)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func (s *TestSuite) TestAppModule_RegisterServices() {
	am := newTestAppModule(s)

	msr := baseapp.NewMsgServiceRouter()
	msr.SetInterfaceRegistry(s.cdc.InterfaceRegistry())
	qr := baseapp.NewGRPCQueryRouter()
	qr.SetInterfaceRegistry(s.cdc.InterfaceRegistry())
	cfg := module.NewConfigurator(s.cdc, msr, qr)

	s.Require().NotPanics(func() { am.RegisterServices(cfg) })

	// A real observable effect of RegisterServices: both routers now resolve
	// a handler for the module's Msg/Query services.
	s.Require().NotNil(msr.Handler(&types.MsgSubmit{}), "MsgSubmit should be routed after RegisterServices")
	s.Require().NotNil(qr.Route("/moca.challenge.Query/Params"), "Query/Params should be routed after RegisterServices")
}

func (s *TestSuite) TestAppModule_InitExportGenesis() {
	am := newTestAppModule(s)

	genesisState := types.GenesisState{Params: types.DefaultParams()}
	bz := s.cdc.MustMarshalJSON(&genesisState)

	updates := am.InitGenesis(s.ctx, s.cdc, bz)
	s.Require().Empty(updates)

	exportedBz := am.ExportGenesis(s.ctx, s.cdc)
	var exported types.GenesisState
	s.Require().NoError(s.cdc.UnmarshalJSON(exportedBz, &exported))
	s.Require().Equal(genesisState.Params, exported.Params)
}

// TestAppModule_BeginBlock proves AppModule.BeginBlock actually delegates to
// the real BeginBlocker rather than being a no-op wrapper: it seeds an
// already-expired challenge that only BeginBlocker's own store-cleanup logic
// (already exercised directly in abci_test.go) will remove.
func (s *TestSuite) TestAppModule_BeginBlock() {
	am := newTestAppModule(s)

	s.challengeKeeper.SaveChallenge(s.ctx, types.Challenge{Id: 500, ExpiredHeight: 100})
	s.ctx = s.ctx.WithBlockHeight(101)

	s.Require().NoError(am.BeginBlock(s.ctx))
	s.Require().False(s.challengeKeeper.ExistsChallenge(s.ctx, 500))
}

// TestAppModule_EndBlock proves AppModule.EndBlock delegates to the real
// EndBlocker: a zero object count is the cheapest deterministic way to reach
// a mocked StorageKeeper call, which a no-op wrapper would never make. The
// mock's Times(1) expectation fails the test if that delegation regresses.
// EndBlocker's own branches are already covered directly in abci_test.go.
func (s *TestSuite) TestAppModule_EndBlock() {
	am := newTestAppModule(s)

	s.storageKeeper.EXPECT().GetObjectInfoCount(gomock.Any()).Return(math.NewUint(0)).Times(1)

	s.Require().NoError(am.EndBlock(s.ctx))
}

func (s *TestSuite) TestAppModule_Markers() {
	am := newTestAppModule(s)

	s.Require().Equal(uint64(1), am.ConsensusVersion())
	s.Require().NotPanics(func() { am.RegisterInvariants(nil) })

	am.IsAppModule()
	am.IsOnePerModuleType()
}
