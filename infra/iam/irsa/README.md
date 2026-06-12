# IRSA IAM Roles (kOps) for AWS Secrets Manager

This folder contains a small, **idempotent** bootstrap script to create the IAM roles used by:

- App pods (via Secrets Store CSI Driver + AWS provider)
- External Secrets Operator (ESO) controller (to materialize infra Kubernetes Secrets)

## Prereqs

- `aws` CLI configured with permissions to manage IAM roles/policies.
- kOps IRSA already enabled and applied (`spec.serviceAccountIssuerDiscovery.enableAWSOIDCProvider: true`).
- OIDC discovery bucket exists (see `infra/kops/scripts/ensure-irsa-oidc-bucket.sh`).

## Usage

```bash
export AWS_REGION=ap-northeast-1
export SECRETS_ENV=production

# Optional helpers (auto-detected if omitted)
export KOPS_OIDC_DISCOVERY_BUCKET=k8s-zinkapi-com-oidc
export KOPS_CLUSTER_NAME=k8s.zinkapi.com

./infra/iam/irsa/create-irsa-roles.sh
```

If your org uses a customer-managed KMS key for Secrets Manager secrets, also set:

```bash
export KMS_KEY_ARN='arn:aws:kms:ap-northeast-1:123456789012:key/xxxx-xxxx-xxxx'
```

## What it creates

- `wallet-app-secrets` (SA: `wallet-biz/wallet-app`)
- `wallet-signer-secrets` (SA: `wallet-biz/wallet-signer`)
- `wallet-infra-secrets` (SA: `external-secrets/external-secrets`)

All policies are scoped to `crypto-wallet/${SECRETS_ENV}/*` Secrets Manager secret ARNs (with `-*` suffix wildcard).
