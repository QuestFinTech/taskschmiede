#!/usr/bin/env bash
# install-taskschmiede.sh -- Install or upgrade Taskschmiede on this machine.
#
# Runs as the deploy user (with sudo). Invoked by scripts/deploy.sh after upload,
# or manually from an extracted package directory.
#
# Steps:
#   1. Pre-flight checks (binaries, systemd units, target directories)
#   2. Activate maintenance mode (proxy returns 503 to clients during upgrade)
#   3. Stop services (reverse dependency order, proxy last for max maintenance coverage)
#   4. Backup current binaries to bin/previous/<version>/
#   5. Install new binaries to /opt/taskschmiede/bin/
#   6. Install/update systemd unit files (diff check, daemon-reload)
#   7. Config setup (first install only -- never overwrites)
#   8. Start services (dependency order: app, proxy, portal)
#   9. Health checks (exit non-zero on failure)

set -euo pipefail

# SCRIPT_DIR is where the install script (and binaries) live.
# Defaults to the script's own directory. Override with --package-dir
# when the script is copied to a fixed path for sudoers scoping.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
for arg in "$@"; do
    case "$arg" in
        --package-dir=*) SCRIPT_DIR="${arg#--package-dir=}" ;;
    esac
done

INSTALL_DIR="/opt/taskschmiede"
BIN_DIR="$INSTALL_DIR/bin"
CONFIG_DIR="$INSTALL_DIR/config"
DATA_DIR="$INSTALL_DIR/data"
LOG_DIR="/var/log/taskschmiede"
SYSTEMD_DIR="/etc/systemd/system"

# Core binaries (community edition). Must be present in every package.
CORE_BINARIES=(taskschmiede taskschmiede-proxy taskschmiede-portal)

# Optional binaries (SaaS edition). Installed if present in package.
OPTIONAL_BINARIES=(taskschmiede-notify taskschmiede-support)

# All possible systemd services (includes support-proxy which reuses the proxy binary).
# Services whose binaries are absent from the package are skipped automatically.
ALL_SERVICES=(taskschmiede taskschmiede-proxy taskschmiede-portal taskschmiede-notify taskschmiede-support taskschmiede-support-proxy)

# How many previous versions to keep
KEEP_VERSIONS=3

# Read version from package
VERSION=""
if [ -f "$SCRIPT_DIR/VERSION" ]; then
    VERSION="$(cat "$SCRIPT_DIR/VERSION")"
fi

die() {
    echo "Error: $*" >&2
    exit 1
}

echo ""
echo "============================================================"
echo "  Taskschmiede Install ${VERSION:-unknown}"
echo "============================================================"
echo ""

# ---------------------------------------------------------------
# 1. Pre-flight checks
# ---------------------------------------------------------------
echo "Pre-flight checks"
echo "------------------------------------------------------------"

for bin in "${CORE_BINARIES[@]}"; do
    if [ ! -f "$SCRIPT_DIR/$bin" ]; then
        die "Core binary not found in package: $bin"
    fi
done
echo "  Core binaries: ${CORE_BINARIES[*]}"

FOUND_OPTIONAL=()
for bin in "${OPTIONAL_BINARIES[@]}"; do
    if [ -f "$SCRIPT_DIR/$bin" ]; then
        FOUND_OPTIONAL+=("$bin")
    fi
done
if [ ${#FOUND_OPTIONAL[@]} -gt 0 ]; then
    echo "  Optional binaries: ${FOUND_OPTIONAL[*]}"
fi

# Combined list of all binaries to install
ALL_BINARIES=("${CORE_BINARIES[@]}" "${FOUND_OPTIONAL[@]}")

# Derive active services from installed binaries.
# taskschmiede-support-proxy uses the proxy binary but is only relevant
# when the support agent is present.
SERVICES=()
for svc in "${ALL_SERVICES[@]}"; do
    case "$svc" in
        taskschmiede-support-proxy)
            # Only if support agent is in the package
            for opt in "${FOUND_OPTIONAL[@]}"; do
                if [ "$opt" = "taskschmiede-support" ]; then
                    SERVICES+=("$svc")
                    break
                fi
            done
            ;;
        *)
            # Include if the matching binary exists
            for bin in "${ALL_BINARIES[@]}"; do
                if [ "$bin" = "$svc" ]; then
                    SERVICES+=("$svc")
                    break
                fi
            done
            ;;
    esac
