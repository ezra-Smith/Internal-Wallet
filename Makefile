.PHONY: help proto swagger docker-up docker-down build-rpc build-gateway build-all run-business run-chainrpc run-chainsync run-signer run-swap run-admin run-accounting run-market run-market-alert run-gateway run-all stop-all lint clean test test-accounting test-market test-market-alert init-db

help:
	@echo "=========================================="
	@echo "Zink Wallet System - Makefile Commands"
	@echo "=========================================="
	@echo ""
	@echo "Infrastructure:"
	@echo "  make docker-up       - Start all infrastructure services (MariaDB, Redis, ETCD, Kafka, etc.)"
	@echo "  make docker-down     - Stop all infrastructure services"
	@echo "  make init-db         - Initialize database schema"
	@echo ""
	@echo "Code Generation:"
	@echo "  make proto           - Generate protobuf files for all services"
	@echo "  make proto-business  - Generate protobuf for business service"
	@echo "  make proto-chainrpc  - Generate protobuf for chainrpc service"
	@echo "  make proto-chainsync - Generate protobuf for chainsync service"
	@echo "  make proto-signer    - Generate protobuf for signer service"
	@echo "  make proto-swap      - Generate protobuf for swap service"
	@echo "  make proto-admin     - Generate protobuf for admin service"
	@echo "  make proto-accounting - Generate protobuf for accounting service"
	@echo "  make swagger         - Sync Swagger/OpenAPI artifacts from api-gateway/routes.yaml"
	@echo "  make gen-swagger     - Generate all Swagger docs (business + admin)"
	@echo "  make gen-swagger-business - Generate Swagger docs for business service only"
	@echo "  make gen-swagger-admin    - Generate Swagger docs for admin service only"
	@echo ""
	@echo "Build:"
	@echo "  make build-rpc       - Build all RPC services"
	@echo "  make build-gateway   - Build API gateway"
	@echo "  make build-all       - Build all services (RPC + gateway)"
	@echo ""
	@echo "Run Services (RPC services MUST start before gateway):"
	@echo "  make run-business    - Run business RPC service"
	@echo "  make run-chainrpc    - Run chainrpc RPC service"
	@echo "  make run-chainsync   - Run chainsync RPC service"
	@echo "  make run-signer      - Run signer RPC service"
	@echo "  make run-admin       - Run admin RPC service"
	@echo "  make run-accounting  - Run accounting RPC service"
	@echo "  make run-market      - Run market service"
	@echo "  make run-market-alert - Run market alert service"
	@echo "  make run-gateway     - Run API gateway (start RPC services first!)"
	@echo "  make run-all         - Run all services in separate windows"
	@echo "  make stop-all        - Stop all running services"
	@echo ""
	@echo "Testing:"
	@echo "  make test            - Run all tests"
	@echo "  make test-business   - Run business service tests"
	@echo "  make test-chainrpc   - Run chainrpc service tests"
	@echo "  make test-signer     - Run signer service tests"
	@echo "  make test-admin      - Run admin service tests"
	@echo "  make test-accounting - Run accounting service tests"
	@echo "  make test-market     - Run market service tests"
	@echo "  make test-market-alert - Run market alert service tests"
	@echo ""
	@echo "Docker Images:"
	@echo "  make docker-build           - Build all Docker images"
	@echo "  make docker-build-service SERVICE=<name> - Build specific service image"
	@echo "  make docker-build-gateway   - Build API Gateway image"
	@echo ""
	@echo "Test Environment (Docker Compose):"
	@echo "  make test-env-deploy        - Deploy full test environment (all services)"
	@echo "  make test-env-infra         - Deploy only infrastructure (for local dev)"
	@echo "  make test-env-status        - Check test environment status"
	@echo "  make test-env-logs SERVICE=<name> - View test environment logs"
	@echo "  make test-env-cleanup       - Clean up test environment"
	@echo ""
	@echo "Kubernetes Deployment:"
	@echo "  make k8s-deploy             - Deploy all resources to Kubernetes"
	@echo "  make k8s-deploy-namespace   - Create Kubernetes namespace"
	@echo "  make k8s-deploy-infrastructure - Deploy infrastructure (DB, Redis, etc.)"
	@echo "  make k8s-deploy-services    - Deploy application services"
	@echo "  make k8s-status             - Check deployment status"
	@echo "  make k8s-logs SERVICE=<name> - View service logs"
	@echo "  make k8s-shell SERVICE=<name> - Open shell in service pod"
	@echo "  make k8s-cleanup            - Clean up all Kubernetes resources"
	@echo ""
	@echo "Quality & Maintenance:"
	@echo "  make lint            - Run golangci-lint"
	@echo "  make clean           - Clean build artifacts"
	@echo ""

