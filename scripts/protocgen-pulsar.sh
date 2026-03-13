#!/usr/bin/env bash

set -eo pipefail

echo "Generating pulsar proto code"

# Install protoc-gen-go-grpc (not included in proto-builder image)
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

cd proto
buf generate --template buf.gen.pulsar.yaml \
  --exclude-path moca \
  --exclude-path ethermint/types/v1/account.proto
cd ..
