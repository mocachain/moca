#!/usr/bin/env bash
# Downloads the third-party protobuf dependencies required for swagger generation
# into swagger-proto/third_party/. Invoked by the `proto-download-deps` Makefile target.
set -eo pipefail

SWAGGER_DIR=./swagger-proto
THIRD_PARTY_DIR="$SWAGGER_DIR/third_party"

# cosmos-sdk (proto + third_party)
(
	mkdir -p "$THIRD_PARTY_DIR/cosmos_tmp"
	cd "$THIRD_PARTY_DIR/cosmos_tmp"
	git init
	git remote add origin "https://github.com/cosmos/cosmos-sdk.git"
	git config core.sparseCheckout true
	printf "proto\nthird_party\n" > .git/info/sparse-checkout
	git pull origin main
	rm -f ./proto/buf.*
	mv ./proto/* ..
)
rm -rf "$THIRD_PARTY_DIR/cosmos_tmp"

# cosmos-proto
(
	mkdir -p "$THIRD_PARTY_DIR/cosmos_proto_tmp"
	cd "$THIRD_PARTY_DIR/cosmos_proto_tmp"
	git init
	git remote add origin "https://github.com/cosmos/cosmos-proto.git"
	git config core.sparseCheckout true
	printf "proto\n" > .git/info/sparse-checkout
	git pull origin main
	rm -f ./proto/buf.*
	mv ./proto/* ..
)
rm -rf "$THIRD_PARTY_DIR/cosmos_proto_tmp"

# cosmos/evm (vm + feemarket protos, pinned to the version moca depends on)
(
	mkdir -p "$THIRD_PARTY_DIR/cosmos_evm_tmp"
	cd "$THIRD_PARTY_DIR/cosmos_evm_tmp"
	git init
	git remote add origin "https://github.com/cosmos/evm.git"
	git config core.sparseCheckout true
	printf "proto\n" > .git/info/sparse-checkout
	git pull origin v0.6.0
	mkdir -p ../cosmos
	cp -r ./proto/cosmos/evm ../cosmos/
)
rm -rf "$THIRD_PARTY_DIR/cosmos_evm_tmp"

# gogoproto
mkdir -p "$THIRD_PARTY_DIR/gogoproto"
curl -SSL https://raw.githubusercontent.com/cosmos/gogoproto/main/gogoproto/gogo.proto > "$THIRD_PARTY_DIR/gogoproto/gogo.proto"

# google/api
mkdir -p "$THIRD_PARTY_DIR/google/api"
curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto > "$THIRD_PARTY_DIR/google/api/annotations.proto"
curl -sSL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto > "$THIRD_PARTY_DIR/google/api/http.proto"

# cosmos/ics23
mkdir -p "$THIRD_PARTY_DIR/cosmos/ics23/v1"
curl -sSL https://raw.githubusercontent.com/cosmos/ics23/master/proto/cosmos/ics23/v1/proofs.proto > "$THIRD_PARTY_DIR/cosmos/ics23/v1/proofs.proto"
