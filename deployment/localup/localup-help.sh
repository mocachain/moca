#!/usr/bin/env bash

# Moca Local Development Environment Setup Script
# Moca blockchain node startup script for local development environment

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

show_banner() {
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}    Moca Local Development Environment  ${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
}

show_help() {
    show_banner

    echo -e "${GREEN}Usage:${NC}"
    echo -e "  ${BLUE}bash localup.sh [COMMAND] [VALIDATORS] [SPs]${NC}"
    echo ""

    echo -e "${GREEN}Commands:${NC}"
    echo -e "  ${YELLOW}all${NC}           Complete startup flow (stop→init→generate genesis→start)"
    echo -e "  ${YELLOW}init${NC}          Initialize node configuration and keys"
    echo -e "  ${YELLOW}generate${NC}      Generate genesis block file"
    echo -e "  ${YELLOW}start${NC}         Start all validator nodes"
    echo -e "  ${YELLOW}stop${NC}          Stop all validator nodes (shows count and details of closed processes)"
    echo -e "  ${YELLOW}export_sps${NC}    Export storage provider configuration to JSON file"
    echo -e "  ${YELLOW}export_validator${NC} Export validator configuration to JSON file"
    echo -e "  ${YELLOW}logs${NC}          Show recent log sessions"
    echo -e "  ${YELLOW}help${NC}          Show this help information"
    echo ""

    echo -e "${GREEN}Parameters:${NC}"
    echo -e "  ${BLUE}VALIDATORS${NC}    Number of validator nodes (default: 3)"
    echo -e "  ${BLUE}SPs${NC}           Number of storage provider nodes (default: 3)"
    echo ""

    echo -e "${GREEN}Example Usage:${NC}"
    echo -e "  ${PURPLE}# Start complete environment with default config (3 validators + 3 SPs)${NC}"
    echo -e "  bash localup.sh all"
    echo ""
    echo -e "  ${PURPLE}# Start 1 validator and 3 storage providers${NC}"
    echo -e "  bash localup.sh all 1 3"
    echo ""
    echo -e "  ${PURPLE}# Initialize 4 validator nodes only${NC}"
    echo -e "  bash localup.sh init 4"
    echo ""
    echo -e "  ${PURPLE}# Generate genesis block (4 validators + 2 SPs)${NC}"
    echo -e "  bash localup.sh generate 4 2"
    echo ""
    echo -e "  ${PURPLE}# Start pre-configured nodes${NC}"
    echo -e "  bash localup.sh start"
    echo ""
    echo -e "  ${PURPLE}# Stop all nodes${NC}"
    echo -e "  bash localup.sh stop"
    echo ""
    echo -e "  ${PURPLE}# Export SP configuration for moca-storage-provider use${NC}"
    echo -e "  bash localup.sh export_sps 1 3 > ../moca-storage-provider/deployment/localup/sp.json"
    echo ""
    echo -e "  ${PURPLE}# View recent log sessions${NC}"
    echo -e "  bash localup.sh logs"
    echo ""

    echo -e "${GREEN}Common Workflows:${NC}"
    echo -e "  ${BLUE}1. Complete development environment startup:${NC}"
    echo -e "     make clean && make build"
    echo -e "     rm -rf ./deployment/localup/.local"
    echo -e "     bash ./deployment/localup/localup.sh all 1 3"
    echo ""
    echo -e "  ${BLUE}2. Restart nodes only (preserve data):${NC}"
    echo -e "     bash ./deployment/localup/localup.sh stop"
    echo -e "     bash ./deployment/localup/localup.sh start"
    echo ""
    echo -e "  ${BLUE}3. Complete reinitialization:${NC}"
    echo -e "     bash ./deployment/localup/localup.sh stop"
    echo -e "     rm -rf ./deployment/localup/.local"
    echo -e "     bash ./deployment/localup/localup.sh all"
    echo ""

    echo -e "${GREEN}Generated File Locations:${NC}"
    echo -e "  ${BLUE}Node configurations:${NC}     ./deployment/localup/.local/"
    echo -e "  ${BLUE}Validator keys:${NC}   ./deployment/localup/.local/validator*/info"
    echo -e "  ${BLUE}SP keys:${NC}       ./deployment/localup/.local/sp*/info"
    echo -e "  ${BLUE}Genesis file:${NC}     ./deployment/localup/.local/validator0/config/genesis.json"
    echo -e "  ${BLUE}Execution logs:${NC}     ./deployment/localup/.local/logs/[session-id]/"
    echo ""

    echo -e "${GREEN}Log File Descriptions:${NC}"
    echo -e "  ${BLUE}01_init.log${NC}      Node initialization logs"
    echo -e "  ${BLUE}02_keygen.log${NC}    Key generation logs"
    echo -e "  ${BLUE}03_genesis.log${NC}   Genesis file generation logs"
    echo -e "  ${BLUE}04_config.log${NC}    Configuration modification logs"
    echo -e "  ${BLUE}05_start.log${NC}     Node startup logs"
    echo -e "  ${BLUE}06_stop.log${NC}      Node shutdown logs"
    echo -e "  ${BLUE}summary.log${NC}      Execution summary logs"
    echo -e "  ${BLUE}error.log${NC}        Error and warning logs"
    echo ""

    echo -e "${GREEN}Network Endpoints:${NC}"
    echo -e "  ${BLUE}RPC:${NC}          http://localhost:26657"
    echo -e "  ${BLUE}API:${NC}          http://localhost:1317"
    echo -e "  ${BLUE}gRPC:${NC}         localhost:9090"
    echo -e "  ${BLUE}EVM RPC:${NC}      http://localhost:8545"
    echo -e "  ${BLUE}EVM WebSocket:${NC} ws://localhost:8546"
    echo ""

    echo -e "${GREEN}Pre-configured Accounts:${NC}"
    echo -e "  ${BLUE}Development account:${NC}     devaccount"
    echo -e "  ${BLUE}Validators:${NC}       validator0, validator1, ..."
    echo -e "  ${BLUE}Relayers:${NC}       relayer0, relayer1, ..."
    echo -e "  ${BLUE}Challengers:${NC}       challenger0, challenger1, ..."
    echo -e "  ${BLUE}Storage providers:${NC}   sp0, sp1, ..."
    echo ""

    echo -e "${GREEN}Important Notes:${NC}"
    echo -e "  • Ensure mocad binary is built: ${YELLOW}make build${NC}"
    echo -e "  • All private keys are preset test keys for development only"
    echo -e "  • Node data is stored in ${YELLOW}./deployment/localup/.local/${NC} directory"
    echo -e "  • Uses ${YELLOW}--keyring-backend test${NC} for key management"
    echo ""

    echo -e "${RED}WARNING:${NC}"
    echo -e "  This script is for development and testing only, ${RED}DO NOT use in production${NC}!"
    echo ""
}

# Check parameters
if [ "$1" = "help" ] || [ "$1" = "--help" ] || [ "$1" = "-h" ] || [ -z "$1" ]; then
    show_help
    exit 0
fi

# If not help command, call the original localup.sh script
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
exec "$SCRIPT_DIR/localup.sh" "$@"
