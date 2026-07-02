package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	rpcclient "github.com/cometbft/cometbft/rpc/client"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/ethereum/go-ethereum/common"
	ethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/cosmos/evm/rpc"
	"github.com/cosmos/evm/rpc/stream"
	serverconfig "github.com/cosmos/evm/server/config"
	servertypes "github.com/cosmos/evm/server/types"

	svrconfig "github.com/mocachain/moca/v2/server/config"
)

// AppWithPendingTxStream is implemented by *app.Moca. cosmos/evm's JSON-RPC
// server registers its pending-tx subscription stream on the app here, so that
// newPendingTransactions fires from CheckTx via the ante TxListenerDecorator.
type AppWithPendingTxStream interface {
	RegisterPendingTxListener(listener func(common.Hash))
}

// StartJSONRPC starts the JSON-RPC server
func StartJSONRPC(ctx *server.Context,
	clientCtx client.Context,
	config *svrconfig.AppConfig,
	indexer servertypes.EVMTxIndexer,
	app AppWithPendingTxStream,
) (*http.Server, chan struct{}, error) {
	logger := ctx.Logger.With("module", "geth")

	// cosmos/evm's RPC layer is driven by its own server config.Config,
	// populated from the same viper instance moca's AppConfig reads
	// (mapstructure keys align). The backend independently re-reads the EVM
	// chain-id from viper (evm.evm-chain-id), matching the keeper.
	cevmCfg, err := serverconfig.GetConfig(ctx.Viper)
	if err != nil {
		return nil, nil, err
	}

	// The in-process CometBFT client (local.New(tmNode)) implements EventsClient.
	// cosmos/evm's stream subscribes to it for newHeads/logs; pending txs arrive
	// via the ante TxListenerDecorator wired through RegisterPendingTxListener.
	evtClient, ok := clientCtx.Client.(rpcclient.EventsClient)
	if !ok {
		return nil, nil, fmt.Errorf("client %T does not implement EventsClient", clientCtx.Client)
	}
	rpcStream := stream.NewRPCStreams(evtClient, logger, clientCtx.TxConfig.TxDecoder())
	app.RegisterPendingTxListener(rpcStream.ListenPendingTx)

	// TODO(cosmos-evm migration): bridge geth library logs into moca's
	// structured logger via an slog.Handler. Until that lands, geth library
	// logs go to geth's default sink instead of merging into moca's output.

	rpcServer := ethrpc.NewServer()
	rpcServer.SetBatchLimits(cevmCfg.JSONRPC.BatchRequestLimit, cevmCfg.JSONRPC.BatchResponseMaxSize)

	allowUnprotectedTxs := cevmCfg.JSONRPC.AllowUnprotectedTxs
	rpcAPIArr := cevmCfg.JSONRPC.API

	// mempool is nil: moca does not run cosmos/evm's experimental EVM mempool.
	// Every backend use of it is nil-guarded; pending nonce still works via
	// CometBFT UnconfirmedTxs.
	apis := rpc.GetRPCAPIs(ctx, clientCtx, rpcStream, allowUnprotectedTxs, indexer, rpcAPIArr, nil)

	for _, api := range apis {
		if err := rpcServer.RegisterName(api.Namespace, api.Service); err != nil {
			ctx.Logger.Error(
				"failed to register service in JSON RPC namespace",
				"namespace", api.Namespace,
				"service", api.Service,
			)
			return nil, nil, err
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/", rpcServer.ServeHTTP).Methods("POST")

	handlerWithCors := cors.Default()
	if config.API.EnableUnsafeCORS {
		handlerWithCors = cors.AllowAll()
	}

	httpSrv := &http.Server{
		Addr:              config.JSONRPC.Address,
		Handler:           handlerWithCors.Handler(r),
		ReadHeaderTimeout: config.JSONRPC.HTTPTimeout,
		ReadTimeout:       config.JSONRPC.HTTPTimeout,
		WriteTimeout:      config.JSONRPC.HTTPTimeout,
		IdleTimeout:       config.JSONRPC.HTTPIdleTimeout,
	}
	httpSrvDone := make(chan struct{}, 1)

	ln, err := Listen(httpSrv.Addr, config)
	if err != nil {
		return nil, nil, err
	}

	errCh := make(chan error)
	go func() {
		ctx.Logger.Info("Starting JSON-RPC server", "address", config.JSONRPC.Address)
		if err := httpSrv.Serve(ln); err != nil {
			if err == http.ErrServerClosed {
				close(httpSrvDone)
				return
			}

			ctx.Logger.Error("failed to start JSON-RPC server", "error", err.Error())
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		ctx.Logger.Error("failed to boot JSON-RPC server", "error", err.Error())
		return nil, nil, err
	case <-time.After(svrconfig.ServerStartTime): // assume JSON RPC server started successfully
	}

	ctx.Logger.Info("Starting JSON WebSocket server", "address", config.JSONRPC.WsAddress)

	// moca's websockets shim (server/websockets.go): a thin copy of cosmos/evm's
	// WS server whose only behavioral difference is that subscribeNewHeads emits
	// the canonical CometBFT block hash (moca #232), so newHeads "hash" matches
	// eth_getBlockByNumber. It shares rpcStream so the HTTP filter APIs and the WS
	// subscriptions observe the same events.
	wsSrv := NewWebsocketsServer(clientCtx, ctx.Logger, rpcStream, &cevmCfg)
	wsSrv.Start()
	return httpSrv, httpSrvDone, nil
}
