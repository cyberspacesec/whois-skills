.PHONY: build test clean docker docker-multi run help

# 版本信息
VERSION ?= 0.1.0
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# 默认目标
all: clean build test

# 帮助信息
help:
	@echo "可用命令:"
	@echo "  make build         - 构建项目"
	@echo "  make build-all     - 构建多平台二进制文件 (linux, windows, darwin)"
	@echo "  make test          - 运行测试"
	@echo "  make clean         - 清理构建产物"
	@echo "  make docker        - 构建Docker镜像"
	@echo "  make docker-multi  - 构建多平台Docker镜像 (linux/amd64, linux/arm64)"
	@echo "  make run           - 直接运行API服务"
	@echo "  make all           - 清理、构建和测试"
	@echo "  make help          - 显示此帮助信息"

# 构建项目
build:
	@echo "构建 whois-hacker..."
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/whois-hacker ./cmd/whois-hacker
	@echo "构建完成: bin/whois-hacker"

# 构建多平台二进制文件
build-all:
	@echo "构建多平台二进制文件..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/whois-hacker-linux-amd64 ./cmd/whois-hacker
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/whois-hacker-linux-arm64 ./cmd/whois-hacker
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/whois-hacker-windows-amd64.exe ./cmd/whois-hacker
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/whois-hacker-darwin-amd64 ./cmd/whois-hacker
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/whois-hacker-darwin-arm64 ./cmd/whois-hacker
	@echo "多平台构建完成"

# 运行测试
test:
	@echo "运行测试..."
	go test -v ./...

# 清理构建产物
clean:
	@echo "清理构建产物..."
	rm -rf bin/
	@echo "清理完成"

# 构建Docker镜像
docker:
	@echo "构建Docker镜像..."
	docker build -t cyberspacesec/whois-hacker:$(VERSION) .
	docker tag cyberspacesec/whois-hacker:$(VERSION) cyberspacesec/whois-hacker:latest
	@echo "Docker镜像构建完成"

# 构建多平台Docker镜像
docker-multi:
	@echo "构建多平台Docker镜像 (需要 Docker BuildX)..."
	docker buildx create --use --name=whois-builder || true
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t cyberspacesec/whois-hacker:$(VERSION) \
		-t cyberspacesec/whois-hacker:latest \
		--push .
	@echo "多平台Docker镜像构建完成"

# 直接运行API服务
run:
	@echo "启动API服务..."
	go run ./cmd/whois-hacker/main.go ./cmd/whois-hacker/api.go serve
