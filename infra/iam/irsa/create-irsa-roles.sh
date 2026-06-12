#!/bin/bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

require_env() {
  local vars=("AWS_REGION" "SECRETS_ENV")
  for v in "${vars[@]}"; do
    if [ -z "${!v:-}" ]; then
      log_error "Missing required env var: $v"
      exit 1
    fi
  done
}

ensure_account_id() {
  if [ -z "${AWS_ACCOUNT_ID:-}" ]; then
    AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
  fi
}

detect_oidc_provider() {
  if [ -n "${OIDC_PROVIDER_ARN:-}" ]; then
    return 0
  fi

  if [ -z "${KOPS_OIDC_DISCOVERY_BUCKET:-}" ] && [ -z "${KOPS_CLUSTER_NAME:-}" ]; then
    log_error "Set OIDC_PROVIDER_ARN or provide KOPS_OIDC_DISCOVERY_BUCKET / KOPS_CLUSTER_NAME for auto-detection."
    exit 1
  fi

  log_info "Detecting IAM OIDC provider..."

  local providers
  providers="$(aws iam list-open-id-connect-providers --query 'OpenIDConnectProviderList[].Arn' --output text)"
  if [ -z "$providers" ]; then
    log_error "No IAM OIDC providers found in account $AWS_ACCOUNT_ID"
    exit 1
  fi

  local arn url
  for arn in $providers; do
    url="$(aws iam get-open-id-connect-provider --open-id-connect-provider-arn "$arn" --query Url --output text)"
    if [ -n "${KOPS_OIDC_DISCOVERY_BUCKET:-}" ] && [[ "$url" == *"$KOPS_OIDC_DISCOVERY_BUCKET"* ]]; then
      OIDC_PROVIDER_ARN="$arn"
      log_info "Found OIDC provider by bucket match: $OIDC_PROVIDER_ARN ($url)"
      return 0
    fi
    if [ -n "${KOPS_CLUSTER_NAME:-}" ] && [[ "$url" == *"$KOPS_CLUSTER_NAME"* ]]; then
      OIDC_PROVIDER_ARN="$arn"
      log_info "Found OIDC provider by cluster match: $OIDC_PROVIDER_ARN ($url)"
      return 0
    fi
  done

  log_error "Failed to auto-detect OIDC provider. Set OIDC_PROVIDER_ARN explicitly."
  exit 1
}

get_oidc_url() {
  local url
  url="$(aws iam get-open-id-connect-provider --open-id-connect-provider-arn "$OIDC_PROVIDER_ARN" --query Url --output text)"
  url="${url#https://}"
  echo "$url"
}

secret_arn() {
  local name="$1"
  echo "arn:aws:secretsmanager:${AWS_REGION}:${AWS_ACCOUNT_ID}:secret:${name}-*"
}

