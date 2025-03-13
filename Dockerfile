# 多阶段构建: 第一阶段 - 构建应用
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git make

# 复制go模块文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用 - 设置为静态链接，优化镜像大小
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w -X main.Version=$(cat VERSION 2>/dev/null || echo 'dev') -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o bin/whois-hacker ./cmd/whois-hacker

# 使用upx压缩可执行文件
RUN apk add --no-cache upx && upx --best --lzma bin/whois-hacker

# 多阶段构建: 第二阶段 - 创建最小运行环境
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata curl && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    rm -rf /var/cache/apk/*

# 工作目录
WORKDIR /app

# 从builder阶段复制编译好的应用
COPY --from=builder /build/bin/whois-hacker /app/whois-hacker

# 可选：复制网站文件
COPY website /app/website

# 确保文件可执行
RUN chmod +x /app/whois-hacker

# 创建数据目录
RUN mkdir -p /app/data

# 创建非root用户运行应用
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app

# 使用非root用户运行
USER appuser

# 定义数据卷
VOLUME /app/data

# 暴露HTTP API端口
EXPOSE 8080

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

# 设置容器启动命令
ENTRYPOINT ["/app/whois-hacker"]

# 默认参数 - 启动API服务
CMD ["serve", "--host", "0.0.0.0", "--port", "8080"]