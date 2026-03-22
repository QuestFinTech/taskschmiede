#!/usr/bin/env bash
#
# Taskschmiede post-install smoke test
#
# Usage:
#   scripts/smoke-test.sh [--host HOST] [--https] [--ssh SSH_HOST] [--verbose]
#
# Defaults: --host localhost, HTTP, no SSH (run locally).

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
HOST="localhost"
PROTOCOL="http"
SSH_HOST=""
VERBOSE=false
CURL_OPTS="--max-time 5 --silent"
DATA_DIR="/opt/taskschmiede/data"
ENV_FILE="/opt/taskschmiede/config/.env"

PASS_COUNT=0
FAIL_COUNT=0
INFO_COUNT=0
SKIP_COUNT=0

# ---------------------------------------------------------------------------
# Color support
# ---------------------------------------------------------------------------
if [ -t 1 ]; then
    C_GREEN="\033[0;32m"
    C_RED="\033[0;31m"
    C_YELLOW="\033[0;33m"
    C_RESET="\033[0m"
else
    C_GREEN=""
    C_RED=""
    C_YELLOW=""
    C_RESET=""
fi

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
    case "$1" in
        --host)
            HOST="$2"
            shift 2
            ;;
        --https)
            PROTOCOL="https"
            shift
            ;;
        --ssh)
            SSH_HOST="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--host HOST] [--https] [--ssh SSH_HOST] [--verbose]"
            echo ""
            echo "Options:"
            echo "  --host HOST       Target host (default: localhost)"
            echo "  --https           Use HTTPS instead of HTTP"
            echo "  --ssh SSH_HOST    Run server-side checks via SSH"
            echo "  --verbose         Show curl output and debug info"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Add --insecure for HTTPS (self-signed certs are common)
if [ "$PROTOCOL" = "https" ]; then
    CURL_OPTS="$CURL_OPTS --insecure"
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log_verbose() {
    if [ "$VERBOSE" = true ]; then
        echo "  [verbose] $*"
    fi
}

result_pass() {
    local label="$1"
    local detail="${2:-}"
    if [ -n "$detail" ]; then
        printf "  %-24s ${C_GREEN}PASS${C_RESET}  (%s)\n" "$label" "$detail"
    else
        printf "  %-24s ${C_GREEN}PASS${C_RESET}\n" "$label"
    fi
    PASS_COUNT=$((PASS_COUNT + 1))
}

