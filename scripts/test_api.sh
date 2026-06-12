#!/bin/bash

################################################################################
# Zink Wallet API Test Script
# Purpose: Test user registration, login, and account asset viewing functionality
# Usage: ./scripts/test_api.sh [OPTIONS]
# Options:
#   --cleanup       Clean old logs before running
#   --verbose       Show detailed request/response
#   --user <name>   Test only specific user (user1 or user2)
#   --help          Show usage information
################################################################################

set -euo pipefail

################################################################################
# CONFIGURATION
################################################################################

API_BASE_URL="http://localhost:8080"
API_VERSION="v1"
SERVICE_NAME="business"
LOG_DIR="logs/test"
MARKDOWN_DOC="${LOG_DIR}/test_workflow.md"

# Test Users with hardcoded verification codes
USER1_EMAIL="test.user1@internal-wallet-test.com"
USER1_CODE="123456"
USER1_NAME="user1"

USER2_EMAIL="test.user2@internal-wallet-test.com"
# Default to the Business service bypass code (dev/test).
# If you want to test real email delivery, call send_email_code first and replace with the real code.
USER2_CODE="123456"
USER2_NAME="user2"

# Behavior Configuration
REQUEST_DELAY=1          # Seconds between requests
CURL_TIMEOUT=30          # Seconds for curl timeout
CLEANUP_LOGS=false       # Auto-cleanup old logs
VERBOSE=false            # Verbose output
SPECIFIC_USER=""         # Test only specific user

# Test Statistics
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
START_TIME=$(date +%s)

################################################################################
# COLOR DEFINITIONS
################################################################################

setup_colors() {
    if [ -t 1 ]; then
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        YELLOW='\033[1;33m'
        BLUE='\033[0;34m'
        CYAN='\033[0;36m'
        NC='\033[0m'
    else
        RED=''
        GREEN=''
        YELLOW=''
        BLUE=''
        CYAN=''
        NC=''
    fi
}

################################################################################
# LOGGING FUNCTIONS
################################################################################

