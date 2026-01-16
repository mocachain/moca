#!/usr/bin/env bash

# Moca Docker LocalUp Logging Management System
# Based on deployment/localup/log-manager.sh adapted for Docker environment

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Global variables
CURRENT_STEP=""
STEP_START_TIME=""
LOG_SESSION_DIR=""
INIT_LOG=""
KEYGEN_LOG=""
GENESIS_LOG=""
CONFIG_LOG=""
START_LOG=""
STOP_LOG=""
ERROR_LOG=""
SUMMARY_LOG=""

# Initialize logging system
init_logging() {
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    LOG_SESSION_DIR="${SCRIPT_DIR}/.local/logs/${timestamp}"
    
    # Create log directory
    mkdir -p "${LOG_SESSION_DIR}"
    
    # Define log file paths
    INIT_LOG="${LOG_SESSION_DIR}/01_init.log"
    KEYGEN_LOG="${LOG_SESSION_DIR}/02_keygen.log" 
    GENESIS_LOG="${LOG_SESSION_DIR}/03_genesis.log"
    CONFIG_LOG="${LOG_SESSION_DIR}/04_config.log"
    START_LOG="${LOG_SESSION_DIR}/05_start.log"
    STOP_LOG="${LOG_SESSION_DIR}/06_stop.log"
    ERROR_LOG="${LOG_SESSION_DIR}/error.log"
    SUMMARY_LOG="${LOG_SESSION_DIR}/summary.log"
    
    # Create log files
    touch "${INIT_LOG}" "${KEYGEN_LOG}" "${GENESIS_LOG}" "${CONFIG_LOG}" "${START_LOG}" "${STOP_LOG}" "${ERROR_LOG}" "${SUMMARY_LOG}"
    
    # Record session start
    local session_id=$(basename "${LOG_SESSION_DIR}")
    echo -e "${CYAN}[INFO]${NC} Logging system initialized"
    echo -e "${CYAN}[INFO]${NC} Log directory: ${LOG_SESSION_DIR}"
    echo -e "${CYAN}[INFO]${NC} Session ID: ${session_id}"
    
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] Docker LocalUp session started (${session_id})" >> "${SUMMARY_LOG}"
}

# Start step
start_step() {
    local step_name="$1"
    CURRENT_STEP="$step_name"
    STEP_START_TIME=$(date +%s)
    
    echo ""
    echo -e "${GREEN}=== Start Step: ${step_name} ($(date)) ===${NC}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [STEP] Start: ${step_name}" >> "${SUMMARY_LOG}"
}

# End step
end_step() {
    if [ -n "$CURRENT_STEP" ] && [ -n "$STEP_START_TIME" ]; then
        local end_time=$(date +%s)
        local duration=$((end_time - STEP_START_TIME))
        
        echo -e "${GREEN}=== Complete Step: ${CURRENT_STEP} (Duration: ${duration}s) ===${NC}"
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [STEP] Complete: ${CURRENT_STEP} (Duration: ${duration}s)" >> "${SUMMARY_LOG}"
        
        CURRENT_STEP=""
        STEP_START_TIME=""
    fi
}

# Record information log
log_info() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${CYAN}[INFO]${NC} ${message}"
    echo "[${timestamp}] [INFO] ${message}" >> "${SUMMARY_LOG}"
}

# Record warning log
log_warn() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${YELLOW}[WARN]${NC} ${message}"
    echo "[${timestamp}] [WARN] ${message}" >> "${SUMMARY_LOG}"
    echo "[${timestamp}] [WARN] ${message}" >> "${ERROR_LOG}"
}

# Record error log
log_error() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${RED}[ERROR]${NC} ${message}"
    echo "[${timestamp}] [ERROR] ${message}" >> "${SUMMARY_LOG}"
    echo "[${timestamp}] [ERROR] ${message}" >> "${ERROR_LOG}"
}

# Record command execution
log_command() {
    local cmd="$1"
    local log_file="$2"
    local description="$3"
    local show_command="${4:-true}"
    
    if [ -n "$description" ]; then
        log_info "$description"
    fi
    
    # Only show command in verbose mode or when explicitly requested
    if [ "$show_command" = "true" ]; then
        log_info "Executing command: $cmd"
    fi
    
    echo "=== Command: $cmd ===" >> "$log_file"
    echo "Time: $(date)" >> "$log_file"
    echo "Step: ${CURRENT_STEP}" >> "$log_file"
    echo "" >> "$log_file"
}

