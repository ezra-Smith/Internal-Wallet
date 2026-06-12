#!/bin/bash
# Create kOps cluster
#
# Usage:
#   source ../env.sh
#   ./create-cluster.sh

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

# Check environment
check_env() {
    if [ -z "${KOPS_CLUSTER_NAME:-}" ] || [ -z "${KOPS_STATE_STORE:-}" ]; then
        log_error "Environment variables not set. Please run: source env.sh"
        exit 1
    fi

    if [ -z "${SSH_PUBLIC_KEY:-}" ]; then
        export SSH_PUBLIC_KEY="$HOME/.ssh/id_ed25519.pub"
    fi

    if [ ! -f "${SSH_PUBLIC_KEY/#\~/$HOME}" ]; then
        log_error "SSH public key not found: $SSH_PUBLIC_KEY"
        log_info "Generate one with: ssh-keygen -t ed25519"
        exit 1
    fi
}

# Check if cluster exists
cluster_exists() {
    kops get cluster --name "$KOPS_CLUSTER_NAME" &>/dev/null
}

# Main
main() {
    log_info "Creating kOps cluster: $KOPS_CLUSTER_NAME"

    check_env

    # Check if cluster already exists
    if cluster_exists; then
        log_warn "Cluster $KOPS_CLUSTER_NAME already exists in state store"
        log_info "Use ./update-cluster.sh to update, or delete first"
        exit 1
    fi

    # Create cluster from manifest
    log_info "Creating cluster from manifest..."
    kops create -f "$KOPS_DIR/cluster.yaml"

    # Add SSH public key
    log_info "Adding SSH public key..."
    kops create secret sshpublickey admin \
        -i "${SSH_PUBLIC_KEY/#\~/$HOME}" \
        --name "$KOPS_CLUSTER_NAME"

    # Preview changes
    log_info "Previewing cluster changes..."
    kops update cluster --name "$KOPS_CLUSTER_NAME"

    echo ""
    log_warn "Review the changes above. To apply, run:"
    echo "  kops update cluster --name $KOPS_CLUSTER_NAME --yes --admin"
    echo ""
    log_info "After applying, validate with:"
    echo "  kops validate cluster --name $KOPS_CLUSTER_NAME --wait 15m"
    echo ""
}

main "$@"
