// Buildx bake definition for wallet-biz service images.
//
// Required env vars (set by workflow):
// - ECR_REGISTRY: e.g. 3019....dkr.ecr.ap-northeast-1.amazonaws.com
// - SHA: git sha to tag images with
// - CACHE_REF: registry cache ref, e.g. ${ECR_REGISTRY}/internal-wallet/buildcache:wallet-biz

variable "ECR_REGISTRY" {
  default = ""
}

variable "SHA" {
  default = ""
}

variable "CACHE_REF" {
  default = ""
}

group "default" {
  targets = [
    "api-gateway",
    "admin-rpc",
    "business-rpc",
    "signer-rpc",
    "chainrpc",
    "chainsync",
    "swap-rpc",
    "accounting-rpc",
    "market",
    "consolidation",
  ]
}

target "common" {
  context    = "."
  dockerfile = "infra/docker/wallet-biz.Dockerfile"
  platforms  = ["linux/amd64"]

  cache-from = [
    "type=registry,ref=${CACHE_REF}",
  ]
  cache-to = [
    "type=registry,ref=${CACHE_REF},mode=max",
  ]
}

target "api-gateway" {
  inherits = ["common"]
  target   = "api-gateway"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/api-gateway:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/api-gateway:production",
  ]
}

target "admin-rpc" {
  inherits = ["common"]
  target   = "admin-rpc"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/admin-rpc:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/admin-rpc:production",
  ]
}

target "business-rpc" {
  inherits = ["common"]
  target   = "business-rpc"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/business-rpc:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/business-rpc:production",
  ]
}

target "signer-rpc" {
  inherits = ["common"]
  target   = "signer-rpc"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/signer-rpc:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/signer-rpc:production",
  ]
}

target "chainrpc" {
  inherits = ["common"]
  target   = "chainrpc"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/chainrpc:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/chainrpc:production",
  ]
}

target "chainsync" {
  inherits = ["common"]
  target   = "chainsync"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/chainsync:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/chainsync:production",
  ]
}

target "swap-rpc" {
  inherits = ["common"]
  target   = "swap-rpc"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/swap-rpc:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/swap-rpc:production",
  ]
}

target "accounting-rpc" {
  inherits = ["common"]
  target   = "accounting-rpc"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/accounting-rpc:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/accounting-rpc:production",
  ]
}

target "market" {
  inherits = ["common"]
  target   = "market"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/market:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/market:production",
  ]
}

target "consolidation" {
  inherits = ["common"]
  target   = "consolidation"
  tags = [
    "${ECR_REGISTRY}/internal-wallet/consolidation:${SHA}",
    "${ECR_REGISTRY}/internal-wallet/consolidation:production",
  ]
}