# ============================================
# Infrastructure Management
# ============================================

docker-up:
	@echo "Starting infrastructure services (MariaDB, Redis, ETCD, Kafka, Prometheus, Grafana, Jaeger)..."
	@cd deploy/docker && docker-compose up -d
	@echo "Waiting for services to be ready..."
	@timeout /t 10 /nobreak >nul
	@echo "✓ All infrastructure services started!"
	@echo ""
	@echo "Service URLs:"
	@echo "  MariaDB:    localhost:3306 (user: crypto, password: crypto123456)"
	@echo "  Redis:      localhost:6379 (password: redis123456)"
	@echo "  ETCD:       localhost:2379"
	@echo "  Kafka:      localhost:9092"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana:    http://localhost:3000 (admin/admin123456)"
	@echo "  Jaeger UI:  http://localhost:16686"

docker-down:
	@echo "Stopping infrastructure services..."
	@cd deploy/docker && docker-compose down
	@echo "✓ All infrastructure services stopped!"

init-db:
	@echo "Initializing database schema..."
	@echo "Database init scripts are in deploy/docker/init-db/"
	@echo "They will be executed automatically when MariaDB container starts for the first time."
	@echo "To re-initialize: docker-compose down -v && docker-compose up -d"

# ============================================
# Code Generation - Protobuf
# ============================================

proto: proto-business proto-chainrpc proto-chainsync proto-signer proto-swap proto-admin proto-accounting proto-consolidation proto-notification
	@echo "✓ All protobuf files generated!"

proto-business:
	@echo "Generating protobuf for business service..."
	@cd proto && goctl rpc protoc business.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/business/rpc --style=goZero
	@echo "✓ Business service proto generated!"

proto-chainrpc:
	@echo "Generating protobuf for chainrpc service..."
	@cd proto && goctl rpc protoc chainrpc.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/chainrpc/rpc
	@echo "✓ ChainRPC service proto generated!"

proto-chainsync:
	@echo "Generating protobuf for chainsync service..."
	@cd proto && goctl rpc protoc chainsync.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/chainsync/rpc
	@echo "✓ ChainSync service proto generated!"

proto-signer:
	@echo "Generating protobuf for signer service..."
	@cd proto && goctl rpc protoc signer.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/signer/rpc
	@echo "✓ Signer service proto generated!"

proto-swap:
	@echo "Generating protobuf for swap service..."
	@cd proto && goctl rpc protoc swap.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/swap/rpc --style=goZero
	@echo "✓ Swap service proto generated!"

proto-admin:
	@echo "Generating protobuf for admin service..."
	@cd proto && goctl rpc protoc admin.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/admin/rpc --style=goZero
	@echo "✓ Admin service proto generated!"

proto-accounting:
	@echo "Generating protobuf for accounting service..."
	@cd proto && goctl rpc protoc accounting.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/accounting/rpc --style=goZero
	@echo "✓ Accounting service proto generated!"

proto-consolidation:
	@echo "Generating protobuf for consolidation service..."
	@cd proto && goctl rpc protoc consolidation.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/consolidation/rpc --style=goZero
	@echo "✓ Consolidation service proto generated!"

proto-notification:
	@echo "Generating protobuf for notification service..."
	@cd proto && goctl rpc protoc notification.proto --go_out=. --go-grpc_out=. --zrpc_out=../services/notification/rpc --style=goZero
	@echo "✓ Notification service proto generated!"

# ============================================
# Build Services
# ============================================

