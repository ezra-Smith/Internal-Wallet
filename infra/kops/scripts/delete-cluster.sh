#!/bin/bash
# Delete kOps cluster
#
# WARNING: This will delete all cluster resources!
#
# Usage:
#   source ../env.sh
#   ./delete-cluster.sh [--yes]

set -euo pipefail

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

while [[ $# -gt 0 ]]; do
    case $1 in
        --yes|-y)
            AUTO_APPROVE=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Usage: $0 [--yes]"
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
    check_env

    if ! cluster_exists; then
        log_warn "Cluster $KOPS_CLUSTER_NAME does not exist in state store"
        exit 0
    fi

    echo ""
    log_warn "============================================================"
    log_warn "WARNING: This will DELETE the cluster and ALL its resources!"
    log_warn "============================================================"
    echo ""
    echo "Cluster: $KOPS_CLUSTER_NAME"
    echo "State Store: $KOPS_STATE_STORE"
    echo ""
    echo "Resources that will be deleted:"
    echo "  - EC2 instances (master, workers, bastion)"
    echo "  - Load balancers"
    echo "  - EBS volumes"
    echo "  - VPC and networking"
    echo "  - Security groups"
    echo "  - IAM roles"
    echo "  - Route53 records"
    echo ""

    if [ "$AUTO_APPROVE" = true ]; then
        log_warn "Auto-approve enabled. Deleting cluster..."
        kops delete cluster --name "$KOPS_CLUSTER_NAME" --yes

        log_info "Cluster deleted successfully"
        log_info "State store (S3 bucket) was NOT deleted. Delete manually if needed."
    else
        log_info "Preview of resources to be deleted:"
        kops delete cluster --name "$KOPS_CLUSTER_NAME"

        echo ""
        log_warn "To confirm deletion, run:"
        echo "  $0 --yes"
        echo ""
    fi
}

main "$@"