done
echo "  Active services: ${SERVICES[*]}"

for dir in "$BIN_DIR" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"; do
    if [ ! -d "$dir" ]; then
        die "Directory does not exist: $dir (run server setup first)"
    fi
done
echo "  Target directories verified"

for svc in "${SERVICES[@]}"; do
    if [ ! -f "$SCRIPT_DIR/systemd/$svc.service" ]; then
        echo "  WARNING: systemd/$svc.service not in package (service will be skipped)"
    fi
done
echo "  Systemd units checked"

echo ""

# ---------------------------------------------------------------
# 2. Activate maintenance mode (if proxy is running)
# ---------------------------------------------------------------
echo "Activating maintenance mode"
echo "------------------------------------------------------------"

MGMT_PORT=9010
MGMT_KEY=""
if [ -f "$CONFIG_DIR/.env" ]; then
    MGMT_KEY=$(grep '^PROXY_MANAGEMENT_KEY=' "$CONFIG_DIR/.env" 2>/dev/null | cut -d= -f2- || true)
fi

MAINT_ACTIVATED=false
if [ -n "$MGMT_KEY" ] && curl -s --max-time 2 "http://127.0.0.1:$MGMT_PORT/proxy/health" >/dev/null 2>&1; then
    MAINT_RESP=$(curl -s --max-time 5 -X POST "http://127.0.0.1:$MGMT_PORT/proxy/maintenance" \
        -H "Authorization: Bearer $MGMT_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"enabled\":true,\"reason\":\"Upgrading to ${VERSION:-unknown}\"}" 2>/dev/null || echo "")
    if echo "$MAINT_RESP" | grep -q '"success"'; then
        echo "  Maintenance mode activated (clients see 503 with reason)"
        MAINT_ACTIVATED=true
    else
        echo "  Could not activate maintenance mode (continuing anyway)"
    fi
else
    echo "  Proxy management API not reachable (first install or proxy not running)"
fi

echo ""

