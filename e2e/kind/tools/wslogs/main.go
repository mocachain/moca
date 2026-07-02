// Command wslogs exercises the eth_subscribe("logs") WebSocket transport
// end-to-end. It dials an EVM JSON-RPC WebSocket endpoint, subscribes to the
// logs of a single contract address, and prints the first matching log as JSON.
//
// It exists so the e2e suite can drive rpc/websockets.go subscribeLogs (which
// rehydrates logs from the finalized block result) over the real WS push path,
// using only the go-ethereum client that moca already depends on — no external
// websocket CLI (websocat/wscat) is installed or required.
//
// Usage:
//
//	wslogs <ws-url> <contract-address> [timeoutSeconds]
//
// It prints "SUBSCRIBED" to stderr as soon as the subscription is live. Because
// eth_subscribe is future-only, the caller must emit the log-producing tx only
// after seeing that line. On the first matching log it writes the log as JSON to
// stdout and exits 0; on a subscription error, dial failure, or timeout it writes
// a message to stderr and exits 1.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const defaultTimeoutSeconds = 40

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "wslogs:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: wslogs <ws-url> <contract-address> [timeoutSeconds]")
	}
	wsURL := args[0]
	if !common.IsHexAddress(args[1]) {
		return fmt.Errorf("invalid contract address: %q", args[1])
	}
	addr := common.HexToAddress(args[1])

	timeoutSeconds := defaultTimeoutSeconds
	if len(args) >= 3 {
		n, err := strconv.Atoi(args[2])
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid timeout seconds: %q", args[2])
		}
		timeoutSeconds = n
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, wsURL)
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer client.Close()

	query := ethereum.FilterQuery{Addresses: []common.Address{addr}}
	logs := make(chan ethtypes.Log)
	sub, err := client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		return fmt.Errorf("subscribe filter logs on %s: %w", wsURL, err)
	}
	defer sub.Unsubscribe()

	// The subscription is now live. Signal the caller so it emits the
	// log-producing tx only now — eth_subscribe delivers future logs only.
	fmt.Fprintln(os.Stderr, "SUBSCRIBED")

	select {
	case vLog := <-logs:
		out, err := json.Marshal(vLog)
		if err != nil {
			return fmt.Errorf("marshal log: %w", err)
		}
		fmt.Println(string(out))
		return nil
	case err := <-sub.Err():
		if err == nil {
			return fmt.Errorf("subscription closed before any matching log arrived")
		}
		return fmt.Errorf("subscription error: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("timed out after %ds waiting for a matching log", timeoutSeconds)
	}
}