ensure_role() {
  local role_name="$1"
  local sa_namespace="$2"
  local sa_name="$3"
  shift 3
  local secret_arns=("$@")

  local oidc_url
  oidc_url="$(get_oidc_url)"

  local trust_tmp policy_tmp
  trust_tmp="$(mktemp)"
  policy_tmp="$(mktemp)"

  cat >"$trust_tmp" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Federated": "${OIDC_PROVIDER_ARN}" },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${oidc_url}:sub": "system:serviceaccount:${sa_namespace}:${sa_name}",
          "${oidc_url}:aud": ["sts.amazonaws.com", "amazonaws.com"]
        }
      }
    }
  ]
}
EOF

  if aws iam get-role --role-name "$role_name" >/dev/null 2>&1; then
    log_info "Updating trust policy: $role_name"
    aws iam update-assume-role-policy --role-name "$role_name" --policy-document "file://${trust_tmp}"
  else
    log_info "Creating role: $role_name"
    aws iam create-role --role-name "$role_name" --assume-role-policy-document "file://${trust_tmp}" >/dev/null
  fi

  # Permission policy (inline)
  {
    echo '{'
    echo '  "Version": "2012-10-17",'
    echo '  "Statement": ['
    echo '    {'
    echo '      "Sid": "ReadSecretsManager",'
    echo '      "Effect": "Allow",'
    echo '      "Action": ['
    echo '        "secretsmanager:GetSecretValue",'
    echo '        "secretsmanager:DescribeSecret",'
    echo '        "secretsmanager:ListSecretVersionIds"'
    echo '      ],'
    echo '      "Resource": ['
    local i
    for i in "${!secret_arns[@]}"; do
      local comma=","
      if [ "$i" -eq "$((${#secret_arns[@]}-1))" ]; then
        comma=""
      fi
      printf '        "%s"%s\n' "${secret_arns[$i]}" "$comma"
    done
    echo '      ]'
    echo '    }'

    if [ -n "${KMS_KEY_ARN:-}" ]; then
      echo '    ,{'
      echo '      "Sid": "DecryptSecretsManagerKMS",'
      echo '      "Effect": "Allow",'
      echo '      "Action": ["kms:Decrypt"],'
      echo "      \"Resource\": [\"${KMS_KEY_ARN}\"]"
      echo '    }'
    fi

    echo '  ]'
    echo '}'
  } >"$policy_tmp"

  log_info "Putting inline policy: $role_name"
  aws iam put-role-policy --role-name "$role_name" --policy-name "${role_name}" --policy-document "file://${policy_tmp}" >/dev/null

  rm -f "$trust_tmp" "$policy_tmp"
}

ensure_role_rw() {
  local role_name="$1"
  local sa_namespace="$2"
  local sa_name="$3"
  shift 3
  local secret_arns=("$@")

  local oidc_url
  oidc_url="$(get_oidc_url)"

  local trust_tmp policy_tmp
  trust_tmp="$(mktemp)"
  policy_tmp="$(mktemp)"

  cat >"$trust_tmp" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Federated": "${OIDC_PROVIDER_ARN}" },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${oidc_url}:sub": "system:serviceaccount:${sa_namespace}:${sa_name}",
          "${oidc_url}:aud": ["sts.amazonaws.com", "amazonaws.com"]
        }
      }
    }
  ]
}
EOF

  if aws iam get-role --role-name "$role_name" >/dev/null 2>&1; then
    log_info "Updating trust policy: $role_name"
    aws iam update-assume-role-policy --role-name "$role_name" --policy-document "file://${trust_tmp}"
  else
    log_info "Creating role: $role_name"
    aws iam create-role --role-name "$role_name" --assume-role-policy-document "file://${trust_tmp}" >/dev/null
  fi

  # Permission policy (inline) - READ/WRITE for Secrets Manager
  {
    echo '{'
    echo '  "Version": "2012-10-17",'
    echo '  "Statement": ['
    echo '    {'
    echo '      "Sid": "CreateSecretsManagerSecret",'
    echo '      "Effect": "Allow",'
    echo '      "Action": ["secretsmanager:CreateSecret"],'
    echo '      "Resource": "*"'
    echo '    },'
    echo '    {'
    echo '      "Sid": "ReadWriteSecretsManager",'
    echo '      "Effect": "Allow",'
    echo '      "Action": ['
    echo '        "secretsmanager:PutSecretValue",'
    echo '        "secretsmanager:DescribeSecret",'
    echo '        "secretsmanager:GetSecretValue",'
    echo '        "secretsmanager:ListSecretVersionIds"'
    echo '      ],'
    echo '      "Resource": ['
    local i
    for i in "${!secret_arns[@]}"; do
      local comma=","
      if [ "$i" -eq "$((${#secret_arns[@]}-1))" ]; then
        comma=""
      fi
      printf '        "%s"%s\n' "${secret_arns[$i]}" "$comma"
    done
    echo '      ]'
    echo '    }'

    if [ -n "${KMS_KEY_ARN:-}" ]; then
      echo '    ,{'
      echo '      "Sid": "DecryptSecretsManagerKMS",'
      echo '      "Effect": "Allow",'
      echo '      "Action": ["kms:Decrypt"],'
      echo "      \"Resource\": [\"${KMS_KEY_ARN}\"]"
      echo '    }'
    fi

    echo '  ]'
    echo '}'
  } >"$policy_tmp"

  log_info "Putting inline policy: $role_name"
  aws iam put-role-policy --role-name "$role_name" --policy-name "${role_name}" --policy-document "file://${policy_tmp}" >/dev/null

  rm -f "$trust_tmp" "$policy_tmp"
}