# ---------------------------------------------------------------
# 3. Stop services (reverse dependency order)
# ---------------------------------------------------------------
echo "Stopping services"
echo "------------------------------------------------------------"
# Stop in reverse dependency order (proxy last for max maintenance coverage).
# Build reverse list from SERVICES.
STOP_ORDER=()
for ((i=${#SERVICES[@]}-1; i>=0; i--)); do
    STOP_ORDER+=("${SERVICES[$i]}")
done
# Always stop proxy last if it's in the list
for svc in "${STOP_ORDER[@]}"; do
    if systemctl is-active --quiet "$svc" 2>/dev/null; then
        echo "  Stopping $svc..."
        sudo systemctl stop "$svc"
        echo "  Stopped $svc"
    else
        echo "  $svc not running (skip)"
    fi
done

echo ""

# ---------------------------------------------------------------
# 4. Backup current binaries
# ---------------------------------------------------------------
echo "Backing up current binaries"
echo "------------------------------------------------------------"

# Detect currently installed version (if any)
CURRENT_VERSION=""
if [ -f "$BIN_DIR/taskschmiede" ]; then
    CURRENT_VERSION=$("$BIN_DIR/taskschmiede" version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+[^ ]*' | head -1 || echo "")
fi

if [ -n "$CURRENT_VERSION" ]; then
    BACKUP_DIR="$BIN_DIR/previous/$CURRENT_VERSION"
    sudo -u taskschmiede mkdir -p "$BACKUP_DIR"

    for bin in "${ALL_BINARIES[@]}"; do
        if [ -f "$BIN_DIR/$bin" ]; then
            sudo -u taskschmiede cp "$BIN_DIR/$bin" "$BACKUP_DIR/$bin"
        fi
    done
    echo "  Backed up to previous/$CURRENT_VERSION/"

    # Prune old versions (keep last N)
    PREV_DIR="$BIN_DIR/previous"
    VERSION_COUNT=$(find "$PREV_DIR" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
    if [ "$VERSION_COUNT" -gt "$KEEP_VERSIONS" ]; then
        # Sort by directory name (version), remove oldest
        REMOVE_COUNT=$((VERSION_COUNT - KEEP_VERSIONS))
        find "$PREV_DIR" -mindepth 1 -maxdepth 1 -type d | sort | head -n "$REMOVE_COUNT" | while read -r old_dir; do
            sudo rm -rf "$old_dir"
            echo "  Pruned old version: $(basename "$old_dir")"
        done
    fi
else
    echo "  No existing installation to back up (first install)"
    sudo -u taskschmiede mkdir -p "$BIN_DIR/previous"
fi

echo ""

# ---------------------------------------------------------------
# 5. Install new binaries
# ---------------------------------------------------------------
echo "Installing binaries"
echo "------------------------------------------------------------"
for bin in "${ALL_BINARIES[@]}"; do
    sudo cp "$SCRIPT_DIR/$bin" "$BIN_DIR/$bin"
    sudo chown taskschmiede:taskschmiede "$BIN_DIR/$bin"
    sudo chmod 755 "$BIN_DIR/$bin"
    echo "  Installed $bin"
done

INSTALLED_VERSION=$("$BIN_DIR/taskschmiede" version 2>/dev/null | head -1 || echo "unknown")
echo "  Version: $INSTALLED_VERSION"

echo ""

# ---------------------------------------------------------------
# 6. Install systemd units
# ---------------------------------------------------------------
echo "Installing systemd units"
echo "------------------------------------------------------------"
UNITS_CHANGED=0
for svc in "${ALL_SERVICES[@]}"; do
    SRC="$SCRIPT_DIR/systemd/$svc.service"
    DST="$SYSTEMD_DIR/$svc.service"
    if [ ! -f "$SRC" ]; then
        continue
    fi
    if [ -f "$DST" ] && diff -q "$SRC" "$DST" >/dev/null 2>&1; then
        echo "  $svc.service unchanged (skip)"
    else
        sudo cp "$SRC" "$DST"
        sudo chmod 644 "$DST"
        echo "  Installed $svc.service"
        UNITS_CHANGED=1
    fi
done

if [ "$UNITS_CHANGED" -eq 1 ]; then
    sudo systemctl daemon-reload
    echo "  systemd daemon reloaded"
fi

echo ""

# ---------------------------------------------------------------
# 7. Config (first install only)
# ---------------------------------------------------------------
echo "Configuration"
echo "------------------------------------------------------------"

# Config dir is owned by taskschmiede (750), so file checks need sudo -u.
if sudo -u taskschmiede test -f "$CONFIG_DIR/config.yaml"; then
    echo "  config.yaml exists (not overwriting)"
else
    if [ -f "$SCRIPT_DIR/config.yaml.example" ]; then
        sudo -u taskschmiede cp "$SCRIPT_DIR/config.yaml.example" "$CONFIG_DIR/config.yaml.example"
        echo "  Installed config.yaml.example"
        echo ""
        echo "  NOTICE: First install detected."
        echo "  Create config.yaml and .env in $CONFIG_DIR/ before starting."
        echo "  See config.yaml.example for reference."
        echo ""
        echo "  Skipping service start (no config)."
        echo "============================================================"
        exit 0
    fi
fi

if ! sudo -u taskschmiede test -f "$CONFIG_DIR/.env"; then
    echo "  WARNING: $CONFIG_DIR/.env does not exist. Services may fail to start."
fi

# Always update examples for reference
for example in config.yaml.example support-agent.yaml.example support-proxy.yaml.example; do
    if [ -f "$SCRIPT_DIR/$example" ]; then
        sudo -u taskschmiede cp "$SCRIPT_DIR/$example" "$CONFIG_DIR/$example"
    fi
done

echo ""

# ---------------------------------------------------------------
# 8. Enable and start services (dependency order)
# ---------------------------------------------------------------
echo "Starting services"
echo "------------------------------------------------------------"
for svc in "${SERVICES[@]}"; do
    sudo systemctl enable "$svc" 2>/dev/null || true
    sudo systemctl start "$svc"
    echo "  Started $svc"
    sleep 2
done

echo ""

# ---------------------------------------------------------------
# 9. Health checks
# ---------------------------------------------------------------
echo "Health checks"
echo "------------------------------------------------------------"
HEALTH_OK=true

# Health check table: service -> port, path, match pattern
# Only checks services that were actually started.
check_health() {
    local label="$1" url="$2" pattern="$3"
    local resp
    resp=$(curl -s --max-time 5 "$url" 2>/dev/null || echo "")
    if echo "$resp" | grep -q "$pattern"; then
        echo "  $label  OK"
    else
        echo "  $label  FAILED"
        HEALTH_OK=false
    fi
}

check_health_http() {
    local label="$1" url="$2"
    local status
    status=$(curl -s --max-time 5 -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    if echo "$status" | grep -qE '^[23]'; then
        echo "  $label  OK"
    else
        echo "  $label  FAILED (HTTP $status)"
        HEALTH_OK=false
    fi
}

for svc in "${SERVICES[@]}"; do
    case "$svc" in
        taskschmiede)
            check_health "App server (:9000):" "http://localhost:9000/mcp/health" '"status"' ;;
        taskschmiede-proxy)
            check_health "MCP Proxy (:9001):" "http://localhost:9001/proxy/health" '"status"' ;;
        taskschmiede-portal)
            check_health_http "Portal (:9090):" "http://localhost:9090/" ;;
        taskschmiede-notify)
            check_health "Notify (:9004):" "http://localhost:9004/notify/health" '"status"' ;;
        taskschmiede-support)
            check_health "Support Agent (:9002):" "http://localhost:9002/mcp/health" '"status"' ;;
        taskschmiede-support-proxy)
            check_health "Support Proxy (:9003):" "http://localhost:9003/proxy/health" '"status"' ;;
    esac
