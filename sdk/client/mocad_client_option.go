package client

import (
	"google.golang.org/grpc"

	"github.com/evmos/evmos/v12/sdk/keys"
)

// MocaClientOption configures how we set up the moca client.
type MocaClientOption interface {
	Apply(*MocaClient)
}

// MocaClientOptionFunc defines an applied function for setting the moca client.
type MocaClientOptionFunc func(*MocaClient)

// Apply set up the option field to the client instance.
func (f MocaClientOptionFunc) Apply(client *MocaClient) {
	f(client)
}

// WithKeyManager returns a MocaClientOption which configures a client key manager option.
func WithKeyManager(km keys.KeyManager) MocaClientOption {
	return MocaClientOptionFunc(func(client *MocaClient) {
		client.keyManager = km
	})
}

// WithGrpcConnectionAndDialOption returns a MocaClientOption which configures a grpc client connection with grpc dail options.
func WithGrpcConnectionAndDialOption(grpcAddr string, opts ...grpc.DialOption) MocaClientOption {
	return MocaClientOptionFunc(func(client *MocaClient) {
		client.grpcConn = grpcConn(grpcAddr, opts...)
	})
}

// WithWebSocketClient returns a MocaClientOption which specify that connection is a websocket connection
func WithWebSocketClient() MocaClientOption {
	return MocaClientOptionFunc(func(client *MocaClient) {
		client.useWebSocket = true
	})
}

// grpcConn is used to establish a connection with a given address and dial options.
func grpcConn(addr string, opts ...grpc.DialOption) *grpc.ClientConn {
	conn, err := grpc.Dial(
		addr,
		opts...,
	)
	if err != nil {
		panic(err)
	}
	return conn
}