main() {
  require_env
  ensure_account_id
  detect_oidc_provider

  case "$SECRETS_ENV" in
    production) ;;
    *)
      log_error "SECRETS_ENV must be production (got: $SECRETS_ENV)"
      exit 1
      ;;
  esac

  local prefix="crypto-wallet/${SECRETS_ENV}"

  log_info "AWS account: $AWS_ACCOUNT_ID"
  log_info "AWS region: $AWS_REGION"
  log_info "Secrets env: $SECRETS_ENV"
  log_info "OIDC provider: $OIDC_PROVIDER_ARN"

  # App services (wallet-biz namespace)
  ensure_role \
    "wallet-app-secrets" \
    "wallet-biz" \
    "wallet-app" \
    "$(secret_arn "${prefix}/database")" \
    "$(secret_arn "${prefix}/redis")" \
    "$(secret_arn "${prefix}/jwt")" \
    "$(secret_arn "${prefix}/admin-jwt")" \
    "$(secret_arn "${prefix}/smtp")" \
    "$(secret_arn "${prefix}/swap")" \
    "$(secret_arn "${prefix}/sms")" \
    "$(secret_arn "${prefix}/s3-config")" \
    "$(secret_arn "${prefix}/chainrpc")" \
    "$(secret_arn "${prefix}/chainsync")" \
    "$(secret_arn "${prefix}/consolidation")"

  ensure_role \
    "wallet-signer-secrets" \
    "wallet-biz" \
    "wallet-signer" \
    "$(secret_arn "${prefix}/database")" \
    "$(secret_arn "${prefix}/redis")" \
    "$(secret_arn "${prefix}/signer")"

  # ESO controller (external-secrets namespace)
  ensure_role \
    "wallet-infra-secrets" \
    "external-secrets" \
    "external-secrets" \
    "$(secret_arn "${prefix}/database")" \
    "$(secret_arn "${prefix}/redis")" \
    "$(secret_arn "${prefix}/mariadb-root")" \
    "$(secret_arn "${prefix}/grafana")" \
    "$(secret_arn "${prefix}/cloudflared")" \
    "$(secret_arn "${prefix}/admin-bootstrap")"

  # Bootstrapper job (wallet-infra namespace): write bootstrap admin creds to Secrets Manager.
  ensure_role_rw \
    "wallet-bootstrapper-secrets" \
    "wallet-infra" \
    "wallet-bootstrapper" \
    "$(secret_arn "${prefix}/admin-bootstrap")"

  log_info "IRSA roles ensured."
  log_info "wallet-app-secrets ARN: arn:aws:iam::${AWS_ACCOUNT_ID}:role/wallet-app-secrets"
  log_info "wallet-signer-secrets ARN: arn:aws:iam::${AWS_ACCOUNT_ID}:role/wallet-signer-secrets"
  log_info "wallet-infra-secrets ARN: arn:aws:iam::${AWS_ACCOUNT_ID}:role/wallet-infra-secrets"
  log_info "wallet-bootstrapper-secrets ARN: arn:aws:iam::${AWS_ACCOUNT_ID}:role/wallet-bootstrapper-secrets"
}

main "$@"
