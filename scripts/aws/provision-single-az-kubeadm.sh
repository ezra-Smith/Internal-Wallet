#!/usr/bin/env bash
set -euo pipefail

# Provisions the minimal AWS foundation for the Internal-Wallet single-AZ kubeadm dev/staging cluster.
# Region/AZ target: ap-northeast-1 / ap-northeast-1a by default.
#
# This script intentionally keeps things simple:
# - 1 VPC + 1 public subnet + IGW + route table
# - 1 SG for node-to-node traffic + SSH
# - 1 SG for ingress NodePorts (30080/30443) attached only to the infra worker
# - 3 EC2 instances (control-plane, business worker, infra worker)
#
# Read first:
# - docs/kubernetes/01-aws-foundation.md
# - docs/kubernetes/13-bootstrap-runbook.md

if [[ "${WALLET_AWS_CONFIRM:-}" != "yes" ]]; then
  cat <<'EOF' >&2
Refusing to run without confirmation.

This script creates AWS resources that cost money.
Review variables below, then rerun with:
  WALLET_AWS_CONFIRM=yes KEY_NAME=... SSH_CIDR=... ./scripts/aws/provision-single-az-kubeadm.sh
EOF
  exit 2
fi

need() {
  command -v "$1" >/dev/null 2>&1 || { echo "Missing required command: $1" >&2; exit 1; }
}

need aws
need jq

REGION="${REGION:-ap-northeast-1}"
AZ="${AZ:-ap-northeast-1a}"
PREFIX="${PREFIX:-wallet-dev}"

VPC_CIDR="${VPC_CIDR:-10.0.0.0/16}"
SUBNET_CIDR="${SUBNET_CIDR:-10.0.0.0/24}"

# Strongly recommended to set this to your office/VPN egress IP, e.g. "203.0.113.10/32".
SSH_CIDR="${SSH_CIDR:?Set SSH_CIDR (recommended: <your-ip>/32)}"

# Key pair must exist in the target region.
KEY_NAME="${KEY_NAME:?Set KEY_NAME (existing EC2 key pair name in ${REGION})}"

# Optional: pin AMI directly; otherwise resolves latest Ubuntu 22.04 via SSM parameter.
AMI_ID="${AMI_ID:-}"

INSTANCE_TYPE_CP="${INSTANCE_TYPE_CP:-t3.medium}"
INSTANCE_TYPE_BIZ="${INSTANCE_TYPE_BIZ:-m6i.large}"
INSTANCE_TYPE_INFRA="${INSTANCE_TYPE_INFRA:-m6i.xlarge}"

DISK_GIB_CP="${DISK_GIB_CP:-50}"
DISK_GIB_BIZ="${DISK_GIB_BIZ:-80}"
DISK_GIB_INFRA="${DISK_GIB_INFRA:-120}"

# By default, only allow ingress NodePorts from the same CIDR as SSH. Override if needed.
INGRESS_CIDR="${INGRESS_CIDR:-$SSH_CIDR}"

# Optional: allocate and associate an Elastic IP to the infra node (recommended if you want stable DNS).
ALLOCATE_EIP_INFRA="${ALLOCATE_EIP_INFRA:-false}"

aws_json() {
  aws --region "$REGION" "$@" --output json
}

echo "[1/7] Validating AWS identity/region..."
aws --region "$REGION" sts get-caller-identity >/dev/null

if [[ -z "$AMI_ID" ]]; then
  echo "[2/7] Resolving Ubuntu 22.04 AMI via SSM..."
  AMI_ID="$(
    aws_json ssm get-parameter \
      --name /aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp3/ami-id \
      | jq -r .Parameter.Value
  )"
fi

echo "[3/7] Creating VPC/subnet/IGW/routes..."
VPC_ID="$(aws_json ec2 create-vpc --cidr-block "$VPC_CIDR" --tag-specifications "ResourceType=vpc,Tags=[{Key=Name,Value=${PREFIX}-vpc}]" | jq -r .Vpc.VpcId)"
aws_json ec2 modify-vpc-attribute --vpc-id "$VPC_ID" --enable-dns-support >/dev/null
aws_json ec2 modify-vpc-attribute --vpc-id "$VPC_ID" --enable-dns-hostnames >/dev/null