build-rpc:
	@echo "Building all RPC services..."
	@if not exist bin mkdir bin
	@echo "Building business service..."
	@cd services\business\rpc && go build -o ..\..\..\bin\business-rpc.exe .
	@echo "Building chainrpc service..."
	@cd services\chainrpc\rpc && go build -o ..\..\..\bin\chainrpc-rpc.exe .
	@echo "Building chainsync service..."
	@cd services\chainsync\rpc && go build -o ..\..\..\bin\chainsync-rpc.exe .
	@echo "Building signer service..."
	@cd services\signer\rpc && go build -o ..\..\..\bin\signer-rpc.exe .
	@echo "Building admin service..."
	@cd services\admin\rpc && go build -o ..\..\..\bin\admin-rpc.exe .
	@echo "Building accounting service..."
	@cd services\accounting\rpc && go build -o ..\..\..\bin\accounting-rpc.exe .
	@echo "Building market service..."
	@cd services\market\rpc && go build -o ..\..\..\bin\market.exe .
	@echo "✓ All RPC services built successfully!"
	@echo "Output: bin\business-rpc.exe, bin\chainrpc-rpc.exe, bin\chainsync-rpc.exe, bin\signer-rpc.exe, bin\admin-rpc.exe, bin\accounting-rpc.exe, bin\market.exe"

build-gateway:
	@echo "Building API gateway..."
	@if not exist bin mkdir bin
	@cd api-gateway && go build -o ..\bin\api-gateway.exe .
	@echo "✓ API gateway built successfully!"
	@echo "Output: bin\api-gateway.exe"

build-all: build-rpc build-gateway
	@echo "✓ All services built successfully!"

# ============================================
# Run Services (Development Mode)
# ============================================

run-business:
	@echo "Starting business RPC service..."
	@cd services\business\rpc && go run business.go -f etc\business.yaml

run-chainrpc:
	@echo "Starting chainrpc RPC service..."
	@cd services\chainrpc\rpc && go run chainrpc.go -f etc\chainrpc.yaml

run-chainsync:
	@echo "Starting chainsync RPC service..."
	@cd services\chainsync\rpc && go run chainsync.go -f etc\chainsync.yaml

run-signer:
	@echo "Starting signer RPC service..."
	@cd services\signer\rpc && go run signer.go -f etc\signer.yaml

run-swap:
	@echo "Starting swap RPC service..."
	@cd services\swap\rpc && go run swap.go -f etc\swap.yaml

run-admin:
	@echo "Starting admin RPC service..."
	@cd services\admin\rpc && go run admin.go -f etc\admin.yaml

run-accounting:
	@echo "Starting accounting RPC service..."
	@cd services\accounting\rpc && go run accounting.go -f etc\accounting.yaml

run-market:
	@echo "Starting market service..."
	@cd services\market\rpc && go run market.go -f etc\market.yaml

run-market-alert:
	@echo "Starting market alert service..."
	@cd services/market-alert && go run . -f etc/alert.yaml

run-gateway:
	@echo "Starting API gateway..."
	@echo "⚠️  WARNING: RPC services must be running first!"
	@timeout /t 2 /nobreak >nul
	@cd api-gateway && go run gateway.go -f etc\gateway.yaml -r routes.yaml

run-all:
	@echo "Starting all services in separate windows..."
	@echo "⚠️  Make sure infrastructure is running (make docker-up)"
	@timeout /t 2 /nobreak >nul	@start "Business RPC" cmd /k "cd services\business\rpc && go run business.go -f etc\business.yaml"
	@timeout /t 1 /nobreak >nul
	@start "ChainRPC RPC" cmd /k "cd services\chainrpc\rpc && go run chainrpc.go -f etc\chainrpc.yaml"
	@timeout /t 1 /nobreak >nul
	@start "ChainSync RPC" cmd /k "cd services\chainsync\rpc && go run chainsync.go -f etc\chainsync.yaml"
	@timeout /t 1 /nobreak >nul
	@start "Signer RPC" cmd /k "cd services\signer\rpc && go run signer.go -f etc\signer.yaml"
	@timeout /t 1 /nobreak >nul
	@start "Admin RPC" cmd /k "cd services\admin\rpc && go run admin.go -f etc\admin.yaml"
	@timeout /t 1 /nobreak >nul
	@start "Accounting RPC" cmd /k "cd services\accounting\rpc && go run accounting.go -f etc\accounting.yaml"
	@timeout /t 1 /nobreak >nul
	@start "Market Service" cmd /k "cd services\market\rpc && go run market.go -f etc\market.yaml"
	@timeout /t 1 /nobreak >nul
	@start "Market Alert Service" cmd /k "cd services\market-alert && go run . -f etc\alert.yaml"
	@timeout /t 3 /nobreak >nul
	@start "API Gateway" cmd /k "cd api-gateway && go run gateway.go -f etc\gateway.yaml -r routes.yaml"
	@echo "✓ All services started in separate windows!"
	@echo "  Close the windows to stop services"

