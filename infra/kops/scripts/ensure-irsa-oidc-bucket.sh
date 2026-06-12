#!/bin/bash
# Ensure S3 bucket exists and is configured for kOps IRSA OIDC discovery docs.
#
# Required env:
#   AWS_REGION
#   KOPS_OIDC_DISCOVERY_BUCKET
#
# NOTE:
# - STS must be able to fetch OIDC discovery objects over HTTPS, so the bucket policy enables public GET.
# - We still block public ACLs; public read is policy-based only.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

require_env() {
  local vars=("AWS_REGION" "KOPS_OIDC_DISCOVERY_BUCKET")
  for v in "${vars[@]}"; do
    if [ -z "${!v:-}" ]; then
      log_error "Missing required env var: $v"
      exit 1
    fi
  done
}

create_or_configure_bucket() {
  local bucket="$KOPS_OIDC_DISCOVERY_BUCKET"

  log_info "Ensuring IRSA OIDC discovery bucket exists: $bucket"

  if aws s3api head-bucket --bucket "$bucket" 2>/dev/null; then
    log_info "Bucket exists: $bucket"
  else
    log_warn "Bucket not found, creating: $bucket"
    if [ "$AWS_REGION" = "us-east-1" ]; then
      aws s3api create-bucket --bucket "$bucket" --region "$AWS_REGION"
    else
      aws s3api create-bucket \
        --bucket "$bucket" \
        --region "$AWS_REGION" \
        --create-bucket-configuration LocationConstraint="$AWS_REGION"
    fi
  fi

  log_info "Enabling versioning on $bucket"
  aws s3api put-bucket-versioning \
    --bucket "$bucket" \
    --versioning-configuration Status=Enabled

  log_info "Enabling SSE-S3 encryption on $bucket"
  aws s3api put-bucket-encryption \
    --bucket "$bucket" \
    --server-side-encryption-configuration '{
      "Rules": [{
        "ApplyServerSideEncryptionByDefault": { "SSEAlgorithm": "AES256" },
        "BucketKeyEnabled": true
      }]
    }'

  log_info "Setting public access block (policy-based public read only) on $bucket"
  aws s3api put-public-access-block \
    --bucket "$bucket" \
    --public-access-block-configuration '{
      "BlockPublicAcls": true,
      "IgnorePublicAcls": true,
      "BlockPublicPolicy": false,
      "RestrictPublicBuckets": false
    }'

  log_info "Applying bucket policy for public HTTPS GET (OIDC discovery) on $bucket"
  aws s3api put-bucket-policy --bucket "$bucket" --policy "{
    \"Version\": \"2012-10-17\",
    \"Statement\": [
      {
        \"Sid\": \"AllowOIDCDiscoveryRead\",
        \"Effect\": \"Allow\",
        \"Principal\": \"*\",
        \"Action\": [\"s3:GetObject\"],
        \"Resource\": [\"arn:aws:s3:::$bucket/*\"]
      },
      {
        \"Sid\": \"DenyInsecureTransport\",
        \"Effect\": \"Deny\",
        \"Principal\": \"*\",
        \"Action\": \"s3:*\",
        \"Resource\": [\"arn:aws:s3:::$bucket\", \"arn:aws:s3:::$bucket/*\"],
        \"Condition\": {\"Bool\": {\"aws:SecureTransport\": \"false\"}}
      }
    ]
  }"

  log_info "OIDC discovery bucket ready: $bucket"
}

main() {
  require_env
  create_or_configure_bucket
}

main "$@"

