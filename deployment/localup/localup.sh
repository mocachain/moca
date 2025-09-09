#!/usr/bin/env bash
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# Get absolute path of project root directory
PROJECT_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
local_env=${SCRIPT_DIR}/.local

source ${SCRIPT_DIR}/.env

# Silent mode flag (enabled by default to reduce terminal output noise)
QUIET_MODE=true

# Helper function to execute commands based on silent mode
execute_with_mode() {
    local cmd="$1"
    local log_file="$2"
    local description="$3"

    if [ "$QUIET_MODE" = "true" ]; then
        execute_logged "$cmd" "$log_file" "$description" "false"
    else
        execute_logged "$cmd" "$log_file" "$description" "true"
    fi
}
source ${SCRIPT_DIR}/utils.sh
source ${SCRIPT_DIR}/log-manager.sh
devaccount_prikey=2228e392584d902843272c37fd62b8c73c10c81a5ecb901773c9ebe366e937bb
validator0_prikey=e54bff83fc945cba77ca3e45d69adc5b57ad8db6073736c8422692abecfb5fe2
relayer0_prikey=3c7ea76ddb53539174caae1dd960b308981933bd6e95196556ba29063200df9c
sp0_prikey=ebbeb28b89bc7ec5da6441ed70452cc413f96ea33a7c790aba06810ae441b776

bin_name=mocad
bin=${PROJECT_ROOT}/build/${bin_name}