SUBNET_ID="$(aws_json ec2 create-subnet --vpc-id "$VPC_ID" --cidr-block "$SUBNET_CIDR" --availability-zone "$AZ" --tag-specifications "ResourceType=subnet,Tags=[{Key=Name,Value=${PREFIX}-public-${AZ}}]" | jq -r .Subnet.SubnetId)"
aws_json ec2 modify-subnet-attribute --subnet-id "$SUBNET_ID" --map-public-ip-on-launch >/dev/null

IGW_ID="$(aws_json ec2 create-internet-gateway --tag-specifications "ResourceType=internet-gateway,Tags=[{Key=Name,Value=${PREFIX}-igw}]" | jq -r .InternetGateway.InternetGatewayId)"
aws_json ec2 attach-internet-gateway --vpc-id "$VPC_ID" --internet-gateway-id "$IGW_ID" >/dev/null

RTB_ID="$(aws_json ec2 create-route-table --vpc-id "$VPC_ID" --tag-specifications "ResourceType=route-table,Tags=[{Key=Name,Value=${PREFIX}-public-rtb}]" | jq -r .RouteTable.RouteTableId)"
aws_json ec2 create-route --route-table-id "$RTB_ID" --destination-cidr-block 0.0.0.0/0 --gateway-id "$IGW_ID" >/dev/null
aws_json ec2 associate-route-table --subnet-id "$SUBNET_ID" --route-table-id "$RTB_ID" >/dev/null

echo "[4/7] Creating Security Groups..."
SG_NODES_ID="$(aws_json ec2 create-security-group --group-name "${PREFIX}-nodes" --description "Internal-Wallet kubeadm nodes (intra-cluster + ssh)" --vpc-id "$VPC_ID" | jq -r .GroupId)"
aws_json ec2 create-tags --resources "$SG_NODES_ID" --tags "Key=Name,Value=${PREFIX}-nodes" >/dev/null

# Intra-cluster: simplest safe option for dev/staging is to allow all traffic within the same SG.
aws_json ec2 authorize-security-group-ingress --group-id "$SG_NODES_ID" --protocol -1 --source-group "$SG_NODES_ID" >/dev/null

# SSH (restrict to your office/VPN).
aws_json ec2 authorize-security-group-ingress --group-id "$SG_NODES_ID" --protocol tcp --port 22 --cidr "$SSH_CIDR" >/dev/null

SG_INGRESS_ID="$(aws_json ec2 create-security-group --group-name "${PREFIX}-ingress" --description "Internal-Wallet ingress NodePorts (30080/30443)" --vpc-id "$VPC_ID" | jq -r .GroupId)"
aws_json ec2 create-tags --resources "$SG_INGRESS_ID" --tags "Key=Name,Value=${PREFIX}-ingress" >/dev/null
aws_json ec2 authorize-security-group-ingress --group-id "$SG_INGRESS_ID" --protocol tcp --port 30080 --cidr "$INGRESS_CIDR" >/dev/null
aws_json ec2 authorize-security-group-ingress --group-id "$SG_INGRESS_ID" --protocol tcp --port 30443 --cidr "$INGRESS_CIDR" >/dev/null

echo "[5/7] Launching EC2 instances..."

BLOCK_CP="[{\"DeviceName\":\"/dev/sda1\",\"Ebs\":{\"VolumeSize\":${DISK_GIB_CP},\"VolumeType\":\"gp3\",\"DeleteOnTermination\":true}}]"
BLOCK_BIZ="[{\"DeviceName\":\"/dev/sda1\",\"Ebs\":{\"VolumeSize\":${DISK_GIB_BIZ},\"VolumeType\":\"gp3\",\"DeleteOnTermination\":true}}]"
BLOCK_INFRA="[{\"DeviceName\":\"/dev/sda1\",\"Ebs\":{\"VolumeSize\":${DISK_GIB_INFRA},\"VolumeType\":\"gp3\",\"DeleteOnTermination\":true}}]"

