# Makefile for Live Service

# 变量定义
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOPROTO=protoc
DOCKER=docker
DOCKER_COMPOSE=docker-compose

# 二进制文件名称
BINARY_NAME=live-service
BINARY_UNIX=$(BINARY_NAME)_unix

# 构建目录
BUILD_DIR=build
CMD_DIR=cmd/server

# 版本信息
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH=$(shell git rev-parse --short HEAD)

# Go 构建参数
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.CommitHash=$(COMMIT_HASH) -s -w"

# 默认目标
.PHONY: all
all: clean proto build

# 清理构建文件
.PHONY: clean
clean:
	@echo "Cleaning build files..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f $(BINARY_UNIX)
	@echo "Clean completed!"

# 下载依赖
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy
	@echo "Dependencies downloaded!"

# 生成protobuf代码
.PHONY: proto
proto:
	@echo "Generating protobuf code..."
	@$(GOPROTO) --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto
	@echo "Protobuf code generated!"

# 构建应用
.PHONY: build
build: deps
	@echo "Building application..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# 构建Linux版本
.PHONY: build-linux
build-linux: deps
	@echo "Building Linux version..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_UNIX) $(CMD_DIR)
	@echo "Linux build completed: $(BUILD_DIR)/$(BINARY_UNIX)"

# 运行应用
.PHONY: run
run: proto
	@echo "Running application..."
	@$(GOCMD) run $(CMD_DIR)/main.go

# 运行开发模式（带热重载）
.PHONY: dev
dev:
	@echo "Running in development mode..."
	@air -c .air.toml

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	@$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests completed!"

# 运行单元测试
.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@$(GOTEST) -v -short ./...
	@echo "Unit tests completed!"

# 运行集成测试
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	@$(GOTEST) -v -tags=integration ./test/...
	@echo "Integration tests completed!"

# 性能测试
.PHONY: benchmark
benchmark:
	@echo "Running benchmarks..."
	@$(GOTEST) -v -bench=. -benchmem ./benchmark/...
	@echo "Benchmarks completed!"

# 代码覆盖率
.PHONY: coverage
coverage: test
	@echo "Generating coverage report..."
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 代码格式化
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .
	@goimports -w .
	@echo "Code formatted!"

# 代码检查
.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...
	@echo "Linting completed!"

# 安全检查
.PHONY: security
security:
	@echo "Running security check..."
	@gosec -quiet ./...
	@echo "Security check completed!"

# 静态分析
.PHONY: static-check
static-check:
	@echo "Running static analysis..."
	@go vet ./...
	@echo "Static analysis completed!"

# 所有代码质量检查
.PHONY: check
check: fmt lint static-check security
	@echo "All code quality checks completed!"

# 构建Docker镜像
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@$(DOCKER) build -t $(BINARY_NAME):latest .
	@$(DOCKER) build -t $(BINARY_NAME):$(VERSION) .
	@echo "Docker image built successfully!"

# 运行Docker容器
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	@$(DOCKER) run -d --name $(BINARY_NAME) -p 8080:8080 -p 8081:8081 $(BINARY_NAME):latest
	@echo "Docker container started!"

# 停止Docker容器
.PHONY: docker-stop
docker-stop:
	@echo "Stopping Docker container..."
	@$(DOCKER) stop $(BINARY_NAME)
	@$(DOCKER) rm $(BINARY_NAME)
	@echo "Docker container stopped!"

# Docker Compose启动
.PHONY: compose-up
compose-up:
	@echo "Starting services with Docker Compose..."
	@$(DOCKER_COMPOSE) up -d
	@echo "Services started!"

# Docker Compose停止
.PHONY: compose-down
compose-down:
	@echo "Stopping services with Docker Compose..."
	@$(DOCKER_COMPOSE) down
	@echo "Services stopped!"

# Docker Compose日志
.PHONY: compose-logs
compose-logs:
	@$(DOCKER_COMPOSE) logs -f

# 数据库迁移
.PHONY: migrate
migrate:
	@echo "Running database migrations..."
	@mysql -h localhost -u root -p live_service < sql/init.sql
	@echo "Database migrations completed!"

# 数据库重置
.PHONY: db-reset
db-reset:
	@echo "Resetting database..."
	@mysql -h localhost -u root -p -e "DROP DATABASE IF EXISTS live_service; CREATE DATABASE live_service;"
	@$(MAKE) migrate
	@echo "Database reset completed!"

# 生成API文档
.PHONY: docs
docs:
	@echo "Generating API documentation..."
	@swag init -g $(CMD_DIR)/main.go
	@echo "API documentation generated!"

# 打包发布
.PHONY: package
package: clean proto build-linux
	@echo "Packaging application..."
	@mkdir -p $(BUILD_DIR)/package
	@cp $(BUILD_DIR)/$(BINARY_UNIX) $(BUILD_DIR)/package/
	@cp -r configs $(BUILD_DIR)/package/
	@cp -r sql $(BUILD_DIR)/package/
	@cp Dockerfile $(BUILD_DIR)/package/
	@cp docker-compose.yml $(BUILD_DIR)/package/
	@cp README.md $(BUILD_DIR)/package/
	@cd $(BUILD_DIR)/package && tar -czf ../$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz .
	@echo "Package created: $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz"

# 部署到测试环境
.PHONY: deploy-test
deploy-test: package
	@echo "Deploying to test environment..."
	@scp $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz test-server:/tmp/
	@ssh test-server "cd /tmp && tar -xzf $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz && ./$(BINARY_UNIX)"
	@echo "Deployed to test environment!"

# 部署到生产环境
.PHONY: deploy-prod
deploy-prod: package
	@echo "Deploying to production environment..."
	@scp $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz prod-server:/tmp/
	@ssh prod-server "cd /tmp && tar -xzf $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz && ./$(BINARY_UNIX)"
	@echo "Deployed to production environment!"

# 清理Docker
.PHONY: docker-clean
docker-clean:
	@echo "Cleaning Docker resources..."
	@$(DOCKER) system prune -f
	@$(DOCKER) volume prune -f
	@echo "Docker cleanup completed!"

# 显示帮助
.PHONY: help
help:
	@echo "Live Service Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all              - Clean, generate protobuf, and build"
	@echo "  clean            - Remove build files"
	@echo "  deps             - Download dependencies"
	@echo "  proto            - Generate protobuf code"
	@echo "  build            - Build application"
	@echo "  build-linux      - Build Linux version"
	@echo "  run              - Run application"
	@echo "  dev              - Run in development mode with hot reload"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  benchmark        - Run benchmarks"
	@echo "  coverage         - Generate coverage report"
	@echo "  fmt              - Format code"
	@echo "  lint             - Run linter"
	@echo "  security         - Run security check"
	@echo "  static-check     - Run static analysis"
	@echo "  check            - Run all code quality checks"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Run Docker container"
	@echo "  docker-stop      - Stop Docker container"
	@echo "  compose-up       - Start services with Docker Compose"
	@echo "  compose-down     - Stop services with Docker Compose"
	@echo "  compose-logs     - View Docker Compose logs"
	@echo "  migrate          - Run database migrations"
	@echo "  db-reset         - Reset database"
	@echo "  docs             - Generate API documentation"
	@echo "  package          - Package application for deployment"
	@echo "  deploy-test      - Deploy to test environment"
	@echo "  deploy-prod      - Deploy to production environment"
	@echo "  docker-clean     - Clean Docker resources"
	@echo "  help             - Show this help message"

# 默认显示帮助
.DEFAULT_GOAL := help