# GitHub Actions Self-Hosted Runner

This directory contains configuration for the GitHub Actions self-hosted runner deployed in the kOps VPC.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  AWS VPC (kOps Cluster)                                         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Private Subnet                                          │   │
│  │  ┌─────────────────────────────────────────────────┐    │   │
│  │  │  Runner EC2 (t3.large)                          │    │   │
│  │  │  ├── GitHub Actions Runner                      │    │   │
│  │  │  ├── Docker (for building images)               │    │   │
│  │  │  ├── kubectl (for deployments)                  │    │   │
│  │  │  ├── kops (for cluster management)              │    │   │
│  │  │  └── AWS CLI (for ECR/S3 access)                │    │   │
│  │  └─────────────────────────────────────────────────┘    │   │
│  │                         │                                │   │
│  │           ┌─────────────┼─────────────┐                 │   │
│  │           ▼             ▼             ▼                 │   │
│  │     ┌──────────┐  ┌──────────┐  ┌──────────┐           │   │
│  │     │ K8s API  │  │   ECR    │  │ S3 State │           │   │
│  │     │ (内网)   │  │ (NAT/EP) │  │  Store   │           │   │
│  │     └──────────┘  └──────────┘  └──────────┘           │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Prerequisites

1. kOps cluster is running
2. Runner EC2 instance is in the same VPC (private subnet)
3. Runner can reach:
   - GitHub API (via NAT Gateway)
   - K8s API Server (internal NLB)
   - ECR (via NAT or VPC Endpoint)
   - S3 (via VPC Endpoint)

## EC2 Instance Setup

### 1. Launch EC2 Instance

```bash
# Using AWS CLI (run from your local machine or bastion)
aws ec2 run-instances \
  --image-id ami-0ab3794db9457b60a \  # Ubuntu 22.04 in ap-northeast-1
  --instance-type t3.large \
  --key-name <YOUR_KEY_NAME> \
  --subnet-id <PRIVATE_SUBNET_ID> \
  --security-group-ids <NODE_SECURITY_GROUP_ID> \
  --block-device-mappings '[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":80,"VolumeType":"gp3"}}]' \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=github-runner},{Key=project,Value=internal-wallet}]' \
  --iam-instance-profile Name=nodes.k8s.zinkapi.com
```

### 2. SSH to Instance (via Bastion)

```bash
# Get bastion IP
BASTION_IP=$(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=bastion.k8s.zinkapi.com" \
  --query 'Reservations[].Instances[].PublicIpAddress' \
  --output text)

# Get runner private IP
RUNNER_IP=$(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=github-runner" \
  --query 'Reservations[].Instances[].PrivateIpAddress' \
  --output text)

# SSH through bastion
ssh -J ubuntu@$BASTION_IP ubuntu@$RUNNER_IP
```

### 3. Run Setup Script

```bash
# On the runner instance
git clone https://github.com/97-web3/Internal-Wallet.git
cd Internal-Wallet/infra/runner

# Set GitHub token (create at https://github.com/settings/tokens with 'repo' scope)
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"

# Run setup
sudo ./setup-runner.sh
```

## IAM Configuration

### GitHub Actions OIDC

The runner uses OIDC to assume AWS IAM roles without storing credentials.

**安全建议（已反映在本仓库的 trust policy 模板中）：**
- 仅允许 `ref:refs/heads/production` 获取 AWS 身份（避免 PR/临时分支拿到生产权限）。
- 进一步限制 `job_workflow_ref` 为 `.../.github/workflows/*@refs/heads/production`，确保只有仓库内的 workflow 文件在 production 分支上才能 AssumeRole。

#### 1. Create OIDC Provider (one-time)

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

#### 2. Create IAM Roles

**Role: github-actions-kops** (for cluster management)

```bash
# Create role with trust policy
aws iam create-role \
  --role-name github-actions-kops \
  --assume-role-policy-document file://iam-trust-policy.json

# Attach policy
aws iam put-role-policy \
  --role-name github-actions-kops \
  --policy-name kops-access \
  --policy-document file://iam-kops-policy.json
```

**Role: github-actions-ecr** (for image push)

```bash
# Create role with trust policy
aws iam create-role \
  --role-name github-actions-ecr \
  --assume-role-policy-document file://iam-trust-policy.json

# Attach policy
aws iam put-role-policy \
  --role-name github-actions-ecr \
  --policy-name ecr-access \
  --policy-document file://iam-ecr-policy.json
```

