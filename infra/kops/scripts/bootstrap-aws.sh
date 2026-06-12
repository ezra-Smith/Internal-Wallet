#!/bin/bash
# Bootstrap AWS resources for kOps
# Creates S3 state store and validates Route53 hosted zone
#
# Usage:
#   source ../env.sh
#   ./bootstrap-aws.sh

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check required environment variables
check_env() {
    local required_vars=(
        "AWS_REGION"
        "KOPS_CLUSTER_NAME"
        "KOPS_STATE_STORE"
        "KOPS_OIDC_DISCOVERY_BUCKET"
        "ROUTE53_ZONE_NAME"
    )

    for var in "${required_vars[@]}"; do
        if [ -z "${!var:-}" ]; then
            log_error "Required environment variable $var is not set"
            log_info "Please run: source env.sh"
            exit 1
        fi
    done
}

# Extract bucket name from KOPS_STATE_STORE
get_bucket_name() {
    echo "${KOPS_STATE_STORE#s3://}"
}

# Create S3 bucket for IRSA OIDC discovery store (serviceAccountIssuerDiscovery)
create_oidc_discovery_bucket() {
    local bucket_name="${KOPS_OIDC_DISCOVERY_BUCKET}"

    log_info "Checking IRSA OIDC discovery bucket: $bucket_name"

    if aws s3api head-bucket --bucket "$bucket_name" 2>/dev/null; then
        log_info "OIDC discovery bucket $bucket_name already exists"
    else
        log_info "Creating OIDC discovery bucket: $bucket_name"

        if [ "$AWS_REGION" = "us-east-1" ]; then
            aws s3api create-bucket \
                --bucket "$bucket_name" \
                --region "$AWS_REGION"
        else
            aws s3api create-bucket \
                --bucket "$bucket_name" \
                --region "$AWS_REGION" \
                --create-bucket-configuration LocationConstraint="$AWS_REGION"
        fi

        log_info "OIDC discovery bucket created: $bucket_name"
    fi

    # Enable versioning (safety)
    log_info "Enabling versioning on $bucket_name"
    aws s3api put-bucket-versioning \
        --bucket "$bucket_name" \
        --versioning-configuration Status=Enabled

    # Enable server-side encryption
    log_info "Enabling SSE-S3 encryption on $bucket_name"
    aws s3api put-bucket-encryption \
        --bucket "$bucket_name" \
        --server-side-encryption-configuration '{
            "Rules": [{
                "ApplyServerSideEncryptionByDefault": {
                    "SSEAlgorithm": "AES256"
                },
                "BucketKeyEnabled": true
            }]
        }'

    # Public access block: allow public bucket policy but block ACLs
    log_info "Configuring public access block on $bucket_name (policy-based public read only)"
    aws s3api put-public-access-block \
        --bucket "$bucket_name" \
        --public-access-block-configuration '{
            "BlockPublicAcls": true,
            "IgnorePublicAcls": true,
            "BlockPublicPolicy": false,
            "RestrictPublicBuckets": false
        }'

    # Bucket policy: allow HTTPS GET for OIDC discovery objects (STS fetch), deny non-TLS.
    log_info "Applying bucket policy for OIDC discovery public read (HTTPS only) on $bucket_name"
    aws s3api put-bucket-policy --bucket "$bucket_name" --policy "{
      \"Version\": \"2012-10-17\",
      \"Statement\": [
        {
          \"Sid\": \"AllowOIDCDiscoveryRead\",
          \"Effect\": \"Allow\",
          \"Principal\": \"*\",
          \"Action\": [\"s3:GetObject\"],
          \"Resource\": [\"arn:aws:s3:::$bucket_name/*\"]
        },
        {
          \"Sid\": \"DenyInsecureTransport\",
          \"Effect\": \"Deny\",
          \"Principal\": \"*\",
          \"Action\": \"s3:*\",
          \"Resource\": [\"arn:aws:s3:::$bucket_name\", \"arn:aws:s3:::$bucket_name/*\"],
          \"Condition\": {\"Bool\": {\"aws:SecureTransport\": \"false\"}}
        }
      ]
    }"
}