stop-all:
	@echo "Stopping all Go services..."
	@taskkill /F /IM go.exe /T 2>nul || echo No Go processes found
	@echo "✓ All services stopped!"

# ============================================
# Testing
# ============================================

test:
	@echo "Running all tests..."
	@go test ./... -v

test-business:
	@echo "Running business service tests..."
	@cd services\business\rpc && go test ./... -v

test-chainrpc:
	@echo "Running chainrpc service tests..."
	@cd services\chainrpc\rpc && go test ./... -v

test-chainsync:
	@echo "Running chainsync service tests..."
	@cd services\chainsync\rpc && go test ./... -v

test-signer:
	@echo "Running signer service tests..."
	@cd services\signer\rpc && go test ./... -v

test-admin:
	@echo "Running admin service tests..."
	@cd services\admin\rpc && go test ./... -v

test-accounting:
	@echo "Running accounting service tests..."
	@cd services\accounting\rpc && go test ./... -v

test-market:
	@echo "Running market service tests..."
	@cd services\market\rpc && go test ./... -v

test-market-alert:
	@echo "Running market alert service tests..."
	@go test ./services/market-alert/... -v

# ============================================
# Quality & Maintenance
# ============================================

lint:
	@echo "Running golangci-lint..."
	@golangci-lint run --timeout=5m
	@echo "✓ Linting completed!"

clean:
	@echo "Cleaning build artifacts..."
	@if exist bin rmdir /S /Q bin
	@if exist services\business\rpc\logs rmdir /S /Q services\business\rpc\logs
	@if exist services\chainrpc\rpc\logs rmdir /S /Q services\chainrpc\rpc\logs
	@if exist services\chainsync\rpc\logs rmdir /S /Q services\chainsync\rpc\logs
	@if exist services\signer\rpc\logs rmdir /S /Q services\signer\rpc\logs
	@if exist services\admin\rpc\logs rmdir /S /Q services\admin\rpc\logs
	@if exist services\accounting\rpc\logs rmdir /S /Q services\accounting\rpc\logs
	@if exist services\market\rpc\logs rmdir /S /Q services\market\rpc\logs
	@if exist api-gateway\logs rmdir /S /Q api-gateway\logs
	@echo "✓ Clean completed!"

swagger:
	@echo "Syncing Swagger/OpenAPI artifacts..."
	@./scripts/sync-swagger-http-yaml.sh
	@echo "✓ Swagger/OpenAPI artifacts synced!"

gen-swagger: gen-swagger-business gen-swagger-admin
	@echo "✓ All Swagger docs generated successfully!"

gen-swagger-business:
	@echo "Generating Swagger docs for business service..."
	@if not exist docs\swagger mkdir docs\swagger
	@powershell -NoProfile -ExecutionPolicy Bypass -Command "$$gopath = (go env GOPATH); $$bin = Join-Path $$gopath 'bin'; if (-not (Test-Path (Join-Path $$bin 'protoc-gen-openapiv2.exe'))) { go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest }; $$gw = (go list -m -f '{{.Dir}}' github.com/grpc-ecosystem/grpc-gateway/v2); Set-Location proto; $$env:PATH = $$bin + ';' + $$env:PATH; protoc --plugin=protoc-gen-openapiv2=$$bin\protoc-gen-openapiv2.exe --proto_path=. --proto_path=$$gw --openapiv2_out=../docs/swagger --openapiv2_opt=grpc_api_configuration=../docs/swagger/business_http.yaml,generate_unbound_methods=true,use_go_templates=true business.proto"
	@rem set info via openapi_configuration; skip post-process to avoid PowerShell pipe errors
	@if not exist docs\swagger\business.swagger.json ( if exist docs\swagger\apidocs.swagger.json ren docs\swagger\apidocs.swagger.json business.swagger.json )
	@if not exist docs\swagger\business.swagger.json ( echo Swagger generation failed for business & exit /b 1 )
	@echo "✓ Swagger docs generated at docs\\swagger\\business.swagger.json"

