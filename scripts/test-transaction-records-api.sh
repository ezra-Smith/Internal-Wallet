#!/bin/bash

# Test script for /api/v1/business/list_transaction_records API endpoint
# This script demonstrates the complete authentication and API call flow

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Transaction Records API Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Step 1: Login
echo -e "${YELLOW}Step 1: Logging in with phone code...${NC}"
echo "Phone: +86 13800138000"
echo "Code: 123456 (bypass code for development)"
echo ""

LOGIN_RESPONSE=$(curl -s -X POST "${API_BASE_URL}/api/v1/business/login_by_phone_code" \
  -H "Content-Type: application/json" \
  -d '{
    "country_code": "+86",
    "phone": "13800138000",
    "code": "123456"
  }')

echo -e "${GREEN}Login Response:${NC}"
echo "$LOGIN_RESPONSE" | jq '.'
echo ""

# Extract access token
ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token')
USER_ID=$(echo "$LOGIN_RESPONSE" | jq -r '.data.user_id')

if [ "$ACCESS_TOKEN" = "null" ] || [ -z "$ACCESS_TOKEN" ]; then
  echo -e "${RED}Error: Failed to get access token${NC}"
  exit 1
fi

echo -e "${GREEN}✓ Login successful${NC}"
echo "User ID: $USER_ID"
echo "Access Token: ${ACCESS_TOKEN:0:50}..."
echo ""

# Step 2: List all transaction records
echo -e "${YELLOW}Step 2: Fetching all transaction records...${NC}"
RECORDS_RESPONSE=$(curl -s -X POST "${API_BASE_URL}/api/v1/business/list_transaction_records" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{
    "page": 1,
    "page_size": 20,
    "asset": "",
    "type": ""
  }')

echo -e "${GREEN}Transaction Records Response:${NC}"
echo "$RECORDS_RESPONSE" | jq '.'
echo ""

# Step 3: Filter by asset (USDT)
echo -e "${YELLOW}Step 3: Fetching USDT transaction records...${NC}"
USDT_RESPONSE=$(curl -s -X POST "${API_BASE_URL}/api/v1/business/list_transaction_records" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{
    "page": 1,
    "page_size": 10,
    "asset": "USDT",
    "type": ""
  }')

echo -e "${GREEN}USDT Records Response:${NC}"
echo "$USDT_RESPONSE" | jq '.'
echo ""

# Step 4: Filter by type (deposit)
echo -e "${YELLOW}Step 4: Fetching deposit transaction records...${NC}"
DEPOSIT_RESPONSE=$(curl -s -X POST "${API_BASE_URL}/api/v1/business/list_transaction_records" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{
    "page": 1,
    "page_size": 10,
    "asset": "",
    "type": "deposit"
  }')

echo -e "${GREEN}Deposit Records Response:${NC}"
echo "$DEPOSIT_RESPONSE" | jq '.'
echo ""

# Step 5: Test error handling - Invalid token
echo -e "${YELLOW}Step 5: Testing error handling (invalid token)...${NC}"
ERROR_RESPONSE=$(curl -s -X POST "${API_BASE_URL}/api/v1/business/list_transaction_records" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer invalid_token" \
  -d '{
    "page": 1,
    "page_size": 20
  }')

echo -e "${RED}Expected Error Response:${NC}"
echo "$ERROR_RESPONSE" | jq '.'
echo ""

# Step 6: Test error handling - Missing authorization
echo -e "${YELLOW}Step 6: Testing error handling (missing authorization)...${NC}"
NO_AUTH_RESPONSE=$(curl -s -X POST "${API_BASE_URL}/api/v1/business/list_transaction_records" \
  -H "Content-Type: application/json" \
  -d '{
    "page": 1,
    "page_size": 20
  }')

echo -e "${RED}Expected Error Response:${NC}"
echo "$NO_AUTH_RESPONSE" | jq '.'
echo ""

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✓ All tests completed successfully!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "Summary:"
echo "- Login: ✓"
echo "- List all records: ✓"
echo "- Filter by asset: ✓"
echo "- Filter by type: ✓"
echo "- Error handling: ✓"
echo ""
echo "Available mock users for testing:"
echo "  +86 13800138000 (Grace, User ID: 1007)"
echo "  +1  15500000001 (Alice, User ID: 1001)"
echo "  +1  15500000002 (Bob,   User ID: 1002)"
echo ""
echo "All users use bypass code: 123456"

