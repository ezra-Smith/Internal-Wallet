#!/bin/bash
# Setup GitHub Actions Self-Hosted Runner on EC2
#
# Prerequisites:
# - Ubuntu 22.04 EC2 instance in kOps VPC private subnet
# - Instance can reach internet via NAT Gateway
# - GitHub Personal Access Token with repo scope
#
# Usage:
#   export GITHUB_TOKEN="ghp_xxxx"
#   ./setup-runner.sh

set -euo pipefail

# Configuration
RUNNER_VERSION="2.311.0"
GITHUB_ORG="97-web3"
GITHUB_REPO="Internal-Wallet"
RUNNER_NAME="${RUNNER_NAME:-$(hostname)}"
RUNNER_LABELS="${RUNNER_LABELS:-self-hosted,linux,x64,aws}"
RUNNER_WORK_DIR="/opt/actions-runner/_work"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Please run as root (sudo)"
        exit 1
    fi
}

# Install system dependencies
install_dependencies() {
    log_info "Installing system dependencies..."

    apt-get update
    apt-get install -y \
        curl \
        wget \
        git \
        jq \
        unzip \
        apt-transport-https \
        ca-certificates \
        gnupg \
        lsb-release \
        software-properties-common
}

# Install Docker
install_docker() {
    log_info "Installing Docker..."

    if command -v docker &> /dev/null; then
        log_info "Docker already installed"
        return
    fi

    curl -fsSL https://get.docker.com | sh

    # Add ubuntu user to docker group
    usermod -aG docker ubuntu

    # Start Docker
    systemctl enable docker
    systemctl start docker

    log_info "Docker installed successfully"
}

# Install kubectl
install_kubectl() {
    log_info "Installing kubectl..."

    if command -v kubectl &> /dev/null; then
        log_info "kubectl already installed"
        return
    fi

    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl

    log_info "kubectl installed: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
}

# Install kops
install_kops() {
    log_info "Installing kops..."

    if command -v kops &> /dev/null; then
        log_info "kops already installed"
        return
    fi

    curl -Lo kops "https://github.com/kubernetes/kops/releases/download/v1.29.0/kops-linux-amd64"
    install -o root -g root -m 0755 kops /usr/local/bin/kops
    rm kops

    log_info "kops installed: $(kops version --short)"
}

# Install AWS CLI v2
install_awscli() {
    log_info "Installing AWS CLI v2..."

    if command -v aws &> /dev/null; then
        log_info "AWS CLI already installed"
        return
    fi

    curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
    unzip -q awscliv2.zip
    ./aws/install
    rm -rf aws awscliv2.zip

    log_info "AWS CLI installed: $(aws --version)"
}

# Install kustomize
install_kustomize() {
    log_info "Installing kustomize..."

    if command -v kustomize &> /dev/null; then
        log_info "kustomize already installed"
        return
    fi

    curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash
    mv kustomize /usr/local/bin/

    log_info "kustomize installed: $(kustomize version)"
}

# Get GitHub runner registration token
get_runner_token() {
    if [ -z "${GITHUB_TOKEN:-}" ]; then
        log_error "GITHUB_TOKEN environment variable is required"
        log_info "Create a token at: https://github.com/settings/tokens"
        log_info "Required scope: repo"
        exit 1
    fi

    log_info "Getting runner registration token..."

    local token_response
    token_response=$(curl -s -X POST \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/actions/runners/registration-token")

    RUNNER_TOKEN=$(echo "$token_response" | jq -r '.token')

    if [ "$RUNNER_TOKEN" = "null" ] || [ -z "$RUNNER_TOKEN" ]; then
        log_error "Failed to get runner token. Response:"
        echo "$token_response"
        exit 1
    fi
}

# Install and configure GitHub Actions Runner
install_runner() {
    log_info "Installing GitHub Actions Runner..."

    local runner_dir="/opt/actions-runner"

    # Create runner directory
    mkdir -p "$runner_dir"
    cd "$runner_dir"

    # Download runner
    if [ ! -f "actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz" ]; then
        curl -o "actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz" -L \
            "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
    fi

    # Extract
    tar xzf "actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"

    # Fix ownership
    chown -R ubuntu:ubuntu "$runner_dir"

    # Get registration token
    get_runner_token

    # Configure runner as ubuntu user
    log_info "Configuring runner..."
    sudo -u ubuntu ./config.sh \
        --url "https://github.com/${GITHUB_ORG}/${GITHUB_REPO}" \
        --token "$RUNNER_TOKEN" \
        --name "$RUNNER_NAME" \
        --labels "$RUNNER_LABELS" \
        --work "$RUNNER_WORK_DIR" \
        --unattended \
        --replace

    # Install as service
    log_info "Installing runner service..."
    ./svc.sh install ubuntu
    ./svc.sh start

    log_info "Runner installed and started"
}

# Print summary
print_summary() {
    echo ""
    echo "============================================================"
    echo "GitHub Actions Runner Setup Complete"
    echo "============================================================"
    echo ""
    echo "Runner Name: $RUNNER_NAME"
    echo "Labels: $RUNNER_LABELS"
    echo "Work Directory: $RUNNER_WORK_DIR"
    echo ""
    echo "Installed tools:"
    echo "  - Docker: $(docker --version)"
    echo "  - kubectl: $(kubectl version --client --short 2>/dev/null || echo 'installed')"
    echo "  - kops: $(kops version --short)"
    echo "  - AWS CLI: $(aws --version)"
    echo "  - kustomize: $(kustomize version --short)"
    echo ""
    echo "Service status:"
    systemctl status actions.runner.${GITHUB_ORG}-${GITHUB_REPO}.${RUNNER_NAME} --no-pager || true
    echo ""
    echo "View runner at:"
    echo "  https://github.com/${GITHUB_ORG}/${GITHUB_REPO}/settings/actions/runners"
    echo ""
}

# Main
main() {
    log_info "Starting GitHub Actions Runner setup"

    check_root
    install_dependencies
    install_docker
    install_kubectl
    install_kops
    install_awscli
    install_kustomize
    install_runner
    print_summary
}

main "$@"