result_fail() {
    local label="$1"
    local detail="${2:-}"
    if [ -n "$detail" ]; then
        printf "  %-24s ${C_RED}FAIL${C_RESET}  (%s)\n" "$label" "$detail"
    else
        printf "  %-24s ${C_RED}FAIL${C_RESET}\n" "$label"
    fi
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

result_info() {
    local label="$1"
    local detail="${2:-}"
    if [ -n "$detail" ]; then
        printf "  %-24s ${C_YELLOW}INFO${C_RESET}  (%s)\n" "$label" "$detail"
    else
        printf "  %-24s ${C_YELLOW}INFO${C_RESET}\n" "$label"
    fi
    INFO_COUNT=$((INFO_COUNT + 1))
}

result_skip() {
    local label="$1"
    local detail="${2:-}"
    if [ -n "$detail" ]; then
        printf "  %-24s SKIP  (%s)\n" "$label" "$detail"
    else
        printf "  %-24s SKIP\n" "$label"
    fi
    SKIP_COUNT=$((SKIP_COUNT + 1))
}

# Run a command locally or via SSH
remote_cmd() {
    if [ -n "$SSH_HOST" ]; then
        ssh -o ConnectTimeout=5 -o BatchMode=yes "$SSH_HOST" "$@" 2>/dev/null
    else
        eval "$@" 2>/dev/null
    fi
}

base_url() {
    local port="$1"
    # When running via SSH, curl runs on the remote host against localhost.
    local target="$HOST"
    if [ -n "$SSH_HOST" ]; then
        target="localhost"
    fi
    echo "http://${target}:${port}"
}

# Run curl locally or via SSH. When --ssh is given, curl runs on the
# remote host against localhost so it can reach ports that are not
# exposed to the network.
remote_curl() {
    if [ -n "$SSH_HOST" ]; then
        ssh -o ConnectTimeout=5 -o BatchMode=yes "$SSH_HOST" "curl $CURL_OPTS $*" 2>/dev/null
    else
        curl $CURL_OPTS "$@" 2>/dev/null
    fi
}

# ---------------------------------------------------------------------------
# Test sections
# ---------------------------------------------------------------------------

test_service_health() {
    echo ""
    echo "Service Health"

    # Core server (9000)
    local url
    url="$(base_url 9000)/mcp/health"
    log_verbose "GET $url"
    local body
    if body=$(remote_curl "$url" 2>&1); then
        log_verbose "Response: $body"
        local status version
        status=$(echo "$body" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
        version=$(echo "$body" | grep -o '"version":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ "$status" = "healthy" ]; then
            result_pass "Core server (9000)" "v${version}, healthy"
        else
            result_fail "Core server (9000)" "status: ${status:-unknown}"
        fi
    else
        result_fail "Core server (9000)" "connection failed"
    fi

    # Portal (9090)
    url="$(base_url 9090)/"
    log_verbose "GET $url"
    local http_code
    if http_code=$(remote_curl -o /dev/null -w "%{http_code}" "$url" 2>&1); then
        log_verbose "HTTP $http_code"
        case "$http_code" in
            200|301|302|303)
                result_pass "Portal (9090)" "HTTP $http_code"
                ;;
            000)
                result_fail "Portal (9090)" "connection failed"
                ;;
            *)
                result_fail "Portal (9090)" "HTTP $http_code"
                ;;
        esac
    else
        result_fail "Portal (9090)" "connection failed"
    fi

    # Proxy (9001)
    url="$(base_url 9001)/proxy/health"
    log_verbose "GET $url"
    if body=$(remote_curl "$url" 2>&1); then
        log_verbose "Response: $body"
        local proxy_status
        proxy_status=$(echo "$body" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
        local upstream_info=""
        if echo "$body" | grep -q '"mcp_upstream"'; then
            local mcp_up
            mcp_up=$(echo "$body" | grep -o '"mcp_upstream":"[^"]*"' | head -1 | cut -d'"' -f4)
            upstream_info="upstream $mcp_up"
        fi
        if [ "$proxy_status" = "healthy" ]; then
            result_pass "Proxy (9001)" "${upstream_info:-healthy}"
        else
            result_fail "Proxy (9001)" "status: ${proxy_status:-unknown}"
        fi
    else
        result_fail "Proxy (9001)" "connection failed"
    fi
}

test_setup_state() {
    echo ""
    echo "Setup"

    local url
    url="$(base_url 9000)/api/v1/admin/setup/status"
    log_verbose "GET $url"
    local body
    if body=$(remote_curl "$url" 2>&1); then
        log_verbose "Response: $body"
        local phase
        phase=$(echo "$body" | grep -o '"phase":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$phase" ]; then
            result_info "Phase" "$phase"
        else
            result_fail "Phase" "could not parse response"
        fi
    else
        result_fail "Phase" "connection failed"
    fi
}

test_database() {
    echo ""
    echo "Database"

    # Main DB
    local size
    if size=$(remote_cmd "stat -c '%s' '${DATA_DIR}/taskschmiede.db' 2>/dev/null || stat -f '%z' '${DATA_DIR}/taskschmiede.db' 2>/dev/null"); then
        if [ "$size" -gt 0 ] 2>/dev/null; then
            result_pass "Main DB" "$size bytes"
        else
            result_fail "Main DB" "empty file"
        fi
    else
        result_fail "Main DB" "file not found"
    fi

    # Message DB
    if size=$(remote_cmd "stat -c '%s' '${DATA_DIR}/taskschmiede_messages.db' 2>/dev/null || stat -f '%z' '${DATA_DIR}/taskschmiede_messages.db' 2>/dev/null"); then
        if [ "$size" -gt 0 ] 2>/dev/null; then
            result_pass "Message DB" "$size bytes"
        else
            result_fail "Message DB" "empty file"
        fi
    else
        result_fail "Message DB" "file not found"
    fi
}

test_email_config() {
    echo ""
    echo "Email"

    local addr
    if addr=$(remote_cmd "grep -E '^EMAIL_SUPPORT_ADDRESS=' '${ENV_FILE}'" 2>/dev/null); then
        addr=$(echo "$addr" | cut -d'=' -f2 | tr -d '"' | tr -d "'")
        if [ -n "$addr" ]; then
            result_info "SMTP configured" "$addr"
        else
            result_info "SMTP configured" "variable set but empty"
        fi
    else
        result_info "SMTP configured" "not configured"
    fi
}

test_systemd_services() {
    echo ""
    echo "Systemd Services"

    # Check if systemd is available
    if ! remote_cmd "command -v systemctl" >/dev/null 2>&1; then
        result_skip "systemd" "not available"
        return
    fi

    local services=("taskschmiede" "taskschmiede-proxy" "taskschmiede-portal")
    for svc in "${services[@]}"; do
        local active enabled
        active=$(remote_cmd "systemctl is-active '$svc'" 2>/dev/null || echo "unknown")
        enabled=$(remote_cmd "systemctl is-enabled '$svc'" 2>/dev/null || echo "unknown")

        case "$active" in
            active)
                result_pass "$svc" "active, $enabled"
                ;;
            inactive|failed|unknown)
                result_fail "$svc" "$active, $enabled"
                ;;
            *)
                result_fail "$svc" "$active"
                ;;
        esac
    done
}

