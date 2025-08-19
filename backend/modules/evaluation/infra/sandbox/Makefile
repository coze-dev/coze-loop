# Makefile for 代码评估器沙箱环境
# 统一构建和部署流程

.PHONY: help install build test clean run docker-build docker-run docker-stop lint fmt deps check-deps

# 默认目标
help: ## 显示帮助信息
	@echo "代码评估器沙箱环境 - 可用命令:"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# 安装和依赖管理
install: check-deps deps install-deno ## 完整环境安装
	@echo "✅ 环境安装完成"

deps: ## 安装Go依赖
	@echo "安装Go依赖..."
	go mod download
	go mod tidy

install-deno: ## 安装Deno运行时
	@echo "安装 Deno..."
	@if ! which deno > /dev/null; then \
		curl -fsSL https://deno.land/x/install/install.sh | sh; \
		echo "请将 ~/.deno/bin 添加到 PATH 环境变量"; \
	else \
		echo "✅ Deno 已安装: $$(deno --version | head -1)"; \
	fi

check-deps: ## 检查系统依赖
	@echo "检查系统依赖..."
	@which go > /dev/null || (echo "❌ Go 未安装" && exit 1)
	@echo "✅ Go 已安装: $$(go version)"
	@which deno > /dev/null || (echo "⚠️  Deno 未安装，请运行 'make install-deno'" && exit 1)
	@echo "✅ Deno 已安装: $$(deno --version | head -1)"
	@echo "✅ 所有依赖已满足"

# 代码质量
fmt: ## 格式化代码
	go fmt ./...
	gofmt -s -w .

lint: ## 代码检查
	@echo "运行代码检查..."
	@if which golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "⚠️  golangci-lint 未安装，使用 go vet"; \
		go vet ./...; \
	fi

# 构建和运行
build: ## 构建应用
	@echo "构建沙箱应用..."
	go build -o bin/sandbox ./cmd/demo

run: build ## 运行应用
	@echo "启动沙箱服务..."
	./bin/sandbox

clean: ## 清理构建文件
	@echo "清理构建文件..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# 测试
test: ## 运行测试
	@echo "运行单元测试..."
	go test -v -race -coverprofile=coverage.out ./...

test-short: ## 运行快速测试
	go test -v -short ./...

test-coverage: test ## 生成测试覆盖率报告
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# API测试
test-api: ## 测试API接口
	@echo "测试健康检查..."
	@curl -f http://localhost:8080/api/v1/sandbox/health 2>/dev/null || echo "❌ 服务未启动"
	@echo ""

test-python: ## 测试Python执行
	@echo "测试Python代码执行..."
	@curl -X POST http://localhost:8080/api/v1/sandbox/execute \
		-H "Content-Type: application/json" \
		-d '{"code": "score = 1.0\nreason = \"Python test\"", "language": "python"}' \
		2>/dev/null || echo "❌ 服务未启动"

test-javascript: ## 测试JavaScript执行
	@echo "测试JavaScript代码执行..."
	@curl -X POST http://localhost:8080/api/v1/sandbox/execute \
		-H "Content-Type: application/json" \
		-d '{"code": "const score = 1.0; const reason = \"JavaScript test\";", "language": "javascript"}' \
		2>/dev/null || echo "❌ 服务未启动"

# Docker相关
docker-build: ## 构建Docker镜像
	@echo "构建Docker镜像..."
	docker build -t cozeloop/sandbox:latest -f Dockerfile ../../../

docker-run: ## 运行Docker容器
	@echo "启动Docker容器..."
	docker-compose up -d

docker-stop: ## 停止Docker容器
	@echo "停止Docker容器..."
	docker-compose down

docker-logs: ## 查看Docker日志
	docker-compose logs -f sandbox

docker-shell: ## 进入Docker容器
	docker-compose exec sandbox sh

# 部署相关
deploy-local: docker-build docker-run ## 本地部署
	@echo "等待服务启动..."
	@sleep 10
	@make test-api

# 性能测试
benchmark: ## 运行性能测试
	@echo "运行性能基准测试..."
	go test -bench=. -benchmem ./...

load-test: ## 负载测试
	@echo "运行负载测试..."
	@if which ab > /dev/null; then \
		ab -n 100 -c 10 http://localhost:8080/api/v1/sandbox/health; \
	else \
		echo "请安装 apache2-utils (ab命令)"; \
	fi

# 安全检查
security-scan: ## 安全扫描
	@echo "运行安全扫描..."
	@if which gosec > /dev/null; then \
		gosec ./...; \
	else \
		echo "请安装 gosec: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# 发布相关
release: test lint ## 准备发布
	@echo "准备发布版本..."
	@git status --porcelain | grep -q . && echo "❌ 工作目录不干净" && exit 1 || echo "✅ 工作目录干净"
	@echo "✅ 测试通过"
	@echo "✅ 代码检查通过"
	@echo "准备发布..."

# 清理所有
clean-all: clean docker-stop ## 清理所有内容
	@echo "清理Docker资源..."
	docker system prune -f
	docker volume prune -f

# 开发相关
dev: ## 开发模式（监听文件变化）
	@echo "开发模式启动..."
	@if [ -f ../../../.air.toml ]; then \
		cd ../../.. && air -c .air.toml; \
	else \
		echo "未找到 .air.toml 配置文件，使用普通模式"; \
		make run; \
	fi

# 验证构建流程
verify: install build test docker-build ## 验证完整构建流程
	@echo "✅ 构建流程验证完成"