log_info() {
    echo -e "${CYAN}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

log_verbose() {
    if [ "$VERBOSE" = "true" ]; then
        echo -e "${BLUE}[VERBOSE]${NC} $1" >&2
    fi
}

################################################################################
# UTILITY FUNCTIONS
################################################################################

show_usage() {
    cat <<EOF
Zink Wallet API Test Script

Usage: $0 [OPTIONS]

Options:
    --cleanup       Clean old logs before running tests
    --verbose       Show detailed request/response information
    --user <name>   Test only specific user (user1 or user2)
    --help          Show this help message

Examples:
    $0                      # Run all tests
    $0 --cleanup            # Clean logs and run tests
    $0 --verbose            # Run with detailed output
    $0 --user user1         # Test only user1
    $0 --cleanup --verbose  # Combine options

EOF
    exit 0
}

check_dependencies() {
    local missing_deps=()

    if ! command -v jq &> /dev/null; then
        missing_deps+=("jq")
    fi

    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_info "Please install missing dependencies:"
        for dep in "${missing_deps[@]}"; do
            echo "  - $dep"
        done
        exit 2
    fi

    log_verbose "All dependencies are installed (jq, curl)"
}

check_api_connectivity() {
    log_info "Checking API Gateway connectivity..."

    if ! curl -s --connect-timeout 5 --max-time 10 "${API_BASE_URL}/health" > /dev/null 2>&1; then
        log_warning "Unable to connect to API Gateway at ${API_BASE_URL}"
        log_warning "Attempting to proceed anyway..."
    else
        log_verbose "API Gateway is accessible at ${API_BASE_URL}"
    fi
}

setup_logging() {
    if [ ! -d "$LOG_DIR" ]; then
        mkdir -p "$LOG_DIR"
        log_verbose "Created log directory: $LOG_DIR"
    fi
}

cleanup_old_logs() {
    if [ "$CLEANUP_LOGS" = "true" ]; then
        log_info "Cleaning up old test logs..."

        # Remove logs older than 7 days
        find "$LOG_DIR" -name "*.log" -mtime +7 -delete 2>/dev/null || true

        # Keep only last 50 log files if too many
        local log_count=$(find "$LOG_DIR" -name "*.log" -type f 2>/dev/null | wc -l)
        if [ "$log_count" -gt 50 ]; then
            find "$LOG_DIR" -name "*.log" -type f -printf '%T@ %p\n' 2>/dev/null | \
                sort -n | head -n -50 | cut -d' ' -f2- | xargs rm -f 2>/dev/null || true
        fi

        log_success "Cleanup complete"
    fi
}

truncate_token() {
    local token="$1"
    if [ ${#token} -gt 20 ]; then
        echo "${token:0:20}...truncated"
    else
        echo "$token"
    fi
}

################################################################################
# RESPONSE VALIDATION
################################################################################

check_response() {
    local http_code="$1"
    local response="$2"
    local operation="$3"

    # Check HTTP status
    if [ "$http_code" != "200" ]; then
        log_error "$operation failed with HTTP $http_code"
        log_verbose "Response: $response"
        return 1
    fi

    # Validate JSON format
    if ! echo "$response" | jq -e '.' >/dev/null 2>&1; then
        log_error "$operation returned invalid JSON"
        log_verbose "Response: $response"
        return 1
    fi

    # Check success field
    local success=$(echo "$response" | jq -r '.success // false')
    if [ "$success" != "true" ]; then
        local error_msg=$(echo "$response" | jq -r '.message // "Unknown error"')
        log_error "$operation failed: $error_msg"
        log_verbose "Full response: $response"
        return 1
    fi

    return 0
}

################################################################################
# LOGGING SUCCESSFUL OPERATIONS
################################################################################

log_successful_operation() {
    local user_name="$1"
    local operation="$2"
    local endpoint="$3"
    local request_body="$4"
    local response="$5"
    local http_code="$6"

    # Verify this is a successful operation
    local success=$(echo "$response" | jq -r '.success // false')
    if [ "$success" != "true" ]; then
        return 1
    fi

    # Generate log file with timestamp
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local logfile="${LOG_DIR}/${user_name}_${operation}_${timestamp}.log"

    # Truncate token in response for security
    local safe_response="$response"
    if echo "$response" | jq -e '.data.access_token' >/dev/null 2>&1; then
        local token=$(echo "$response" | jq -r '.data.access_token')
        local truncated=$(truncate_token "$token")
        safe_response=$(echo "$response" | jq --arg trunc "$truncated" '.data.access_token = $trunc')
    fi

    # Write log file
    {
        echo "==========================================="
        echo "Test Run: Zink Wallet API Test"
        echo "Timestamp: $(date '+%Y-%m-%d %H:%M:%S %Z')"
        # Bash 3.2 (macOS default) does not support ${var^^}
        echo "Operation: $(echo "$operation" | tr '[:lower:]' '[:upper:]')"
        echo "User: $user_name"
        echo "Status: SUCCESS"
        echo "==========================================="
        echo ""
        echo ">>> REQUEST <<<"
        echo "Endpoint: POST $endpoint"
        echo "Headers: Content-Type: application/json"
        if echo "$request_body" | jq -e '.access_token' >/dev/null 2>&1; then
            echo "         Authorization: Bearer <token>"
        fi
        echo "Body:"
        echo "$request_body" | jq '.'
        echo ""
        echo "<<< RESPONSE <<<"
        echo "HTTP Status: $http_code"
        echo "Body:"
        echo "$safe_response" | jq '.'
        echo ""
        echo "==========================================="
        echo "Log written by: Zink Wallet Test Script v1.0"
        echo "==========================================="
    } > "$logfile"

    log_verbose "Logged successful operation to: $logfile"
    return 0
}

################################################################################
# API OPERATIONS
################################################################################

do_login() {
    local email="$1"
    local code="$2"
    local user_name="$3"

    local endpoint="${API_BASE_URL}/api/${API_VERSION}/${SERVICE_NAME}/login_by_email_code"
    local request_body=$(jq -n \
        --arg email "$email" \
        --arg code "$code" \
        '{email: $email, code: $code}')

    log_verbose "Attempting login for: $email"

    # Make request
    local full_response=$(curl -s -w "\n%{http_code}" \
        -X POST "$endpoint" \
        -H "Content-Type: application/json" \
        -d "$request_body" \
        --max-time "$CURL_TIMEOUT")

    local http_code=$(echo "$full_response" | tail -n1)
    local response=$(echo "$full_response" | sed '$d')

    # Validate response
    if ! check_response "$http_code" "$response" "Login"; then
        return 1
    fi

    # Extract token
    local token=$(echo "$response" | jq -r '.data.access_token // empty')
    if [ -z "$token" ] || [ "$token" = "null" ]; then
        log_error "Failed to extract access token from response"
        return 1
    fi

    # Extract user_id
    local user_id=$(echo "$response" | jq -r '.data.user_id // "unknown"')

    # Log successful operation
    log_successful_operation "$user_name" "login" "$endpoint" "$request_body" "$response" "$http_code"

    # Return token via stdout
    echo "$token"

    log_verbose "Login successful, user_id: $user_id"
    return 0
}

get_asset_overview() {
    local token="$1"
    local user_name="$2"

    local endpoint="${API_BASE_URL}/api/${API_VERSION}/${SERVICE_NAME}/get_asset_overview"
    local request_body='{"asset":""}'

    log_verbose "Fetching asset overview..."

    # Make request
    local full_response=$(curl -s -w "\n%{http_code}" \
        -X POST "$endpoint" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $token" \
        -d "$request_body" \
        --max-time "$CURL_TIMEOUT")

    local http_code=$(echo "$full_response" | tail -n1)
    local response=$(echo "$full_response" | sed '$d')

    # Validate response
    if ! check_response "$http_code" "$response" "Get Asset Overview"; then
        return 1
    fi

    # Log successful operation
    log_successful_operation "$user_name" "asset_overview" "$endpoint" "$request_body" "$response" "$http_code"

    log_verbose "Asset overview retrieved successfully"
    return 0
}

get_my_overview() {
    local token="$1"
    local user_name="$2"

    local endpoint="${API_BASE_URL}/api/${API_VERSION}/${SERVICE_NAME}/get_my_overview"
    local request_body='{}'

    log_verbose "Fetching my overview..."

    # Make request
    local full_response=$(curl -s -w "\n%{http_code}" \
        -X POST "$endpoint" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $token" \
        -d "$request_body" \
        --max-time "$CURL_TIMEOUT")

    local http_code=$(echo "$full_response" | tail -n1)
    local response=$(echo "$full_response" | sed '$d')

    # Validate response
    if ! check_response "$http_code" "$response" "Get My Overview"; then
        return 1
    fi

    # Log successful operation
    log_successful_operation "$user_name" "my_overview" "$endpoint" "$request_body" "$response" "$http_code"

    log_verbose "My overview retrieved successfully"
    return 0
}

################################################################################
# USER WORKFLOW
################################################################################

test_user_workflow() {
    local user_name="$1"
    local user_email="$2"
    local user_code="$3"

    echo ""
    log_info "Testing User: ${user_email}"

    local user_token=""
    local user_id=""

    # Step 1: Login
    printf "  Login... "
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if user_token=$(do_login "$user_email" "$user_code" "$user_name"); then
        user_id=$(echo "$user_token" | jq -r '.user_id // "unknown"' 2>/dev/null || echo "unknown")
        echo -e "${GREEN}✓${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        log_error "Login failed for $user_email, skipping remaining tests for this user"
        return 1
    fi

    # Rate limiting delay
    sleep "$REQUEST_DELAY"

    # Step 2: Get Asset Overview
    printf "  Asset Overview... "
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if get_asset_overview "$user_token" "$user_name"; then
        echo -e "${GREEN}✓${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi

    # Rate limiting delay
    sleep "$REQUEST_DELAY"

    # Step 3: Get My Overview
    printf "  My Overview... "
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if get_my_overview "$user_token" "$user_name"; then
        echo -e "${GREEN}✓${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi

    # Clear token from memory
    user_token=""

    return 0
}

################################################################################
# SUMMARY REPORT
################################################################################

print_summary() {
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))

    echo ""
    echo "========================================="
    echo "          TEST SUMMARY REPORT           "
    echo "========================================="
    echo "Total Users Tested:    $((TOTAL_TESTS / 3))"
    echo "Total Operations:      $TOTAL_TESTS"

    if [ "$PASSED_TESTS" -gt 0 ]; then
        echo -e "Passed:                ${GREEN}${PASSED_TESTS}${NC}"
    else
        echo "Passed:                $PASSED_TESTS"
    fi

    if [ "$FAILED_TESTS" -gt 0 ]; then
        echo -e "Failed:                ${RED}${FAILED_TESTS}${NC}"
    else
        echo "Failed:                $FAILED_TESTS"
    fi

    echo "Duration:              ${duration}s"
    echo "Log Directory:         $LOG_DIR"
    echo "========================================="

    if [ "$FAILED_TESTS" -eq 0 ]; then
        echo -e "${GREEN}✓ All tests passed!${NC}"
        echo ""
        log_success "Logs saved to $LOG_DIR/"
    else
        echo -e "${YELLOW}⚠ Some tests failed. Check output above.${NC}"
        echo ""
    fi
}

################################################################################
# MAIN EXECUTION
################################################################################

main() {
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --cleanup)
                CLEANUP_LOGS=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --user)
                SPECIFIC_USER="$2"
                shift 2
                ;;
            --help)
                show_usage
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                ;;
        esac
    done

    # Setup
    setup_colors

    # Header
    echo ""
    echo "========================================="
    echo "   Zink Wallet API Test Script"
    echo "========================================="
    echo ""

    # Pre-flight checks
    log_info "Running pre-flight checks..."
    check_dependencies
    check_api_connectivity
    setup_logging
    cleanup_old_logs

    log_success "Pre-flight checks complete"

    # Run tests
    log_info "Starting Zink Wallet API Tests..."

    if [ -z "$SPECIFIC_USER" ] || [ "$SPECIFIC_USER" = "user1" ]; then
        test_user_workflow "$USER1_NAME" "$USER1_EMAIL" "$USER1_CODE"
    fi

    if [ -z "$SPECIFIC_USER" ] || [ "$SPECIFIC_USER" = "user2" ]; then
        test_user_workflow "$USER2_NAME" "$USER2_EMAIL" "$USER2_CODE"
    fi

    # Print summary
    print_summary

    # Exit with appropriate code
    if [ "$FAILED_TESTS" -eq 0 ]; then
        exit 0
    else
        exit 1
    fi
}

# Run main function
main "$@"