done

# Proxy management API (check if proxy has maintenance mode enabled)
for svc in "${SERVICES[@]}"; do
    if [ "$svc" = "taskschmiede-proxy" ]; then
        MGMT_HEALTH=$(curl -s --max-time 5 http://127.0.0.1:9010/proxy/health 2>/dev/null || echo "")
        if echo "$MGMT_HEALTH" | grep -q '"upstream_state"'; then
            UPSTREAM_STATE=$(echo "$MGMT_HEALTH" | grep -o '"upstream_state":"[^"]*"' | cut -d'"' -f4)
            echo "  Proxy Mgmt (:9010):  OK (upstream: $UPSTREAM_STATE)"
        else
            echo "  Proxy Mgmt (:9010):  not configured (maintenance mode disabled)"
        fi
        break
    fi
done

echo ""

if [ "$HEALTH_OK" = true ]; then
    echo "============================================================"
    echo "  Deployment successful: ${VERSION:-unknown}"
    echo "============================================================"
else
    echo "============================================================"
    echo "  WARNING: One or more health checks failed."
    echo ""
    echo "  Diagnose:"
    for svc in "${SERVICES[@]}"; do
        echo "    journalctl -u $svc --no-pager -n 50"
    done
    echo ""
    echo "  Rollback:"
    echo "    ls $BIN_DIR/previous/"
    echo "    sudo -u taskschmiede cp $BIN_DIR/previous/<version>/* $BIN_DIR/"
    echo "    sudo systemctl restart ${SERVICES[*]}"
    echo "============================================================"
    exit 1
fi
