#!/usr/bin/env bash
# Generate IP-based NetworkPolicies from egress-inventory.json
# Usage: ./scripts/network/generate-ip-based-policies.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
INVENTORY_FILE="$REPO_ROOT/docs/egress-inventory.json"
OUTPUT_DIR="$REPO_ROOT/deploy/k8s/base/network/generated"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Generating IP-based NetworkPolicies ===${NC}"

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}Error: jq is required but not installed${NC}"
    exit 1
fi

# Check if inventory file exists
if [[ ! -f "$INVENTORY_FILE" ]]; then
    echo -e "${RED}Error: Inventory file not found: $INVENTORY_FILE${NC}"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Function to resolve hostname to IPs
resolve_host() {
    local host="$1"
    # Use dig to resolve (more reliable than host/nslookup)
    dig +short "$host" A | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || true
}

# Function to generate CIDR from IP
ip_to_cidr() {
    local ip="$1"
    echo "${ip}/32"
}

echo -e "${YELLOW}Resolving hostnames to IPs...${NC}"

# Example: Generate policy for blockchain RPC endpoints
cat > "$OUTPUT_DIR/wallet-biz-allow-blockchain-rpc-ips.yaml" <<'EOF'
# Auto-generated from egress-inventory.json
# WARNING: IPs may change. Monitor and update regularly.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-blockchain-rpc-ips
  namespace: wallet-biz
  annotations:
    generated-at: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    generated-from: "docs/egress-inventory.json"
spec:
  podSelector:
    matchExpressions:
      - key: app.kubernetes.io/name
        operator: In
        values:
          - chainrpc
          - chainsync
  policyTypes:
    - Egress
  egress:
EOF

# Resolve and add IPs for each blockchain RPC host
echo "    # Ethereum RPC (eth.llamarpc.com)" >> "$OUTPUT_DIR/wallet-biz-allow-blockchain-rpc-ips.yaml"
for ip in $(resolve_host "eth.llamarpc.com"); do
    cat >> "$OUTPUT_DIR/wallet-biz-allow-blockchain-rpc-ips.yaml" <<EOF
    - to:
        - ipBlock:
            cidr: $(ip_to_cidr "$ip")
      ports:
        - protocol: TCP
          port: 443
EOF
done

echo -e "${GREEN}✓ Generated: $OUTPUT_DIR/wallet-biz-allow-blockchain-rpc-ips.yaml${NC}"
echo -e "${YELLOW}Note: This is a basic example. Full implementation would parse egress-inventory.json${NC}"
echo -e "${YELLOW}and generate policies for all services.${NC}"