function init() {
	size=$1
	start_step "Environment Initialization"

	log_info "Starting initialization of ${size} validator node environments"
	execute_with_mode "rm -rf ${local_env}" "${INIT_LOG}" "Clean old environment directory"

	# Re-initialize logging system (because the entire .local directory was deleted above)
	mkdir -p "${LOG_SESSION_DIR}"
	touch "${INIT_LOG}" "${KEYGEN_LOG}" "${GENESIS_LOG}" "${CONFIG_LOG}" "${START_LOG}" "${STOP_LOG}" "${ERROR_LOG}" "${SUMMARY_LOG}"

	execute_with_mode "mkdir -p ${local_env}" "${INIT_LOG}" "Create local environment directory"

	for ((i = 0; i < ${size}; i++)); do
		log_info "Initializing validator node ${i}"
		execute_quiet "mkdir -p ${local_env}/validator${i}" "${INIT_LOG}" "Create validator${i} directory"
		execute_quiet "mkdir -p ${local_env}/relayer${i}" "${INIT_LOG}" "Create relayer${i} directory"
		execute_quiet "mkdir -p ${local_env}/challenger${i}" "${INIT_LOG}" "Create challenger${i} directory"

		# init chain
		execute_with_mode "${bin} init validator${i} --chain-id \"${CHAIN_ID}\" --default-denom \"${STAKING_BOND_DENOM}\" --home ${local_env}/validator${i}" "${INIT_LOG}" "Initialize validator${i} chain configuration"

		# create genesis accounts
		start_step "Key Generation - Validator${i}"
		if [ "$i" -eq 0 ]; then
			log_info "Importing main validator preset keys"
			execute_with_mode "${bin} keys import devaccount ${devaccount_prikey} --secp256k1-private-key --keyring-backend test --home ${local_env}/validator0" "${KEYGEN_LOG}" "Import dev account key"
			execute_with_mode "${bin} keys import validator0 ${validator0_prikey} --secp256k1-private-key --keyring-backend test --home ${local_env}/validator0" "${KEYGEN_LOG}" "Import validator0 key"
			execute_with_mode "${bin} keys import relayer0 ${relayer0_prikey} --secp256k1-private-key --keyring-backend test --home ${local_env}/relayer0" "${KEYGEN_LOG}" "Import relayer0 key"
			execute_quiet "${bin} keys show devaccount --keyring-backend test --home ${local_env}/validator0" "${KEYGEN_LOG}" "Generate dev account info"
			execute_quiet "${bin} keys show validator0 --keyring-backend test --home ${local_env}/validator0" "${KEYGEN_LOG}" "Generate validator0 info"
			execute_quiet "${bin} keys show relayer0 --keyring-backend test --home ${local_env}/relayer0" "${KEYGEN_LOG}" "Generate relayer0 info"
		else
			log_info "Generating new keys for validator${i}"
			execute_quiet "${bin} keys add validator${i} --keyring-backend test --home ${local_env}/validator${i}" "${KEYGEN_LOG}" "Generate validator${i} key"
			execute_quiet "${bin} keys add relayer${i} --keyring-backend test --home ${local_env}/relayer${i}" "${KEYGEN_LOG}" "Generate relayer${i} key"
		fi
		execute_quiet "${bin} keys add validator_bls${i} --keyring-backend test --home ${local_env}/validator${i} --algo eth_bls" "${KEYGEN_LOG}" "Generate validator${i} BLS key"
		execute_quiet "${bin} keys add challenger${i} --keyring-backend test --home ${local_env}/challenger${i}" "${KEYGEN_LOG}" "Generate challenger${i} key"
		end_step
	done

	# add sp accounts
	start_step "Storage Provider Initialization"
	sp_size=1
	if [ $# -eq 2 ]; then
		sp_size=$2
	fi
	log_info "Starting initialization of ${sp_size} storage providers"
	for ((i = 0; i < ${sp_size}; i++)); do
		execute_quiet "mkdir -p ${local_env}/sp${i}" "${INIT_LOG}" "Create SP${i} directory"
		if [ "$i" -eq 0 ]; then
			log_info "Importing main SP preset keys"
			execute_with_mode "${bin} keys import sp0 ${sp0_prikey} --secp256k1-private-key --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Import SP0 operator key"
			execute_quiet "${bin} keys show sp0 --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Generate SP0 operator info"
		else
			log_info "Generating new keys for SP${i}"
			execute_quiet "${bin} keys add sp${i} --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Generate SP${i} operator key"
		fi
		execute_quiet "${bin} keys add sp${i}_fund --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Generate SP${i} funding key"
		execute_quiet "${bin} keys add sp${i}_seal --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Generate SP${i} seal key"
		execute_quiet "${bin} keys add sp${i}_bls --keyring-backend test --home ${local_env}/sp${i} --algo eth_bls" "${KEYGEN_LOG}" "Generate SP${i} BLS key"
		execute_quiet "${bin} keys add sp${i}_approval --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Generate SP${i} approval key"
		execute_quiet "${bin} keys add sp${i}_gc --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Generate SP${i} garbage collection key"
		execute_quiet "${bin} keys add sp${i}_maintenance --keyring-backend test --home ${local_env}/sp${i}" "${KEYGEN_LOG}" "Generate SP${i} maintenance key"
	done
	end_step
}

function generate_genesis() {
	size=$1
	sp_size=1
	if [ $# -eq 2 ]; then
		sp_size=$2
	fi

	start_step "Genesis File Generation"
	log_info "Starting genesis file generation (Validators: ${size}, SPs: ${sp_size})"

	declare -a addrs=(
		"0x1111102dd32160b064f2a512cdef74bfdb6a9f96"
		"0x2222207b1f7b8d37566d9a2778732451dbfbc5d0"
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
		"0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
		"0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
		"0x90F79bf6EB2c4f870365E785982E1f101E93b906"
		"0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"
		"0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"
		"0x976EA74026E726554dB657fA54763abd0C3a0aa9"
		"0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
		"0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f"
		"0xa0Ee7A142d267C1f36714E4a8F75612F20a79720"
		"0x000000000000000000000000000000000000dead"
		"0x76d244CE05c3De4BbC6fDd7F56379B145709ade9"
		"0xF03Af05bE1a3ADF6Df23E7ee9A151718284FAE1a"
	)

	declare -a validator_addrs=()
	for ((i = 0; i < ${size}; i++)); do
		# export validator addresses
		validator_addrs+=("$(${bin} keys show validator${i} -a --keyring-backend test --home ${local_env}/validator${i})")
	done

	declare -a deletgator_addrs=()
	for ((i = 0; i < ${size}; i++)); do
		# export delegator addresses
		deletgator_addrs+=("$(${bin} keys show validator${i} -a --keyring-backend test --home ${local_env}/validator${i})")
	done

	declare -a relayer_addrs=()
	for ((i = 0; i < ${size}; i++)); do
		# export validator addresses
		relayer_addrs+=("$(${bin} keys show relayer${i} -a --keyring-backend test --home ${local_env}/relayer${i})")
	done

	declare -a challenger_addrs=()
	for ((i = 0; i < ${size}; i++)); do
		# export validator addresses
		challenger_addrs+=("$(${bin} keys show challenger${i} -a --keyring-backend test --home ${local_env}/challenger${i})")
	done

	mkdir -p ${local_env}/gentx
	for ((i = 0; i < ${size}; i++)); do
		if [ "$i" -eq 0 ]; then
			for addr in "${addrs[@]}"; do
				# preallocate funds for testing purposes.
				${bin} add-genesis-account "$addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator${i}
			done
			devaccount_addr=$(${bin} keys show devaccount -a --keyring-backend test --home ${local_env}/validator${i})
			${bin} add-genesis-account "${devaccount_addr}" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator${i}
		fi
		for validator_addr in "${validator_addrs[@]}"; do
			# init genesis account in genesis state
			${bin} add-genesis-account "$validator_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator${i}
		done

		#for deletgator_addr in "${deletgator_addrs[@]}"; do
		# init genesis account in genesis state
		#${bin} add-genesis-account "$deletgator_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator${i}
		#done

		for relayer_addr in "${relayer_addrs[@]}"; do
			# init genesis account in genesis state
			${bin} add-genesis-account "$relayer_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator${i}
		done

		for challenger_addr in "${challenger_addrs[@]}"; do
			# init genesis account in genesis state
			${bin} add-genesis-account "$challenger_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator${i}
		done

		rm -rf ${local_env}/validator${i}/config/gentx/

		validatorAddr=${validator_addrs[$i]}
		deletgatorAddr=${deletgator_addrs[$i]}
		relayerAddr="$(${bin} keys show relayer${i} -a --keyring-backend test --home ${local_env}/relayer${i})"
		challengerAddr="$(${bin} keys show challenger${i} -a --keyring-backend test --home ${local_env}/challenger${i})"
		blsKey="$(${bin} keys show validator_bls${i} --keyring-backend test --home ${local_env}/validator${i} --output json | jq -r .pubkey_hex)"
		blsProof="$(${bin} keys sign "${blsKey}" --from validator_bls${i} --keyring-backend test --home ${local_env}/validator${i})"

		# create bond validator tx
		${bin} gentx "validator${i}" "${STAKING_BOND_AMOUNT}${STAKING_BOND_DENOM}" "$validatorAddr" "$deletgatorAddr" "$relayerAddr" "$challengerAddr" "$blsKey" "$blsProof" \
			--home ${local_env}/validator${i} \
			--keyring-backend=test \
			--chain-id="${CHAIN_ID}" \
			--moniker="validator${i}" \
			--commission-max-change-rate="${COMMISSION_MAX_CHANGE_RATE}" \
			--commission-max-rate="${COMMISSION_MAX_RATE}" \
			--commission-rate="${COMMISSION_RATE}" \
			--details="validator${i}" \
			--website="http://website" \
			--node tcp://localhost:$((${VALIDATOR_RPC_PORT_START} + ${i})) \
			--node-id "validator${i}" \
			--ip 127.0.0.1 \
			--gas ""
		cp ${local_env}/validator${i}/config/gentx/gentx-validator${i}.json ${local_env}/gentx/
	done

	node_ids=""
	# bond validator tx in genesis state
	for ((i = 0; i < ${size}; i++)); do
		cp ${local_env}/gentx/* ${local_env}/validator${i}/config/gentx/
		execute_with_mode "${bin} collect-gentxs --home ${local_env}/validator${i}" "${GENESIS_LOG}" "Collect genesis transactions for validator${i}"
		node_ids="$(${bin} tendermint show-node-id --home ${local_env}/validator${i})@127.0.0.1:$((${VALIDATOR_P2P_PORT_START} + ${i})) ${node_ids}"
	done

	# generate sp to genesis
	generate_sp_genesis "$size" "$sp_size"

	persistent_peers=$(joinByString ',' ${node_ids})
	for ((i = 0; i < ${size}; i++)); do
		if [ "$i" -gt 0 ]; then
			cp ${local_env}/validator0/config/genesis.json ${local_env}/validator${i}/config/
		fi
		sed -i -e "s/minimum-gas-prices = \"0amoca\"/minimum-gas-prices = \"5000000000${BASIC_DENOM}\"/g" ${local_env}/*/config/app.toml
		sed -i -e "s/\"stake\"/\"${BASIC_DENOM}\"/g" ${local_env}/validator${i}/config/genesis.json
		#sed -i -e "s/\"no_base_fee\": false/\"no_base_fee\": true/g" ${local_env}/*/config/genesis.json
		sed -i -e "s/\"denom_metadata\": \[\]/\"denom_metadata\": \[${NATIVE_COIN_DESC}\]/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/seeds = \"[^\"]*\"/seeds = \"\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/persistent_peers = \".*\"/persistent_peers = \"${persistent_peers}\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/timeout_propose = \"3s\"/timeout_propose = \"600ms\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/timeout_propose_delta = \"500ms\"/timeout_propose_delta = \"200ms\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/timeout_prevote = \"1s\"/timeout_prevote = \"500ms\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/timeout_prevote_delta = \"500ms\"/timeout_prevote_delta = \"200ms\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/timeout_precommit = \"1s\"/timeout_precommit = \"500ms\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/timeout_precommit_delta = \"500ms\"/timeout_precommit_delta = \"200ms\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/timeout_commit = \"3s\"/timeout_commit = \"500ms\"/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/addr_book_strict = true/addr_book_strict = false/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/allow_duplicate_ip = false/allow_duplicate_ip = true/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s/snapshot-interval = 0/snapshot-interval = ${SNAPSHOT_INTERVAL}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/src-chain-id = 1/src-chain-id = ${SRC_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-bsc-chain-id = 2/dest-bsc-chain-id = ${DEST_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-op-chain-id = 3/dest-op-chain-id = ${DEST_OP_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-polygon-chain-id = 4/dest-polygon-chain-id = ${DEST_POLYGON_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-scroll-chain-id = 5/dest-scroll-chain-id = ${DEST_SCROLL_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-linea-chain-id = 6/dest-linea-chain-id = ${DEST_LINEA_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-mantle-chain-id = 7/dest-mantle-chain-id = ${DEST_MANTLE_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-arbitrum-chain-id = 8/dest-arbitrum-chain-id = ${DEST_ARBITRUM_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-optimism-chain-id = 9/dest-optimism-chain-id = ${DEST_OPTIMISM_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/dest-base-chain-id = 10/dest-base-chain-id = ${DEST_BASE_CHAIN_ID}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/snapshot-keep-recent = 2/snapshot-keep-recent = ${SNAPSHOT_KEEP_RECENT}/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/enabled-unsafe-cors = false/enabled-unsafe-cors = true/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/pruning = \"default\"/pruning = \"nothing\"/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/eth,net,web3/eth,txpool,personal,net,debug,web3/g" ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/\"reserve_time\": \"15552000\"/\"reserve_time\": \"60\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"forced_settle_time\": \"86400\"/\"forced_settle_time\": \"30\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"signed_blocks_window\": \"100\"/\"signed_blocks_window\": \"10000\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"min_gas_price\": \"0.000000000000000000\"/\"min_gas_price\": \"1000000000.000000000000000000\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/172800s/${DEPOSIT_VOTE_PERIOD}/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"10000000\"/\"${GOV_MIN_DEPOSIT_AMOUNT}\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"max_bytes\": \"22020096\"/\"max_bytes\": \"1048576\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"challenge_count_per_block\": \"1\"/\"challenge_count_per_block\": \"5\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"challenge_keep_alive_period\": \"300\"/\"challenge_keep_alive_period\": \"10\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"heartbeat_interval\": \"1000\"/\"heartbeat_interval\": \"100\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"attestation_inturn_interval\": \"120\"/\"attestation_inturn_interval\": \"10\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"discontinue_confirm_period\": \"604800\"/\"discontinue_confirm_period\": \"5\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"discontinue_deletion_max\": \"100\"/\"discontinue_deletion_max\": \"2\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"update_global_price_interval\": \"0\"/\"update_global_price_interval\": \"1\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"update_price_disallowed_days\": 2/\"update_price_disallowed_days\": 0/g" ${local_env}/validator${i}/config/genesis.json
		#sed -i -e "s/\"community_tax\": \"0.020000000000000000\"/\"community_tax\": \"0\"/g" ${local_env}/validator${i}/config/genesis.json
		# sed -i -e "s/\"voting_period\": \"30s\"/\"voting_period\": \"5s\"/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"redundant_data_chunk_num\": 4/\"redundant_data_chunk_num\": 1/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/\"redundant_parity_chunk_num\": 2/\"redundant_parity_chunk_num\": 1/g" ${local_env}/validator${i}/config/genesis.json
		sed -i -e "s/log_level = \"info\"/\log_level= \"debug\"/g" ${local_env}/validator${i}/config/config.toml
		#echo -e '[payment-check]\nenabled = true\ninterval = 1' >> ${local_env}/validator${i}/config/app.toml
		sed -i -e "s/cors_allowed_origins = \[\]/cors_allowed_origins = \[\"*\"\]/g" ${local_env}/validator${i}/config/config.toml
		sed -i -e "s#node = \"tcp://localhost:26657\"#node = \"tcp://localhost:$((${VALIDATOR_RPC_PORT_START} + ${i}))\"#g" ${local_env}/validator${i}/config/client.toml
		sed -i -e "/Address defines the gRPC server address to bind to/{N;s/address = \"localhost:9090\"/address = \"localhost:$((${VALIDATOR_GRPC_PORT_START} + ${i}))\"/;}" ${local_env}/validator${i}/config/app.toml
		sed -i -e "/Address defines the gRPC-web server address to bind to/{N;s/address = \"localhost:9091\"/address = \"localhost:$((${VALIDATOR_GRPC_PORT_START} - 1 - ${i}))\"/;}" ${local_env}/validator${i}/config/app.toml
		sed -i -e "/Address defines the EVM RPC HTTP server address to bind to/{N;s/address = \"127.0.0.1:8545\"/address = \"127.0.0.1:$((${EVM_SERVER_PORT_START} + ${i} * 2))\"/;}" ${local_env}/validator${i}/config/app.toml
		sed -i -e "/Address defines the EVM WebSocket server address to bind to/{N;s/address = \"127.0.0.1:8546\"/address = \"127.0.0.1:$((${EVM_SERVER_PORT_START} + 1 + ${i} * 2))\"/;}" ${local_env}/validator${i}/config/app.toml
	done

	# enable swagger API for validator0
	execute_with_mode "sed -i -e \"/Enable defines if the API server should be enabled/{N;s/enable = false/enable = true/;}\" ${local_env}/validator0/config/app.toml" "${CONFIG_LOG}" "Enable API server for validator0"
	execute_with_mode "sed -i -e 's/swagger = false/swagger = true/' ${local_env}/validator0/config/app.toml" "${CONFIG_LOG}" "Enable Swagger documentation for validator0"

	# enable telemetry for validator0
	execute_with_mode "sed -i -e \"/other sinks such as Prometheus/{N;s/enable = false/enable = true/;}\" ${local_env}/validator0/config/app.toml" "${CONFIG_LOG}" "Enable telemetry for validator0"

	end_step
}

function start() {
	size=$1
	start_step "Node Startup"
	log_info "Starting ${size} validator nodes"

	for ((i = 0; i < ${size}; i++)); do
		execute_quiet "mkdir -p ${local_env}/validator${i}/logs" "${START_LOG}" "Create validator${i} log directory"
		log_info "Starting validator node ${i}"

		local start_cmd="nohup \"${bin}\" start --home ${local_env}/validator${i} \
			--keyring-backend test \
			--api.enabled-unsafe-cors true \
			--address 0.0.0.0:$((${VALIDATOR_ADDRESS_PORT_START} + ${i})) \
			--grpc.address 0.0.0.0:$((${VALIDATOR_GRPC_PORT_START} + ${i})) \
			--p2p.laddr tcp://0.0.0.0:$((${VALIDATOR_P2P_PORT_START} + ${i})) \
			--p2p.external-address 127.0.0.1:$((${VALIDATOR_P2P_PORT_START} + ${i})) \
			--rpc.laddr tcp://0.0.0.0:$((${VALIDATOR_RPC_PORT_START} + ${i})) \
			--rpc.unsafe true \
			--log_format json >${local_env}/validator${i}/logs/node.log 2>&1 &"

		execute_with_mode "$start_cmd" "${START_LOG}" "Start validator${i} node process"

		# Record started process information
		sleep 1
		local pid=$(ps -ef | grep "${bin}" | grep "validator${i}" | grep -v grep | awk '{print $2}' | head -1)
		if [ -n "$pid" ]; then
			log_info "Validator${i} started, PID: ${pid}"
			echo "Validator${i} PID: ${pid} start time: $(date)" >> "${START_LOG}"
		else
			log_warn "Validator${i} startup status unknown"
		fi
	done

	end_step
}

function stop() {
	start_step "Node Shutdown"

	# First check how many processes are running
	running_pids=$(ps -ef | grep ${bin_name} | grep validator | grep -v grep | awk '{print $2}')
	process_count=$(echo "$running_pids" | grep -v '^$' | wc -l)

	if [ "$process_count" -eq 0 ]; then
		log_info "No running ${bin_name} processes found"
		end_step
		return 0
	fi

	log_info "Found ${process_count} running ${bin_name} processes, shutting down..."

	# Show details of processes being shut down
	echo "=== Process Shutdown Details ===" >> "${STOP_LOG}"
	echo "Shutdown time: $(date)" >> "${STOP_LOG}"
	echo "Processes found: ${process_count}" >> "${STOP_LOG}"
	ps -ef | grep ${bin_name} | grep validator | grep -v grep | while read line; do
		pid=$(echo "$line" | awk '{print $2}')
		home_dir=$(echo "$line" | grep -o '\-\-home [^ ]*' | cut -d' ' -f2)
		validator_name=$(basename "$home_dir" 2>/dev/null || echo "unknown")
		log_info "  PID: $pid ($validator_name)"
		echo "PID: $pid ($validator_name)" >> "${STOP_LOG}"
	done

	# Shutdown processes
	execute_with_mode "ps -ef | grep ${bin_name} | grep validator | grep -v grep | awk '{print \$2}' | xargs kill" "${STOP_LOG}" "Send termination signal to all validator processes"

	# Wait a bit to ensure processes are closed
	log_info "Waiting for processes to shut down gracefully..."
	sleep 2

	# Check if there are any remaining processes
	remaining_pids=$(ps -ef | grep ${bin_name} | grep validator | grep -v grep | awk '{print $2}')
	remaining_count=$(echo "$remaining_pids" | grep -v '^$' | wc -l)

	if [ "$remaining_count" -eq 0 ]; then
		log_info "Successfully shut down ${process_count} ${bin_name} processes"
		echo "Successfully shut down processes: ${process_count}" >> "${STOP_LOG}"
	else
		log_warn "Force killing remaining ${remaining_count} processes..."
		execute_with_mode "echo \"$remaining_pids\" | xargs kill -9" "${STOP_LOG}" "Force terminate remaining processes"
		sleep 1
		log_info "All processes have been shut down"
	fi

	end_step
}

# create sp in genesis use genesis transaction like validator
function generate_sp_genesis {
	# create sp address in genesis
	size=$1
	sp_size=1
	if [ $# -eq 2 ]; then
		sp_size=$2
	fi
	for ((i = 0; i < ${sp_size}; i++)); do
		#create sp and sp fund account
		spoperator_addr=("$(${bin} keys show sp${i} -a --keyring-backend test --home ${local_env}/sp${i})")
		spfund_addr=("$(${bin} keys show sp${i}_fund -a --keyring-backend test --home ${local_env}/sp${i})")
		spseal_addr=("$(${bin} keys show sp${i}_seal -a --keyring-backend test --home ${local_env}/sp${i})")
		spapproval_addr=("$(${bin} keys show sp${i}_approval -a --keyring-backend test --home ${local_env}/sp${i})")
		spgc_addr=("$(${bin} keys show sp${i}_gc -a --keyring-backend test --home ${local_env}/sp${i})")
		spmaintenance_addr=("$(${bin} keys show sp${i}_maintenance -a --keyring-backend test --home ${local_env}/sp${i})")
		${bin} add-genesis-account "$spoperator_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator0
		${bin} add-genesis-account "$spfund_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator0
		${bin} add-genesis-account "$spseal_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator0
		${bin} add-genesis-account "$spapproval_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator0
		${bin} add-genesis-account "$spgc_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator0
		${bin} add-genesis-account "$spmaintenance_addr" "${GENESIS_ACCOUNT_BALANCE}""${STAKING_BOND_DENOM}" --home ${local_env}/validator0
	done

	rm -rf ${local_env}/gensptx
	mkdir -p ${local_env}/gensptx
	for ((i = 0; i < ${sp_size}; i++)); do
		cp ${local_env}/validator0/config/genesis.json ${local_env}/sp${i}/config/
		spoperator_addr=("$(${bin} keys show sp${i} -a --keyring-backend test --home ${local_env}/sp${i})")
		spfund_addr=("$(${bin} keys show sp${i}_fund -a --keyring-backend test --home ${local_env}/sp${i})")
		spseal_addr=("$(${bin} keys show sp${i}_seal -a --keyring-backend test --home ${local_env}/sp${i})")
		bls_pub_key=("$(${bin} keys show sp${i}_bls --keyring-backend test --home ${local_env}/sp${i} --output json | jq -r .pubkey_hex)")
		bls_proof=("$(${bin} keys sign "${bls_pub_key}" --from sp${i}_bls --keyring-backend test --home ${local_env}/sp${i})")
		spapproval_addr=("$(${bin} keys show sp${i}_approval -a --keyring-backend test --home ${local_env}/sp${i})")
		spgc_addr=("$(${bin} keys show sp${i}_gc -a --keyring-backend test --home ${local_env}/sp${i})")
		spmaintenance_addr=("$(${bin} keys show sp${i}_maintenance -a --keyring-backend test --home ${local_env}/sp${i})")
		# create bond storage provider tx
		${bin} spgentx "sp${i}" "${SP_MIN_DEPOSIT_AMOUNT}""${STAKING_BOND_DENOM}" \
			--home ${local_env}/sp${i} \
			--creator="${spoperator_addr}" \
			--operator-address="${spoperator_addr}" \
			--funding-address="${spfund_addr}" \
			--seal-address="${spseal_addr}" \
			--bls-pub-key="${bls_pub_key}" \
			--bls-proof="${bls_proof}" \
			--approval-address="${spapproval_addr}" \
			--gc-address="${spgc_addr}" \
			--maintenance-address="${spmaintenance_addr}" \
			--keyring-backend=test \
			--chain-id="${CHAIN_ID}" \
			--moniker="sp${i}" \
			--details="detail_sp${i}" \
			--website="http://website" \
			--endpoint="http://127.0.0.1:$((${STOREAGE_PROVIDER_ADDRESS_PORT_START} + ${i}))" \
			--node tcp://localhost:$((${VALIDATOR_RPC_PORT_START} + ${i})) \
			--node-id "sp${i}" \
			--ip 127.0.0.1 \
			--gas "" \
			--output-document=${local_env}/gensptx/gentx-sp${i}.json
	done

	rm -rf ${local_env}/validator0/config/gensptx/
	mkdir -p ${local_env}/validator0/config/gensptx
	cp ${local_env}/gensptx/* ${local_env}/validator0/config/gensptx/
	execute_with_mode "${bin} collect-spgentxs --gentx-dir ${local_env}/validator0/config/gensptx --home ${local_env}/validator0" "${GENESIS_LOG}" "Collect storage provider genesis transactions"
}

function export_validator {
	size=$1
	for ((i = 0; i < ${size}; i++)); do
		bls_priv_key=("$(echo "y" | ${bin} keys export validator_bls${i} --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/validator${i})")
		relayer_key=("$(echo "y" | ${bin} keys export relayer${i} --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/relayer${i})")

		echo "validator_bls${i} bls_priv_key: ${bls_priv_key}"
		echo "relayer${i} relayer_key: ${relayer_key}"
	done
}

function export_sps {
	size=$1
	sp_size=1
	if [ $# -eq 2 ]; then
		sp_size=$2
	fi
	output="{"
	for ((i = 0; i < ${sp_size}; i++)); do
		spoperator_addr=("$(${bin} keys show sp${i} -a --keyring-backend test --home ${local_env}/sp${i})")
		spfund_addr=("$(${bin} keys show sp${i}_fund -a --keyring-backend test --home ${local_env}/sp${i})")
		spseal_addr=("$(${bin} keys show sp${i}_seal -a --keyring-backend test --home ${local_env}/sp${i})")
		spapproval_addr=("$(${bin} keys show sp${i}_approval -a --keyring-backend test --home ${local_env}/sp${i})")
		spgc_addr=("$(${bin} keys show sp${i}_gc -a --keyring-backend test --home ${local_env}/sp${i})")
		spmaintenance_addr=("$(${bin} keys show sp${i}_maintenance -a --keyring-backend test --home ${local_env}/sp${i})")
		bls_pub_key=("$(${bin} keys show sp${i}_bls --keyring-backend test --home ${local_env}/sp${i} --output json | jq -r .pubkey_hex)")
		spoperator_priv_key=("$(echo "y" | ${bin} keys export sp${i} --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/sp${i})")
		spfund_priv_key=("$(echo "y" | ${bin} keys export sp${i}_fund --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/sp${i})")
		spseal_priv_key=("$(echo "y" | ${bin} keys export sp${i}_seal --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/sp${i})")
		spapproval_priv_key=("$(echo "y" | ${bin} keys export sp${i}_approval --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/sp${i})")
		spgc_priv_key=("$(echo "y" | ${bin} keys export sp${i}_gc --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/sp${i})")
		spmaintenance_priv_key=("$(echo "y" | ${bin} keys export sp${i}_maintenance --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/sp${i})")
		bls_priv_key=("$(echo "y" | ${bin} keys export sp${i}_bls --unarmored-hex --unsafe --keyring-backend test --home ${local_env}/sp${i})")
		output="${output}\"sp${i}\":{"
		output="${output}\"OperatorAddress\": \"${spoperator_addr}\","
		output="${output}\"FundingAddress\": \"${spfund_addr}\","
		output="${output}\"SealAddress\": \"${spseal_addr}\","
		output="${output}\"ApprovalAddress\": \"${spapproval_addr}\","
		output="${output}\"GcAddress\": \"${spgc_addr}\","
		output="${output}\"MaintenanceAddress\": \"${spmaintenance_addr}\","
		output="${output}\"BlsPubKey\": \"${bls_pub_key}\","
		output="${output}\"OperatorPrivateKey\": \"${spoperator_priv_key}\","
		output="${output}\"FundingPrivateKey\": \"${spfund_priv_key}\","
		output="${output}\"SealPrivateKey\": \"${spseal_priv_key}\","
		output="${output}\"ApprovalPrivateKey\": \"${spapproval_priv_key}\","
		output="${output}\"GcPrivateKey\": \"${spgc_priv_key}\","
		output="${output}\"MaintenancePrivateKey\": \"${spmaintenance_priv_key}\","
		output="${output}\"BlsPrivateKey\": \"${bls_priv_key}\""
		output="${output}},"
	done
	output="${output%?}}"

	# Control output based on silent mode
	if [ "$QUIET_MODE" = "true" ]; then
		# Silent mode: save to file
		sp_export_file="${local_env}/sp_export.json"
		echo "${output}" | jq . > "${sp_export_file}"
		echo "[INFO] SP configuration exported to file: ${sp_export_file}"
		echo "[INFO] Use --verbose parameter to display directly in terminal"
	else
		# Verbose mode: output directly to terminal
		echo "${output}" | jq .
	fi
}

# Check output mode parameters
for arg in "$@"; do
    case $arg in
        --quiet|-q)
            QUIET_MODE=true
            shift
            ;;
        --verbose|-v)
            QUIET_MODE=false
            shift
            ;;
    esac
done

CMD=$1
SIZE=3
SP_SIZE=3
if [ -n "$2" ] && [ "$2" -gt 0 ]; then
	SIZE=$2
fi
if [ -n "$3" ] && [ "$3" -gt 0 ]; then
	SP_SIZE=$3
fi

case ${CMD} in
init)
	init_logging
	echo "===== init ===="
	init "$SIZE" "$SP_SIZE"
	echo "===== end ===="
	generate_final_report
	;;
generate)
	init_logging
	echo "===== generate genesis ===="
	generate_genesis "$SIZE" "$SP_SIZE"
	echo "===== end ===="
	generate_final_report
	;;

export_sps)
	export_sps "$SIZE" "$SP_SIZE"
	;;

export_validator)
	export_validator "$SIZE"
	;;
start)
	init_logging
	echo "===== start ===="
	start "$SIZE"
	echo "===== end ===="
	generate_final_report
	;;
stop)
	init_logging
	echo "===== stop ===="
	stop
	echo "===== end ===="
	generate_final_report
	;;
all)
	init_logging
	cleanup_old_logs
	echo "===== Complete Moca Local Environment Deployment ===="
	log_info "Starting complete deployment flow: Stop → Initialize → Generate Genesis → Start"

	echo "===== stop ===="
	stop
	echo "===== init ===="
	init "$SIZE" "$SP_SIZE"
	echo "===== generate genesis ===="
	generate_genesis "$SIZE" "$SP_SIZE"
	echo "===== start ===="
	start "$SIZE"
	echo "===== end ===="

	generate_final_report
	;;
logs)
	list_recent_sessions
	;;
help|--help|-h)
	bash "${SCRIPT_DIR}/localup-help.sh" help
	;;
*)
	echo "Usage: localup.sh [OPTIONS] COMMAND [VALIDATORS] [STORAGE_PROVIDERS]"
	echo ""
	echo "Commands:"
	echo "  all                   Complete flow (init + genesis + start)"
	echo "  init                  Initialize environment only"
	echo "  generate              Generate genesis file only"
	echo "  start                 Start nodes only"
	echo "  stop                  Stop nodes"
	echo "  export_sps            Export storage provider info (saved to file in silent mode)"
	echo "  logs                  Show log sessions"
	echo "  help                  Show detailed help"
	echo ""
	echo "Options:"
	echo "  --quiet, -q           Silent mode (reduce terminal output, enabled by default)"
	echo "  --verbose, -v         Verbose mode (show all output to terminal)"
	echo ""
	echo "Parameters:"
	echo "  VALIDATORS            Number of validators (default: 3)"
	echo "  STORAGE_PROVIDERS     Number of storage providers (default: 3)"
	echo ""
	echo "Examples:"
	echo "  bash localup.sh all 1 1              # Start 1 validator and 1 SP (silent mode)"
	echo "  bash localup.sh --verbose all 1 3    # Verbose mode: start 1 validator and 3 SPs"
	echo "  bash localup.sh export_sps 1 1       # Export SP info to file (silent mode)"
	echo "  bash localup.sh --verbose export_sps 1 1  # Display SP info directly in terminal"
	echo "  bash localup.sh stop                 # Stop all nodes"
	echo ""
	echo "For detailed help with examples, run: bash localup.sh help"
	;;
esac
