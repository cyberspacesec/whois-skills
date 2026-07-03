# 🐳 Docker 部署

> 📖 whois-skills 提供多阶段构建的 Dockerfile，基于 `golang:1.23-alpine` 构建、`alpine:3.19` 运行，静态链接 + UPX 压缩，非 root 用户运行，内置健康检查。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| Dockerfile | 仓库根目录 `Dockerfile` |
| 构建镜像 | `golang:1.23-alpine`（builder） |
| 运行镜像 | `alpine:3.19` |
| 暴露端口 | `8080` |
| 数据卷 | `/app/data` |
| 健康检查 | `curl -f http://localhost:8080/api/health` |

---

## 🏗️ 多阶段构建

### 第一阶段：builder（`golang:1.23-alpine`）

| 步骤 | 说明 |
|------|------|
| 安装依赖 | `git make` |
| `go mod download` | 预取依赖，利用缓存 |
| 复制源码 | `COPY . .` |
| `go build` | `CGO_ENABLED=0` 静态链接，`-ldflags` 注入 Version/BuildTime/GitCommit，输出 `bin/whois-hacker` |
| UPX 压缩 | `upx --best --lzma` 压缩可执行文件 |

关键构建命令：

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo \
    -ldflags="-s -w -X main.Version=... -X main.BuildTime=... -X main.GitCommit=..." \
    -o bin/whois-hacker ./cmd/whois-hacker
```

### 第二阶段：运行时（`alpine:3.19`）

| 步骤 | 说明 |
|------|------|
| 安装运行时依赖 | `ca-certificates tzdata curl` |
| 时区 | `Asia/Shanghai`（复制 localtime + 写 timezone） |
| 工作目录 | `/app` |
| 复制二进制 | `bin/whois-hacker → /app/whois-hacker` |
| 复制网站 | `website → /app/website` |
| 非 root 用户 | `appuser:appgroup` |
| 数据卷 | `VOLUME /app/data` |
| 暴露端口 | `EXPOSE 8080` |
| 健康检查 | `curl -f /api/health`，30s 间隔，5s 超时，3 次重试 |
| 入口 | `ENTRYPOINT ["/app/whois-hacker"]` |
| 默认命令 | `CMD ["serve", "--host", "0.0.0.0", "--port", "8080"]` |

---

## 🚀 快速使用

### 拉取官方镜像

```bash
docker pull cyberspacesec/whois-skills:latest
```

### 运行

```bash
docker run -d \
  --name whois-hacker \
  -p 8080:8080 \
  -v whois_data:/app/data \
  cyberspacesec/whois-skills:latest
```

### 自行构建

```bash
docker build -t cyberspacesec/whois-skills:latest .
docker run -d -p 8080:8080 cyberspacesec/whois-skills:latest
```

### 健康检查

```bash
curl http://localhost:8080/api/health
# {"status":"ok","time":"..."}
```

---

## 🔧 Makefile 目标

| 命令 | 说明 |
|------|------|
| `make docker` | 构建单平台镜像，打 `:$(VERSION)` 与 `:latest` 标签 |
| `make docker-multi` | 多平台构建（`linux/amd64` + `linux/arm64`），需 Docker BuildX，`--push` 推送 |

```bash
make docker                # 本地构建
make docker-multi VERSION=0.1.0   # 多平台推送
```

---

## ⚠️ 注意事项

::: warning ⚠️ serve 子命令未实现
`CMD` 中的 `serve` 是子命令形式，但 `main.go` 当前**未实现子命令分发**。`flag.Parse` 会忽略非 flag 参数（`serve`），仍以默认 flag 启动 HTTP 服务，因此功能可用，但 `--host`/`--port` 通过 CMD 传入时会被 flag 包当作首个非 flag 参数之后的位置忽略。

**实际效果**：容器仍会以 `--host 0.0.0.0 --port 8080` 启动（因 flag 解析在遇到首个非 flag 参数 `serve` 后停止，后续 `--host`/`--port` 未被解析）。若需指定参数，建议通过 `docker run` 覆盖 CMD，直接传 flag：

```bash
docker run -d -p 8080:8080 cyberspacesec/whois-skills:latest \
  --host 0.0.0.0 --port 8080
```
:::

- 镜像内时区固定为 `Asia/Shanghai`。
- 以 `appuser` 非 root 运行，挂载卷需注意属主权限。
- UPX 压缩可减小体积，但部分运行时（如启用了 seccomp 限制禁止 unpacking）可能拒绝执行，可移除该步骤。

---

## 🔗 相关链接

- [Docker Compose 部署](./compose.md)
- [二进制部署](./binary.md)
- [cmd 模块](../modules/cmd.md)