CP_ID="$(aws_json ec2 run-instances \
  --image-id "$AMI_ID" \
  --instance-type "$INSTANCE_TYPE_CP" \
  --key-name "$KEY_NAME" \
  --subnet-id "$SUBNET_ID" \
  --security-group-ids "$SG_NODES_ID" \
  --block-device-mappings "$BLOCK_CP" \
  --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=${PREFIX}-cp-1},{Key=wallet,Value=internal-wallet},{Key=role,Value=control-plane}]" \
  --count 1 | jq -r .Instances[0].InstanceId)"

BIZ_ID="$(aws_json ec2 run-instances \
  --image-id "$AMI_ID" \
  --instance-type "$INSTANCE_TYPE_BIZ" \
  --key-name "$KEY_NAME" \
  --subnet-id "$SUBNET_ID" \
  --security-group-ids "$SG_NODES_ID" \
  --block-device-mappings "$BLOCK_BIZ" \
  --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=${PREFIX}-worker-business-1},{Key=wallet,Value=internal-wallet},{Key=role,Value=business}]" \
  --count 1 | jq -r .Instances[0].InstanceId)"

INFRA_ID="$(aws_json ec2 run-instances \
  --image-id "$AMI_ID" \
  --instance-type "$INSTANCE_TYPE_INFRA" \
  --key-name "$KEY_NAME" \
  --subnet-id "$SUBNET_ID" \
  --security-group-ids "$SG_NODES_ID" "$SG_INGRESS_ID" \
  --block-device-mappings "$BLOCK_INFRA" \
  --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=${PREFIX}-worker-infra-1},{Key=wallet,Value=internal-wallet},{Key=role,Value=infrastructure}]" \
  --count 1 | jq -r .Instances[0].InstanceId)"

aws --region "$REGION" ec2 wait instance-running --instance-ids "$CP_ID" "$BIZ_ID" "$INFRA_ID"

echo "[6/7] Fetching public/private IPs..."
IPS="$(aws_json ec2 describe-instances --instance-ids "$CP_ID" "$BIZ_ID" "$INFRA_ID" | jq -r '.Reservations[].Instances[] | [.Tags[]? | select(.Key=="Name").Value, .InstanceId, .PrivateIpAddress, (.PublicIpAddress // "")] | @tsv' | sort)"

echo ""
echo "Instances (Name / InstanceId / PrivateIP / PublicIP):"
echo "$IPS" | column -t
echo ""

if [[ "$ALLOCATE_EIP_INFRA" == "true" ]]; then
  echo "[7/7] Allocating + associating EIP to infra node..."
  ALLOC_ID="$(aws_json ec2 allocate-address --domain vpc | jq -r .AllocationId)"
  aws_json ec2 associate-address --instance-id "$INFRA_ID" --allocation-id "$ALLOC_ID" >/dev/null
  INFRA_EIP="$(aws_json ec2 describe-addresses --allocation-ids "$ALLOC_ID" | jq -r .Addresses[0].PublicIp)"
  echo "Infra EIP: $INFRA_EIP (allocation-id: $ALLOC_ID)"
fi

cat <<EOF

Next steps:
1) SSH to the control-plane and install kubeadm + containerd (see docs/kubernetes/02-kops-cluster.md).
2) Install Calico CNI, then join both workers.
3) Label/taint nodes (see docs/kubernetes/13-bootstrap-runbook.md Phase 2).
4) Create required Secrets (wallet-tls, wallet-biz-secrets, registry creds) and apply Kustomize overlay.

Helpful links:
- docs/kubernetes/13-bootstrap-runbook.md
- deploy/k8s/README.md

Resource IDs (for cleanup):
  VPC_ID=$VPC_ID
  SUBNET_ID=$SUBNET_ID
  IGW_ID=$IGW_ID
  RTB_ID=$RTB_ID
  SG_NODES_ID=$SG_NODES_ID
  SG_INGRESS_ID=$SG_INGRESS_ID
  CP_ID=$CP_ID
  BIZ_ID=$BIZ_ID
  INFRA_ID=$INFRA_ID
EOF