test_port_connectivity() {
    echo ""
    echo "Port Connectivity"

    local ports=(9000 9001 9090)
    for port in "${ports[@]}"; do
        log_verbose "Testing TCP connect to port ${port}"
        if remote_curl --max-time 3 -o /dev/null "http://localhost:${port}/" 2>/dev/null; then
            result_pass "${port}/tcp"
        else
            result_fail "${port}/tcp"
        fi
    done
}

test_proxy_features() {
    echo ""
    echo "Proxy"

    local url
    url="$(base_url 9001)/proxy/health"
    log_verbose "GET $url"
    local body
    if body=$(remote_curl "$url" 2>&1); then
        log_verbose "Response: $body"

        # MCP upstream
        local mcp_up
        mcp_up=$(echo "$body" | grep -o '"mcp_upstream":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$mcp_up" ]; then
            if [ "$mcp_up" = "connected" ]; then
                result_pass "Upstream MCP" "connected"
            else
                result_fail "Upstream MCP" "$mcp_up"
            fi
        else
            result_skip "Upstream MCP" "not reported"
        fi

        # REST upstream
        local rest_up
        rest_up=$(echo "$body" | grep -o '"rest_upstream":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$rest_up" ]; then
            if [ "$rest_up" = "connected" ]; then
                result_pass "Upstream REST" "connected"
            else
                result_fail "Upstream REST" "$rest_up"
            fi
        else
            result_skip "Upstream REST" "not reported"
        fi
    else
        result_fail "Upstream MCP" "proxy unreachable"
        result_fail "Upstream REST" "proxy unreachable"
    fi
}

test_tls_certificate() {
    echo ""
    echo "TLS Certificate"

    if [ "$PROTOCOL" != "https" ]; then
        result_skip "Certificate" "not using HTTPS"
        return
    fi

    local expiry
    expiry=$(echo | openssl s_client -servername "$HOST" -connect "${HOST}:9090" 2>/dev/null \
        | openssl x509 -noout -enddate 2>/dev/null \
        | cut -d'=' -f2)

    if [ -n "$expiry" ]; then
        # Check if cert is still valid
        local expiry_epoch now_epoch
        if date --version >/dev/null 2>&1; then
            # GNU date
            expiry_epoch=$(date -d "$expiry" +%s 2>/dev/null || echo 0)
            now_epoch=$(date +%s)
        else
            # BSD date (macOS)
            expiry_epoch=$(date -jf "%b %d %T %Y %Z" "$expiry" +%s 2>/dev/null || echo 0)
            now_epoch=$(date +%s)
        fi

        if [ "$expiry_epoch" -gt "$now_epoch" ] 2>/dev/null; then
            result_pass "Certificate" "expires $expiry"
        else
            result_fail "Certificate" "expired $expiry"
        fi
    else
        result_fail "Certificate" "could not read certificate"
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

echo ""
echo "Taskschmiede Smoke Test"
echo "========================"
echo "Host: $HOST"
echo "Protocol: $PROTOCOL"
if [ -n "$SSH_HOST" ]; then
    echo "SSH: $SSH_HOST"
fi

test_service_health
test_setup_state
test_database
test_email_config
test_systemd_services
test_port_connectivity
test_proxy_features
test_tls_certificate

echo ""
echo "========================"
printf "Results: ${C_GREEN}%d passed${C_RESET}, ${C_RED}%d failed${C_RESET}, ${C_YELLOW}%d info${C_RESET}" \
    "$PASS_COUNT" "$FAIL_COUNT" "$INFO_COUNT"
if [ "$SKIP_COUNT" -gt 0 ]; then
    printf ", %d skipped" "$SKIP_COUNT"
fi
echo ""
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
exit 0
