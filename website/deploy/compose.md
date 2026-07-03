# 🐙 Docker Compose

> 📖 仓库提供 `docker-compose.yml`，一键编排 whois-skills 服务。本文逐字段说明配置，并指出与 Dockerfile 的路径不一致问题及环境变量未被读取的问题。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 编排文件 | 仓库根目录 `docker-compose.yml` |
| Compose 版本 | `3.8` |
| 服务 | `whois-hacker` |
| 镜像 | `whois-hacker:latest` |
| 端口 | `8080:8080` |
| 数据卷 | `whois_data` → `/app/data` |

---

## 📄 docker-compose.yml 详解

```yaml
version: '3.8'

services:
  whois-hacker:
    build:
      context: .
      dockerfile: Dockerfile
    image: whois-hacker:latest
    container_name: whois-hacker
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - HTTP_HOST=0.0.0.0
      - HTTP_PORT=8080
      - RATE_LIMIT_PER_MINUTE=60
      - ENABLE_RATE_LIMIT=true
      - LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "/app/bin/whois-hacker", "version"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 5s
    command: ["/app/bin/whois-hacker", "serve", "--host", "0.0.0.0", "--port", "8080"]
    volumes:
      - whois_data:/app/data

volumes:
  whois_data:
```

### 字段说明

| 字段 | 值 | 说明 |
|------|----|------|
| `build.context` | `.` | 构建上下文为仓库根 |
| `build.dockerfile` | `Dockerfile` | 使用根 Dockerfile |
| `image` | `whois-hacker:latest` | 镜像名（注意与 Docker Hub 的 `cyberspacesec/whois-skills` 不同） |
| `container_name` | `whois-hacker` | 容器名 |
| `restart` | `unless-stopped` | 异常退出自动重启，手动 stop 不重启 |
| `ports` | `8080:8080` | 宿主机:容器端口映射 |
| `environment` | 见下 | 环境变量（⚠️ 当前未被 main 读取） |
| `healthcheck` | `/app/bin/whois-hacker version` | 健康检查（⚠️ 路径不一致） |
| `command` | `/app/bin/whois-hacker serve ...` | 启动命令（⚠️ 路径不一致） |
| `volumes` | `whois_data:/app/data` | 数据持久化 |

### 环境变量

| 变量 | 值 |
|------|----|
| `HTTP_HOST` | `0.0.0.0` |
| `HTTP_PORT` | `8080` |
| `RATE_LIMIT_PER_MINUTE` | `60` |
| `ENABLE_RATE_LIMIT` | `true` |
| `LOG_LEVEL` | `info` |

---

## 🚀 常用命令

```bash
# 构建并启动（后台）
docker-compose up -d

# 查看日志
docker-compose logs -f

# 查看状态
docker-compose ps

# 停止并移除容器
docker-compose down

# 停止但保留容器
docker-compose stop

# 重新构建
docker-compose up -d --build
```

---

## ⚠️ 已知问题

::: warning 🐛 问题一：路径不一致
`command` 与 `healthcheck` 引用的可执行文件路径为 `/app/bin/whois-hacker`，但 [Dockerfile](./docker.md) 将二进制复制到了 `/app/whois-hacker`（无 `bin/` 子目录）。这会导致容器**启动失败**（找不到可执行文件）。

**修复方案**（任选其一）：

1. 修改 `docker-compose.yml` 中的路径：
   ```yaml
   command: ["/app/whois-hacker", "--host", "0.0.0.0", "--port", "8080"]
   healthcheck:
     test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
   ```

2. 或修改 Dockerfile，复制到 `/app/bin/whois-hacker` 并保证目录存在。
:::

::: warning 🐛 问题二：环境变量未被读取
`main.go` 当前**不读取任何环境变量**（无 `os.Getenv` 调用），所有配置走 flag 或 YAML。因此 `HTTP_HOST`、`HTTP_PORT`、`RATE_LIMIT_PER_MINUTE`、`ENABLE_RATE_LIMIT`、`LOG_LEVEL` 这些 environment 配置**全部无效**。

**修复方案**：

1. 通过 `command` 以 flag 形式传递：
   ```yaml
   command: ["/app/whois-hacker", "--host", "0.0.0.0", "--port", "8080", "--log-level", "info"]
   ```

2. 或挂载 YAML 配置文件，通过 `--config` 指定。
:::

::: warning 🐛 问题三：serve 子命令未实现
`command` 中的 `serve` 子命令在 `main.go` 中**未实现分发逻辑**。`flag.Parse` 遇到非 flag 参数 `serve` 会停止解析，其后的 `--host`/`--port` 不会被解析。建议 `command` 直接以 flag 开头，去掉 `serve`。
:::

---

## ✅ 修正后的可用配置

```yaml
services:
  whois-hacker:
    build: .
    image: whois-hacker:latest
    container_name: whois-hacker
    restart: unless-stopped
    ports:
      - "8080:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 5s
    command: ["--host", "0.0.0.0", "--port", "8080", "--log-level", "info"]
    volumes:
      - whois_data:/app/data

volumes:
  whois_data:
```

---

## 🔗 相关链接

- [Docker 部署](./docker.md)
- [cmd 模块](../modules/cmd.md) — flag 与配置加载
- [故障排查](../reference/troubleshooting.md)
