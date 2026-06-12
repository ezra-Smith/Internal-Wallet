# kOps Cluster Configuration

This directory contains the kOps cluster definition for the Internal-Wallet Kubernetes cluster.

## Architecture

- **Region**: ap-northeast-1 (Tokyo)
- **Availability Zone**: ap-northeast-1a (single-AZ)
- **Topology**: Private (nodes in private subnets, NAT gateway for egress)
- **CNI**: Calico
- **API Access**: Internal NLB (no public exposure)

## Prerequisites

1. **AWS CLI** configured with appropriate permissions
2. **kops** CLI installed (v1.29+)
3. **kubectl** CLI installed
4. **SSH key pair** for node access
5. **Route53 hosted zone** delegated from Cloudflare

### Required AWS Permissions

The IAM user/role running kops needs permissions for:
- EC2 (instances, volumes, security groups, VPCs)
- S3 (state store bucket)
- Route53 (DNS records)
- IAM (instance profiles, roles)
- ELB (load balancers)
- ASG (auto scaling groups)

## Quick Start

### 1. Configure Environment

```bash
cd infra/kops
cp env.example.sh env.sh
# Edit env.sh with your values
source env.sh
```

### 2. Bootstrap AWS Resources

```bash
./scripts/bootstrap-aws.sh
```

This creates:
- S3 bucket for kOps state store (versioning + encryption + no public access)
- S3 bucket for IRSA OIDC discovery docs (versioning + encryption + policy-based public read)
- Route53 hosted zone (if not exists)
- GitHub OIDC provider (for CI/CD)

### 3. Configure DNS Delegation

After running bootstrap, add the NS records to Cloudflare:

1. Go to Cloudflare Dashboard → DNS
2. Add NS records for `k8s` subdomain pointing to Route53 nameservers
3. Wait for DNS propagation (~5-10 minutes)

### 4. Create Cluster

```bash
./scripts/create-cluster.sh

# Review the output, then apply:
kops update cluster --name $KOPS_CLUSTER_NAME --yes --admin

# Wait for cluster to be ready:
kops validate cluster --name $KOPS_CLUSTER_NAME --wait 15m
```

### 5. Access Cluster

```bash
# Export kubeconfig
kops export kubeconfig --name $KOPS_CLUSTER_NAME --admin

# Verify
kubectl get nodes
```

## Cluster Nodes

| Node Type | Instance | vCPU | Memory | Disk | Purpose |
|-----------|----------|------|--------|------|---------|
| Master | m6i.large | 2 | 8 GB | 50GB gp3 | Control plane |
| Business | m6i.2xlarge | 8 | 32 GB | 300GB gp3 | Go microservices |
| Infra | r6i.xlarge | 4 | 32 GB | 500GB gp3 | DB, MQ, monitoring |
| Bastion | t3.micro | 1 | 1 GB | 20GB gp3 | SSH jump host |

## Common Operations

### Update Cluster Configuration

```bash
# Edit cluster.yaml, then:
./scripts/update-cluster.sh

# Review changes, then apply:
./scripts/update-cluster.sh --yes

# If nodes need recreation:
./scripts/update-cluster.sh --yes --rolling-update
```

### SSH to Nodes

```bash
# Get bastion public IP
aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=bastion.k8s.zinkapi.com" \
  --query 'Reservations[].Instances[].PublicIpAddress' \
  --output text

# SSH through bastion
ssh -A ubuntu@<BASTION_IP>

# From bastion, SSH to nodes (use private IPs from `kubectl get nodes -o wide`)
ssh ubuntu@<NODE_PRIVATE_IP>
```

### View Cluster State

```bash
# Show cluster configuration
kops get cluster --name $KOPS_CLUSTER_NAME -o yaml

# Show instance groups
kops get instancegroups --name $KOPS_CLUSTER_NAME

# Validate cluster health
kops validate cluster --name $KOPS_CLUSTER_NAME
```

### Delete Cluster

```bash
# Preview deletion
./scripts/delete-cluster.sh

# Confirm deletion (DESTRUCTIVE!)
./scripts/delete-cluster.sh --yes
```

## GitOps Workflow

This cluster is managed via GitOps:

1. **PR to `infra/kops/**`**: Triggers `kops-diff.yml` workflow
   - Shows diff between repo and current cluster state
   - No changes applied (read-only)

2. **Merge to `main`**: Triggers `kops-apply.yml` workflow
   - Syncs cluster.yaml to state store
   - Applies changes to AWS
   - Validates cluster health

## GitHub Actions OIDC

CI/CD uses OIDC authentication (no long-term credentials):

### Required IAM Role Trust Policy

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::301918028034:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:97-web3/Internal-Wallet:*"
        }
      }
    }
  ]
}
```

See `../runner/` for complete IAM policy definitions.

## Troubleshooting

### Cluster validation fails

```bash
# Check node status
kubectl get nodes

# Check system pods
kubectl get pods -n kube-system

# View kops events
kops toolbox dump --name $KOPS_CLUSTER_NAME
```

### Cannot reach API server

1. Ensure you're on the VPC or connected via bastion
2. Check security group rules
3. Verify internal NLB is healthy

### Node not joining cluster

```bash
# SSH to node via bastion
ssh -J ubuntu@<BASTION_IP> ubuntu@<NODE_IP>

# Check kubelet logs
journalctl -u kubelet -f

# Check cloud-init logs
cat /var/log/cloud-init-output.log
```

## Security Considerations

- **API Server**: Internal only, no public exposure
- **Bastion**: Only SSH access point, restrict source IPs in env.sh
- **Nodes**: Private subnets, egress via NAT gateway
- **etcd**: Encrypted volumes
- **State Store**: S3 versioning + encryption + no public access

## Cost Estimation

| Resource | Monthly Cost (USD) |
|----------|-------------------|
| EC2 Instances | ~$620 |
| EBS Storage (950GB) | ~$76 |
| NAT Gateway | ~$45 |
| Route53 | ~$1 |
| **Total** | **~$745** |

## Files

```
infra/kops/
├── README.md           # This file
├── env.example.sh      # Environment variables template
├── cluster.yaml        # kOps Cluster + InstanceGroup definitions
└── scripts/
    ├── bootstrap-aws.sh    # Create S3 + Route53 + OIDC
    ├── create-cluster.sh   # Create new cluster
    ├── update-cluster.sh   # Update existing cluster
    └── delete-cluster.sh   # Delete cluster
```