gen-swagger-admin:
	@echo "Generating Swagger docs for admin service..."
	@if not exist docs\swagger mkdir docs\swagger
	@powershell -NoProfile -ExecutionPolicy Bypass -Command "$$gopath = (go env GOPATH); $$bin = Join-Path $$gopath 'bin'; if (-not (Test-Path (Join-Path $$bin 'protoc-gen-openapiv2.exe'))) { go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest }; $$gw = (go list -m -f '{{.Dir}}' github.com/grpc-ecosystem/grpc-gateway/v2); Set-Location proto; $$env:PATH = $$bin + ';' + $$env:PATH; protoc --plugin=protoc-gen-openapiv2=$$bin\protoc-gen-openapiv2.exe --proto_path=. --proto_path=$$gw --openapiv2_out=../docs/swagger --openapiv2_opt=grpc_api_configuration=../docs/swagger/admin_http.yaml,generate_unbound_methods=true,use_go_templates=true admin.proto"
	@rem set info via openapi_configuration; skip post-process to avoid PowerShell pipe errors
	@if not exist docs\swagger\admin.swagger.json ( if exist docs\swagger\apidocs.swagger.json ren docs\swagger\apidocs.swagger.json admin.swagger.json )
	@if not exist docs\swagger\admin.swagger.json ( echo Swagger generation failed for admin & exit /b 1 )
	@echo "✓ Swagger docs generated at docs\\swagger\\admin.swagger.json"

# ============================================
# Docker Image Build
# ============================================

docker-build:
	@echo "Building Docker images..."
	@cd deploy\docker && bash build-images.sh latest

docker-build-service:
	@echo "Building Docker image for $(SERVICE)..."
	@docker build -f deploy\docker\Dockerfile.service --build-arg SERVICE_NAME=$(SERVICE) -t internal-wallet-$(SERVICE):latest .

docker-build-gateway:
	@echo "Building Docker image for API Gateway..."
	@docker build -f deploy\docker\Dockerfile.gateway -t internal-wallet-gateway:latest .

# ============================================
# Kubernetes Deployment
# ============================================

k8s-deploy:
	@echo "Deploying to Kubernetes..."
	@cd deploy\k8s && bash deploy.sh all

k8s-deploy-namespace:
	@echo "Creating Kubernetes namespace..."
	@cd deploy\k8s && bash deploy.sh namespace

k8s-deploy-infrastructure:
	@echo "Deploying infrastructure services to Kubernetes..."
	@cd deploy\k8s && bash deploy.sh infrastructure

k8s-deploy-services:
	@echo "Deploying application services to Kubernetes..."
	@cd deploy\k8s && bash deploy.sh services

k8s-status:
	@echo "Checking Kubernetes deployment status..."
	@kubectl get all -n crypto-wallet

k8s-logs:
	@echo "Viewing logs for $(SERVICE) service..."
	@kubectl logs -f deployment/$(SERVICE) -n crypto-wallet

k8s-cleanup:
	@echo "Cleaning up Kubernetes resources..."
	@cd deploy\k8s && bash cleanup.sh all

k8s-shell:
	@echo "Opening shell in $(SERVICE) pod..."
	@kubectl exec -it deployment/$(SERVICE) -n crypto-wallet -- sh

# ============================================
# Test Environment (Docker Compose)
# ============================================

test-env-deploy:
	@echo "Deploying full test environment..."
	@cd deploy/docker && powershell -ExecutionPolicy Bypass -File deploy-test.ps1 deploy

test-env-infra:
	@echo "Deploying infrastructure only..."
	@cd deploy/docker && powershell -ExecutionPolicy Bypass -File deploy-test.ps1 infra

test-env-status:
	@echo "Checking test environment status..."
	@cd deploy/docker && powershell -ExecutionPolicy Bypass -File deploy-test.ps1 status

test-env-logs:
	@echo "Viewing logs for $(SERVICE) service..."
	@cd deploy/docker && powershell -ExecutionPolicy Bypass -File deploy-test.ps1 logs $(SERVICE)

test-env-cleanup:
	@echo "Cleaning up test environment..."
	@cd deploy/docker && powershell -ExecutionPolicy Bypass -File deploy-test.ps1 cleanup
