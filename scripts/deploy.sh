#!/usr/bin/env bash
# deploy.sh -- Deploy Taskschmiede to staging or production.
#
# Usage: deploy.sh <staging|intranet|production> <package.tar.gz>
#
# Uploads the package to the target via scp, extracts it, runs the
# install script, and cleans up. Production deploys require interactive
# confirmation.
#
# Production uses ~/staging/ as the working directory (not /tmp/) to
# match scoped sudoers rules that avoid /tmp/ race conditions.
# Staging and intranet use /tmp/.

set -euo pipefail

TARGET="${1:-}"
PACKAGE="${2:-}"

# -- Helpers --
usage() {
    echo "Usage: $0 <staging|intranet|production> <package.tar.gz>"
    echo ""
    echo "Deploys a Taskschmiede package to the specified environment."
    echo "The package is created by 'make package-saas' or 'make package-community'."
    echo ""
    echo "Environments:"
    echo "  staging     Staging server"
    echo "  intranet    Intranet (community edition)"
    echo "  production  Production server"
    exit 1
}

die() {
    echo "Error: $*" >&2
    exit 1
}

# Resolve target to SSH host, label, and remote working directory.
# SSH hosts are read from environment variables (DEPLOY_HOST_STAGING,
# DEPLOY_HOST_INTRANET, DEPLOY_HOST_PRODUCTION) or SSH config aliases.
resolve_target() {
    case "$1" in
        staging)
            SSH_HOST="${DEPLOY_HOST_STAGING:-staging}"
            TARGET_LABEL="Staging"
            REMOTE_DIR="/tmp"
            ;;
        intranet)
            SSH_HOST="${DEPLOY_HOST_INTRANET:-intranet}"
            TARGET_LABEL="Intranet (community edition)"
            REMOTE_DIR="/tmp"
            ;;
        production)
            SSH_HOST="${DEPLOY_HOST_PRODUCTION:-production}"
            TARGET_LABEL="Production"
            REMOTE_DIR="staging"    # ~/staging/ (deploy user's home)
            ;;
        *)
            die "Unknown target '$1'. Use: staging, intranet, production"
            ;;
    esac
}

# -- Validation --
if [ -z "$TARGET" ] || [ -z "$PACKAGE" ]; then
    usage
fi

resolve_target "$TARGET"

if [ ! -f "$PACKAGE" ]; then
    die "Package not found: $PACKAGE"
fi

PACKAGE_NAME="$(basename "$PACKAGE")"

# -- Safety check for production --
if [ "$TARGET" = "production" ]; then
    echo ""
    echo "WARNING: You are about to deploy to PRODUCTION."
    echo "  Package: $PACKAGE_NAME"
    echo "  Target:  $TARGET_LABEL"
    echo ""
    printf "Type 'yes' to continue: "
    read -r answer
    if [ "$answer" != "yes" ]; then
        echo "Aborted."
        exit 1
    fi
fi

echo ""
echo "============================================================"
echo "  Deploying to $TARGET_LABEL"
echo "  Package: $PACKAGE_NAME"
echo "============================================================"
echo ""

# -- Step 1: Upload --
echo "Uploading package..."
ssh "$SSH_HOST" "mkdir -p $REMOTE_DIR"
scp "$PACKAGE" "$SSH_HOST:$REMOTE_DIR/$PACKAGE_NAME"
echo "  Uploaded to $SSH_HOST:$REMOTE_DIR/$PACKAGE_NAME"
echo ""

# -- Step 2: Extract and install --
echo "Running remote install..."
echo "------------------------------------------------------------"
ssh "$SSH_HOST" bash -s -- "$PACKAGE_NAME" "$REMOTE_DIR" <<'REMOTE_EOF'
set -euo pipefail

PACKAGE_NAME="$1"
REMOTE_DIR="$2"

cd "$REMOTE_DIR"

# Clean stale extracted directories from previous failed deploys
rm -rf taskschmiede-*/

tar xzf "$PACKAGE_NAME"

# Find the extracted directory (taskschmiede-*)
PKG_DIR=""
for d in taskschmiede-*/; do
    if [ -d "$d" ]; then
        PKG_DIR="${d%/}"
        break
    fi
done

if [ -z "$PKG_DIR" ]; then
    echo "Error: Could not find extracted package directory" >&2
    exit 1
fi

PKG_PATH="$(pwd)/$PKG_DIR"
WORK_DIR="$(pwd)"

# Copy install script to fixed path for sudoers scoping.
# On production, sudoers allows only ~/staging/install-taskschmiede.sh.
# The --package-dir flag tells the script where to find binaries.
cp "$PKG_DIR/install-taskschmiede.sh" "$WORK_DIR/install-taskschmiede.sh"
chmod +x "$WORK_DIR/install-taskschmiede.sh"
sudo bash "$WORK_DIR/install-taskschmiede.sh" --package-dir="$PKG_PATH"
INSTALL_EXIT=$?

# Clean up (own files, no sudo needed)
rm -rf "$WORK_DIR/$PACKAGE_NAME" "$WORK_DIR/$PKG_DIR" "$WORK_DIR/install-taskschmiede.sh"

exit $INSTALL_EXIT
REMOTE_EOF

echo ""
echo "============================================================"
echo "  Deployment to $TARGET_LABEL complete."
echo "============================================================"
