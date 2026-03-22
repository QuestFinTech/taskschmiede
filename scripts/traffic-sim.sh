#!/usr/bin/env bash
#
# traffic-sim.sh -- Simple HTTP traffic simulator for testing rate limiting.
#
# Sends N requests to a URL and reports status code distribution.
# Designed for testing Taskschmiede's security middleware (Phase 1).
#
# Usage:
#   scripts/traffic-sim.sh [OPTIONS]
#
# Options:
#   --url URL          Target URL (required)
#   --count N          Number of requests (default: 130)
#   --concurrency N    Max parallel requests (default: 1)
#   --method METHOD    HTTP method: GET or POST (default: GET)
#   --data DATA        POST data (e.g., "email=x&password=y")
#   --delay MS         Delay between batches in ms (default: 0)
#   --quiet            Only show summary, not per-request output
#   --help             Show this help
#
# Examples:
#   # Test IP rate limit (120 req/min) on MCP health
#   scripts/traffic-sim.sh --url http://localhost:9000/mcp/health --count 130
#
#   # Test auth rate limit (5 req/min) on login
#   scripts/traffic-sim.sh --url http://localhost:9090/login --count 8 --method POST --data "email=x&password=y"
#
#   # Parallel burst against proxy
#   scripts/traffic-sim.sh --url http://localhost:9001/proxy/health --count 130 --concurrency 10
#

set -euo pipefail

# Defaults
URL=""
COUNT=130
CONCURRENCY=1
METHOD="GET"
DATA=""
DELAY=0
QUIET=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --url)       URL="$2"; shift 2 ;;
        --count)     COUNT="$2"; shift 2 ;;
        --concurrency) CONCURRENCY="$2"; shift 2 ;;
        --method)    METHOD="$2"; shift 2 ;;
        --data)      DATA="$2"; shift 2 ;;
        --delay)     DELAY="$2"; shift 2 ;;
        --quiet)     QUIET=true; shift ;;
        --help|-h)
            head -35 "$0" | tail -30
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

if [[ -z "$URL" ]]; then
    echo "Error: --url is required" >&2
    echo "Usage: scripts/traffic-sim.sh --url <URL> [--count N] [--concurrency N]" >&2
    exit 1
fi

# Build curl command
CURL_OPTS=(-s -o /dev/null -w "%{http_code}")
if [[ "$METHOD" == "POST" ]]; then
    CURL_OPTS+=(-X POST)
    if [[ -n "$DATA" ]]; then
        CURL_OPTS+=(-d "$DATA")
    fi
fi

echo "Traffic Simulator"
echo "  URL:         $URL"
echo "  Count:       $COUNT"
echo "  Concurrency: $CONCURRENCY"
echo "  Method:      $METHOD"
[[ -n "$DATA" ]] && echo "  Data:        $DATA"
echo ""

# Track results
declare -A STATUS_COUNTS
RESULTS_FILE=$(mktemp)
trap 'rm -f "$RESULTS_FILE"' EXIT

# Send requests
send_request() {
    local i=$1
    local code
    code=$(curl "${CURL_OPTS[@]}" "$URL" 2>/dev/null || echo "000")
    echo "$code" >> "$RESULTS_FILE"
    if [[ "$QUIET" == "false" ]]; then
        printf "  [%3d] HTTP %s\n" "$i" "$code"
    fi
}

START_TIME=$(date +%s%N 2>/dev/null || date +%s)

if [[ "$CONCURRENCY" -le 1 ]]; then
    # Sequential
    for i in $(seq 1 "$COUNT"); do
        send_request "$i"
        if [[ "$DELAY" -gt 0 ]]; then
            sleep "$(echo "scale=3; $DELAY / 1000" | bc)"
        fi
    done
else
    # Parallel (batch)
    i=0
    while [[ $i -lt $COUNT ]]; do
        batch_end=$((i + CONCURRENCY))
        if [[ $batch_end -gt $COUNT ]]; then
            batch_end=$COUNT
        fi

        pids=()
        for j in $(seq $((i + 1)) "$batch_end"); do
            send_request "$j" &
            pids+=($!)
        done

        for pid in "${pids[@]}"; do
            wait "$pid" 2>/dev/null || true
        done

        i=$batch_end

        if [[ "$DELAY" -gt 0 && $i -lt $COUNT ]]; then
            sleep "$(echo "scale=3; $DELAY / 1000" | bc)"
        fi
    done
fi

END_TIME=$(date +%s%N 2>/dev/null || date +%s)

# Calculate duration
if [[ ${#START_TIME} -gt 10 ]]; then
    DURATION_MS=$(( (END_TIME - START_TIME) / 1000000 ))
else
    DURATION_MS=$(( (END_TIME - START_TIME) * 1000 ))
fi

# Summarize results
echo ""
echo "--- Results ---"

TOTAL=0
while IFS= read -r code; do
    STATUS_COUNTS[$code]=$(( ${STATUS_COUNTS[$code]:-0} + 1 ))
    TOTAL=$((TOTAL + 1))
done < "$RESULTS_FILE"

for code in $(echo "${!STATUS_COUNTS[@]}" | tr ' ' '\n' | sort); do
    count=${STATUS_COUNTS[$code]}
    printf "  HTTP %s: %d requests (%d%%)\n" "$code" "$count" $((count * 100 / TOTAL))
done

echo ""
echo "  Total:    $TOTAL requests"
echo "  Duration: ${DURATION_MS}ms"
if [[ $DURATION_MS -gt 0 ]]; then
    RPS=$((TOTAL * 1000 / DURATION_MS))
    echo "  Rate:     ~${RPS} req/s"
fi

# Exit code: 0 if any 429s were seen (rate limiting works), 1 if none
if [[ -n "${STATUS_COUNTS[429]:-}" ]]; then
    echo ""
    echo "Rate limiting: ACTIVE (429 responses observed)"
    exit 0
elif [[ $TOTAL -gt 120 ]]; then
    echo ""
    echo "Rate limiting: NOT OBSERVED (sent $TOTAL requests, no 429)"
    echo "  This may indicate rate limiting is disabled or misconfigured."
    exit 1
fi
