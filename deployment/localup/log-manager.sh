#!/usr/bin/env bash

# Moca LocalUp Logging Management Module
# For managing and categorizing various log outputs from localup.sh

# Log directory settings
LOG_BASE_DIR="${SCRIPT_DIR}/.local/logs"
LOG_TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
LOG_SESSION_DIR="${LOG_BASE_DIR}/${LOG_TIMESTAMP}"

# Log file definitions
INIT_LOG="${LOG_SESSION_DIR}/01_init.log"
KEYGEN_LOG="${LOG_SESSION_DIR}/02_keygen.log"
GENESIS_LOG="${LOG_SESSION_DIR}/03_genesis.log"
CONFIG_LOG="${LOG_SESSION_DIR}/04_config.log"
START_LOG="${LOG_SESSION_DIR}/05_start.log"
STOP_LOG="${LOG_SESSION_DIR}/06_stop.log"
ERROR_LOG="${LOG_SESSION_DIR}/error.log"
SUMMARY_LOG="${LOG_SESSION_DIR}/summary.log"

# Current step tracking
CURRENT_STEP=""
STEP_START_TIME=""

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
GRAY='\033[0;37m'
NC='\033[0m' # No Color

# Initialize logging system
init_logging() {
    mkdir -p "${LOG_SESSION_DIR}"
    
    # Ensure all log files can be created
    touch "${INIT_LOG}" "${KEYGEN_LOG}" "${GENESIS_LOG}" "${CONFIG_LOG}" "${START_LOG}" "${STOP_LOG}" "${ERROR_LOG}" "${SUMMARY_LOG}"
    
    # Create session information file
    cat > "${LOG_SESSION_DIR}/session_info.txt" << EOF
Moca LocalUp Log Session
==================
Start time: $(date)
Command: $0 $@
User: $(whoami)
Host: $(hostname)
Working directory: $(pwd)
Binary file: ${bin}
Chain ID: ${CHAIN_ID:-Not set}
Validator count: ${SIZE:-3}
SP count: ${SP_SIZE:-3}
==================
EOF
    
    # Initialize summary log
    echo "=== Moca LocalUp Execution Summary ===" > "${SUMMARY_LOG}"
    echo "Session ID: ${LOG_TIMESTAMP}" >> "${SUMMARY_LOG}"
    echo "Start time: $(date)" >> "${SUMMARY_LOG}"
    echo "" >> "${SUMMARY_LOG}"
    
    log_info "Logging system initialized"
    log_info "Log directory: ${LOG_SESSION_DIR}"
    log_info "Session ID: ${LOG_TIMESTAMP}"
}

# Start new step
start_step() {
    local step_name="$1"
    CURRENT_STEP="$step_name"
    STEP_START_TIME=$(date +%s)
    
    local step_header="=== Start Step: ${step_name} ($(date)) ==="
    echo -e "${BLUE}${step_header}${NC}"
    echo "${step_header}" >> "${SUMMARY_LOG}"
}

# End current step
end_step() {
    if [ -n "$CURRENT_STEP" ] && [ -n "$STEP_START_TIME" ]; then
        local end_time=$(date +%s)
        local duration=$((end_time - STEP_START_TIME))
        local step_footer="=== Complete Step: ${CURRENT_STEP} (Duration: ${duration}s) ==="
        
        echo -e "${GREEN}${step_footer}${NC}"
        echo "${step_footer}" >> "${SUMMARY_LOG}"
        echo "" >> "${SUMMARY_LOG}"
        
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
    local total_end_time=$(date +%s)
    local session_start_time=$(stat -f %B "${LOG_SESSION_DIR}" 2>/dev/null || echo $total_end_time)
    local total_duration=$((total_end_time - session_start_time))
    
    echo "" >> "${SUMMARY_LOG}"
    echo "=== Execution Complete ===" >> "${SUMMARY_LOG}"
    echo "End time: $(date)" >> "${SUMMARY_LOG}"
    echo "Total duration: ${total_duration}s" >> "${SUMMARY_LOG}"
    echo "" >> "${SUMMARY_LOG}"
    
    # Count statistics for each log file
    echo "=== Log File Statistics ===" >> "${SUMMARY_LOG}"
    for log_file in "${LOG_SESSION_DIR}"/*.log; do
        if [ -f "$log_file" ]; then
            local size=$(wc -l < "$log_file" 2>/dev/null || echo "0")
            local filename=$(basename "$log_file")
            echo "${filename}: ${size} lines" >> "${SUMMARY_LOG}"
        fi
    done
    
    # Error statistics
    if [ -f "${ERROR_LOG}" ]; then
        local error_count=$(wc -l < "${ERROR_LOG}" 2>/dev/null || echo "0")
        echo "Total errors/warnings: ${error_count}" >> "${SUMMARY_LOG}"
    fi
    
    log_info "Execution completed!"
    log_info "Log location: ${LOG_SESSION_DIR}"
    log_info "View summary: cat ${SUMMARY_LOG}"
    
    echo ""
    echo -e "${CYAN}=== Log File Summary ===${NC}"
    echo -e "${GRAY}Session directory: ${LOG_SESSION_DIR}${NC}"
    echo -e "${GRAY}Summary log: ${SUMMARY_LOG}${NC}"
    echo -e "${GRAY}Initialization log: ${INIT_LOG}${NC}"
    echo -e "${GRAY}Key generation log: ${KEYGEN_LOG}${NC}"
    echo -e "${GRAY}Genesis file log: ${GENESIS_LOG}${NC}"
    echo -e "${GRAY}Configuration log: ${CONFIG_LOG}${NC}"
    echo -e "${GRAY}Startup log: ${START_LOG}${NC}"
    if [ -f "${ERROR_LOG}" ]; then
        echo -e "${RED}Error log: ${ERROR_LOG}${NC}"
    fi
}

# Clean old logs (keep last 10 sessions)
cleanup_old_logs() {
    if [ -d "${LOG_BASE_DIR}" ]; then
        local log_count=$(ls -1 "${LOG_BASE_DIR}" | wc -l)
        if [ "$log_count" -gt 10 ]; then
            log_info "Cleaning old log files..."
            ls -1t "${LOG_BASE_DIR}" | tail -n +11 | while read old_session; do
                rm -rf "${LOG_BASE_DIR}/${old_session}"
                log_info "Deleted old session: ${old_session}"
            done
        fi
    fi
}

# Show recent log sessions
list_recent_sessions() {
    echo -e "${CYAN}=== Recent Log Sessions ===${NC}"
    if [ -d "${LOG_BASE_DIR}" ]; then
        ls -1t "${LOG_BASE_DIR}" | head -5 | while read session; do
            local session_info="${LOG_BASE_DIR}/${session}/session_info.txt"
            if [ -f "$session_info" ]; then
                local start_time=$(grep "Start time:" "$session_info" | cut -d: -f2-)
                echo -e "${GRAY}${session}:${start_time}${NC}"
            else
                echo -e "${GRAY}${session}${NC}"
            fi
        done
    else
        echo "No log sessions available"
    fi
}

# Export variables for use by other scripts
export LOG_SESSION_DIR INIT_LOG KEYGEN_LOG GENESIS_LOG CONFIG_LOG START_LOG STOP_LOG ERROR_LOG SUMMARY_LOG
export -f init_logging start_step end_step log_info log_warn log_error log_command execute_logged execute_quiet generate_final_report
