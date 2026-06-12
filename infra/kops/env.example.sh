#!/bin/bash
# kOps Environment Variables
# Copy this file to env.sh and customize for your environment:
#   cp env.example.sh env.sh
#   source env.sh

# ============================================================
# AWS Configuration
# ============================================================
export AWS_REGION="ap-northeast-1"
export AWS_ZONE="ap-northeast-1a"
export AWS_ACCOUNT_ID="301918028034"

# If your local AWS auth is provided by a non-standard source (e.g. `aws login`),
# create an AWS profile with credential_process and enable shared config loading:
#   [profile kops]
#   region = ap-northeast-1
#   credential_process = aws configure export-credentials --profile default --format process
#
# Then run kOps with:
#   export AWS_PROFILE="kops"
#   export AWS_SDK_LOAD_CONFIG="1"

# ============================================================
# kOps Configuration
# ============================================================
export KOPS_CLUSTER_NAME="k8s.zinkapi.com"
export KOPS_STATE_STORE="s3://kops-state-zinkapi"

# ============================================================
# IRSA / OIDC Discovery (for ServiceAccountIssuerDiscovery)
# ============================================================
# kOps publishes OIDC discovery documents to this bucket.
# This bucket MUST allow public HTTPS GET for the published objects (STS will fetch them).
export KOPS_OIDC_DISCOVERY_BUCKET="k8s-zinkapi-com-oidc"

# ============================================================
# Route53 Configuration
# ============================================================
# The parent domain managed by Cloudflare (NS delegation to Route53)
export ROUTE53_ZONE_NAME="k8s.zinkapi.com"

# ============================================================
# Network Configuration
# ============================================================
export VPC_CIDR="10.0.0.0/16"
export PUBLIC_SUBNET_CIDR="10.0.0.0/24"
export PRIVATE_SUBNET_CIDR="10.0.128.0/17"

# ============================================================
# SSH Configuration
# ============================================================
# Path to your SSH public key for node access
export SSH_PUBLIC_KEY="~/.ssh/id_ed25519.pub"

# Allowed CIDR for SSH access to bastion (your IP or office network)
# Example: "203.0.113.10/32" or "10.0.0.0/8"
export SSH_ALLOWED_CIDR="0.0.0.0/0"  # CHANGE THIS! Restrict to your IP

# ============================================================
# ECR Configuration
# ============================================================
export ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
export ECR_REPOSITORY_PREFIX="internal-wallet"

# ============================================================
# GitHub Configuration (for OIDC)
# ============================================================
export GITHUB_ORG="97-web3"
export GITHUB_REPO="Internal-Wallet"

# ============================================================
# Convenience aliases
# ============================================================
alias kops-env='echo "KOPS_CLUSTER_NAME=$KOPS_CLUSTER_NAME"; echo "KOPS_STATE_STORE=$KOPS_STATE_STORE"'
alias kops-validate='kops validate cluster --name $KOPS_CLUSTER_NAME --wait 10m'
alias kops-export='kops export kubeconfig --name $KOPS_CLUSTER_NAME --admin'
