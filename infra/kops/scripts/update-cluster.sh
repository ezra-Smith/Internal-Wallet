#!/bin/bash
# Update kOps cluster
#
# Usage:
#   source ../env.sh
#   ./update-cluster.sh [--yes] [--rolling-update]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KOPS_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Parse arguments
AUTO_APPROVE=false
ROLLING_UPDATE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --yes|-y)
            AUTO_APPROVE=true
            shift
            ;;
        --rolling-update|-r)
            ROLLING_UPDATE=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Usage: $0 [--yes] [--rolling-update]"
            exit 1
            ;;
    esac
done

# Check environment
check_env() {
    if [ -z "${KOPS_CLUSTER_NAME:-}" ] || [ -z "${KOPS_STATE_STORE:-}" ]; then
        log_error "Environment variables not set. Please run: source env.sh"
        exit 1
    fi
}

# Check if cluster exists
cluster_exists() {
    kops get cluster --name "$KOPS_CLUSTER_NAME" &>/dev/null
}

# Main
main() {
    log_info "Updating kOps cluster: $KOPS_CLUSTER_NAME"

    check_env

    if ! cluster_exists; then
        log_error "Cluster $KOPS_CLUSTER_NAME does not exist"
        log_info "Use ./create-cluster.sh to create it first"
        exit 1
    fi

    # Replace cluster spec from manifest
    log_info "Replacing cluster spec from manifest..."
    kops replace -f "$KOPS_DIR/cluster.yaml" --force

    # Preview or apply changes
    if [ "$AUTO_APPROVE" = true ]; then
        log_info "Applying cluster changes..."
        kops update cluster --name "$KOPS_CLUSTER_NAME" --yes --admin

        # Rolling update if requested
        if [ "$ROLLING_UPDATE" = true ]; then
            log_info "Performing rolling update..."
            kops rolling-update cluster --name "$KOPS_CLUSTER_NAME" --yes
        fi

        # Validate
        log_info "Validating cluster..."
        kops validate cluster --name "$KOPS_CLUSTER_NAME" --wait 10m
    else
        log_info "Previewing cluster changes..."
        kops update cluster --name "$KOPS_CLUSTER_NAME"

        echo ""
        log_warn "This is a dry-run. To apply changes, run:"
        echo "  $0 --yes"
        echo ""
        log_info "To also perform rolling update (recreate nodes):"
        echo "  $0 --yes --rolling-update"
        echo ""
    fi
}

main "$@"