# Create S3 bucket for kOps state store
create_s3_bucket() {
    local bucket_name=$(get_bucket_name)

    log_info "Checking S3 bucket: $bucket_name"

    if aws s3api head-bucket --bucket "$bucket_name" 2>/dev/null; then
        log_info "S3 bucket $bucket_name already exists"
    else
        log_info "Creating S3 bucket: $bucket_name"

        # Create bucket (LocationConstraint required for non-us-east-1)
        if [ "$AWS_REGION" = "us-east-1" ]; then
            aws s3api create-bucket \
                --bucket "$bucket_name" \
                --region "$AWS_REGION"
        else
            aws s3api create-bucket \
                --bucket "$bucket_name" \
                --region "$AWS_REGION" \
                --create-bucket-configuration LocationConstraint="$AWS_REGION"
        fi

        log_info "S3 bucket created: $bucket_name"
    fi

    # Enable versioning
    log_info "Enabling versioning on $bucket_name"
    aws s3api put-bucket-versioning \
        --bucket "$bucket_name" \
        --versioning-configuration Status=Enabled

    # Enable server-side encryption
    log_info "Enabling SSE-S3 encryption on $bucket_name"
    aws s3api put-bucket-encryption \
        --bucket "$bucket_name" \
        --server-side-encryption-configuration '{
            "Rules": [{
                "ApplyServerSideEncryptionByDefault": {
                    "SSEAlgorithm": "AES256"
                },
                "BucketKeyEnabled": true
            }]
        }'

    # Block public access
    log_info "Blocking public access on $bucket_name"
    aws s3api put-public-access-block \
        --bucket "$bucket_name" \
        --public-access-block-configuration '{
            "BlockPublicAcls": true,
            "IgnorePublicAcls": true,
            "BlockPublicPolicy": true,
            "RestrictPublicBuckets": true
        }'

    log_info "S3 bucket configured successfully"
}

# Check/Create Route53 hosted zone
setup_route53() {
    local zone_name="${ROUTE53_ZONE_NAME}"

    log_info "Checking Route53 hosted zone: $zone_name"

    # Check if hosted zone exists
    local zone_id=$(aws route53 list-hosted-zones-by-name \
        --dns-name "$zone_name" \
        --query "HostedZones[?Name=='${zone_name}.'].Id" \
        --output text | head -1 | sed 's|/hostedzone/||')

    if [ -n "$zone_id" ] && [ "$zone_id" != "None" ]; then
        log_info "Route53 hosted zone exists: $zone_id"
    else
        log_info "Creating Route53 hosted zone: $zone_name"

        local caller_ref="kops-$(date +%s)"
        local result=$(aws route53 create-hosted-zone \
            --name "$zone_name" \
            --caller-reference "$caller_ref" \
            --hosted-zone-config Comment="kOps cluster DNS for Internal-Wallet" \
            --output json)

        zone_id=$(echo "$result" | jq -r '.HostedZone.Id' | sed 's|/hostedzone/||')
        log_info "Route53 hosted zone created: $zone_id"
    fi

    # Get NS records for delegation
    log_info "Route53 NS records for Cloudflare delegation:"
    echo ""
    aws route53 get-hosted-zone --id "$zone_id" \
        --query 'DelegationSet.NameServers' \
        --output text | tr '\t' '\n' | while read ns; do
        echo "  NS: $ns"
    done
    echo ""
    log_warn "Add these NS records to Cloudflare for zone: $zone_name"
}

# Create GitHub OIDC provider
create_oidc_provider() {
    local oidc_url="https://token.actions.githubusercontent.com"
    local oidc_arn="arn:aws:iam::${AWS_ACCOUNT_ID:-$(aws sts get-caller-identity --query Account --output text)}:oidc-provider/token.actions.githubusercontent.com"

    log_info "Checking GitHub OIDC provider"

    if aws iam get-open-id-connect-provider --open-id-connect-provider-arn "$oidc_arn" 2>/dev/null; then
        log_info "GitHub OIDC provider already exists"
    else
        log_info "Creating GitHub OIDC provider"

        # Get GitHub's OIDC thumbprint
        local thumbprint="6938fd4d98bab03faadb97b34396831e3780aea1"

        aws iam create-open-id-connect-provider \
            --url "$oidc_url" \
            --client-id-list "sts.amazonaws.com" \
            --thumbprint-list "$thumbprint"

        log_info "GitHub OIDC provider created"
    fi
}

# Print summary
print_summary() {
    local bucket_name=$(get_bucket_name)

    echo ""
    echo "============================================================"
    echo "Bootstrap Complete"
    echo "============================================================"
    echo ""
    echo "S3 State Store:"
    echo "  Bucket: $bucket_name"
    echo "  URI: $KOPS_STATE_STORE"
    echo "  Versioning: Enabled"
    echo "  Encryption: SSE-S3"
    echo "  Public Access: Blocked"
    echo ""
    echo "Route53:"
    echo "  Zone: $ROUTE53_ZONE_NAME"
    echo ""
    echo "Next steps:"
    echo "  1. Add Route53 NS records to Cloudflare (see above)"
    echo "  2. Wait for DNS propagation (~5-10 minutes)"
    echo "  3. Create the cluster:"
    echo "     kops create -f cluster.yaml"
    echo "     kops create secret sshpublickey admin -i ~/.ssh/id_rsa.pub"
    echo "     kops update cluster --name \$KOPS_CLUSTER_NAME --yes --admin"
    echo ""
}

# Main
main() {
    log_info "Starting AWS bootstrap for kOps"

    check_env
    create_s3_bucket
    create_oidc_discovery_bucket
    setup_route53
    create_oidc_provider
    print_summary
}

main "$@"