**Role: github-actions-s3** (for publishing swagger artifacts)

```bash
# Create role with trust policy (Internal-Wallet repo only)
aws iam create-role \
  --role-name github-actions-s3 \
  --assume-role-policy-document file://iam-trust-policy.json

# Attach policy (S3 write to s3://internal-wallet-artifacts-301918028034/swagger/*)
aws iam put-role-policy \
  --role-name github-actions-s3 \
  --policy-name s3-artifacts-publish \
  --policy-document file://iam-s3-artifacts-publish-policy.json
```

**Role: github-actions-admin-web** (for admin-web repo: read swagger artifacts + push image)

```bash
# Create role with trust policy (Internal-Wallet-Admin-Web repo only)
aws iam create-role \
  --role-name github-actions-admin-web \
  --assume-role-policy-document file://iam-trust-policy-admin-web.json

# Attach ECR policy (push internal-wallet-admin-web)
aws iam put-role-policy \
  --role-name github-actions-admin-web \
  --policy-name ecr-admin-web \
  --policy-document file://iam-ecr-admin-web-policy.json

# Attach S3 policy (read s3://internal-wallet-artifacts-301918028034/swagger/*)
aws iam put-role-policy \
  --role-name github-actions-admin-web \
  --policy-name s3-artifacts-read \
  --policy-document file://iam-s3-artifacts-read-policy.json
```

## Workflow Usage

Workflows use the `self-hosted` runner with OIDC authentication:

```yaml
jobs:
  deploy:
    runs-on: self-hosted
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4

      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::301918028034:role/github-actions-ecr
          aws-region: ap-northeast-1

      - name: Login to ECR
        run: |
          aws ecr get-login-password | docker login --username AWS --password-stdin \
            301918028034.dkr.ecr.ap-northeast-1.amazonaws.com
```

## Management

### View Runner Status

```bash
# On the runner instance
sudo systemctl status actions.runner.97-web3-Internal-Wallet.*

# View logs
sudo journalctl -u actions.runner.97-web3-Internal-Wallet.* -f
```

### Restart Runner

```bash
sudo systemctl restart actions.runner.97-web3-Internal-Wallet.*
```

### Update Runner

```bash
# Stop service
sudo systemctl stop actions.runner.97-web3-Internal-Wallet.*

# Download new version
cd /opt/actions-runner
RUNNER_VERSION="2.312.0"  # Update to latest
curl -o actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz -L \
  https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz

# Extract and restart
tar xzf actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz
sudo systemctl start actions.runner.97-web3-Internal-Wallet.*
```

### Remove Runner

```bash
# On the runner instance
cd /opt/actions-runner
sudo ./svc.sh stop
sudo ./svc.sh uninstall
./config.sh remove --token <REMOVAL_TOKEN>
```

Get removal token from: `https://github.com/97-web3/Internal-Wallet/settings/actions/runners`

## Security Considerations

1. **No long-term credentials**: Uses OIDC for AWS access
2. **Private subnet**: Runner not directly accessible from internet
3. **Limited IAM scope**: Separate roles for kops vs ECR
4. **Bastion access only**: SSH via jump host

## Files

```
infra/runner/
├── README.md              # This file
├── setup-runner.sh        # Runner installation script
├── iam-trust-policy.json  # GitHub OIDC trust policy
├── iam-kops-policy.json   # kOps permissions
└── iam-ecr-policy.json    # ECR permissions
```

## Troubleshooting

### Runner offline

```bash
# Check service status
sudo systemctl status actions.runner.*

# Check network connectivity to GitHub
curl -s https://api.github.com/zen

# Check logs
sudo journalctl -u actions.runner.* -n 100
```

### Cannot push to ECR

```bash
# Test ECR login
aws ecr get-login-password --region ap-northeast-1 | \
  docker login --username AWS --password-stdin \
  301918028034.dkr.ecr.ap-northeast-1.amazonaws.com

# Check IAM role
aws sts get-caller-identity
```

### Cannot access K8s API

```bash
# Test kubectl
kubectl get nodes

# Check kubeconfig
cat ~/.kube/config

# Export kubeconfig from kops
kops export kubeconfig --name k8s.zinkapi.com --admin
```