# Execute command and record log
execute_logged() {
    local cmd="$1"
    local log_file="$2"
    local description="$3"
    local show_output="${4:-true}"
    
    # In silent mode, do not show specific execution commands
    log_command "$cmd" "$log_file" "$description" "$show_output"
    
    if [ "$show_output" = "true" ]; then
        # Display output to terminal and record to log file
        eval "$cmd" 2>&1 | tee -a "$log_file"
        local exit_code=${PIPESTATUS[0]}
    else
        # Only record to log file
        eval "$cmd" >> "$log_file" 2>&1
        local exit_code=$?
    fi
    
    echo "" >> "$log_file"
    
    if [ $exit_code -ne 0 ]; then
        log_error "Command execution failed: $cmd (exit code: $exit_code)"
        return $exit_code
    fi
    
    return 0
}

# Execute command and hide output (only record to log)
execute_quiet() {
    local cmd="$1"
    local log_file="$2"
    local description="$3"
    
    execute_logged "$cmd" "$log_file" "$description" "false"
}

# Generate final report
generate_final_report() {
    if [ -z "$LOG_SESSION_DIR" ]; then
        return
    fi
    
    local session_id=$(basename "${LOG_SESSION_DIR}")
    echo ""
    echo -e "${CYAN}[INFO]${NC} Execution completed!"
    echo -e "${CYAN}[INFO]${NC} Log location: ${LOG_SESSION_DIR}"
    echo -e "${CYAN}[INFO]${NC} View summary: cat ${SUMMARY_LOG}"
    echo ""
    
    echo -e "${BLUE}=== Log File Summary ===${NC}"
    echo "Session directory: ${LOG_SESSION_DIR}"
    echo "Summary log: ${SUMMARY_LOG}"
    echo "Initialization log: ${INIT_LOG}"
    echo "Key generation log: ${KEYGEN_LOG}"
    echo "Genesis file log: ${GENESIS_LOG}"
    echo "Configuration log: ${CONFIG_LOG}"
    echo "Startup log: ${START_LOG}"
    echo "Shutdown log: ${STOP_LOG}"
    echo "Error log: ${ERROR_LOG}"
    
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] Docker LocalUp session ended (${session_id})" >> "${SUMMARY_LOG}"
}

# List recent log sessions
list_recent_sessions() {
    local logs_dir="${SCRIPT_DIR}/.local/logs"
    
    if [ ! -d "$logs_dir" ]; then
        echo "Log directory not found: $logs_dir"
        return
    fi
    
    echo -e "${BLUE}=== Recent Log Sessions ===${NC}"
    ls -lt "$logs_dir" | head -10 | while read line; do
        if [[ $line == d* ]]; then
            local session=$(echo "$line" | awk '{print $9}')
            if [ -n "$session" ] && [ "$session" != "." ] && [ "$session" != ".." ]; then
                local summary_file="$logs_dir/$session/summary.log"
                if [ -f "$summary_file" ]; then
                    local first_line=$(head -1 "$summary_file")
                    echo "Session: $session - $first_line"
                else
                    echo "Session: $session - No summary information"
                fi
            fi
        fi
    done
}

# Clean old logs (keep last 10 sessions)
cleanup_old_logs() {
    local logs_dir="${SCRIPT_DIR}/.local/logs"
    
    if [ ! -d "$logs_dir" ]; then
        return
    fi
    
    # Keep the last 10 sessions, delete older ones
    ls -t "$logs_dir" | tail -n +11 | while read old_session; do
        if [ -d "$logs_dir/$old_session" ]; then
            echo "Cleaning old log session: $old_session"
            rm -rf "$logs_dir/$old_session"
        fi
    done
}

# Export variables for use by other scripts
export LOG_SESSION_DIR INIT_LOG KEYGEN_LOG GENESIS_LOG CONFIG_LOG START_LOG STOP_LOG ERROR_LOG SUMMARY_LOG
export -f init_logging start_step end_step log_info log_warn log_error log_command execute_logged execute_quiet generate_final_report
