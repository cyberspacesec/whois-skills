# 📦 二进制部署

> 📖 whois-skills 提供 Makefile 一键构建与多平台交叉编译，也可从 GitHub Releases 下载预编译二进制，配合 systemd 或直接运行即可提供服务。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 构建工具 | Makefile + go build |
| Go 版本 | 1.23+ |
| 支持平台 | linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64 |
| 输出目录 | `bin/` |

---

## 🛠️ 构建

### 单平台构建

```bash
make build
# 输出：bin/whois-hacker
```

### 多平台交叉编译

```bash
make build-all
```

### 平台对照表

| 文件名 | OS | ARCH |
|--------|----|------|
| `whois-hacker-linux-amd64` | Linux | amd64 |
| `whois-hacker-linux-arm64` | Linux | arm64 |
| `whois-hacker-windows-amd64.exe` | Windows | amd64 |
| `whois-hacker-darwin-amd64` | macOS | amd64 |
| `whois-hacker-darwin-arm64` | macOS | arm64（Apple Silicon） |

版本注入：`make build` 通过 `LDFLAGS` 注入 `main.Version`、`main.BuildTime`、`main.GitCommit`。

---

## 📥 从 GitHub Releases 下载

```bash
# 下载（替换 VERSION 与平台）
wget https://github.com/cyberspacesec/whois-skills/releases/download/v0.1.0/whois-hacker-linux-amd64
chmod +x whois-hacker-linux-amd64
./whois-hacker-linux-amd64 --host 0.0.0.0 --port 8080
```

---

## 🚀 直接运行

```bash
# 默认配置
./whois-hacker

# 自定义
./whois-hacker --host 0.0.0.0 --port 8080 --log-level debug --cache-ttl 7200

# 启用代理
./whois-hacker --proxy --proxy-file config/proxies.json
```

完整 flag 列表见 [cmd 模块](../modules/cmd.md)。

---

## 🖥️ systemd 服务（可选）

`/etc/systemd/system/whois-hacker.service`：

```ini
[Unit]
Description=Whois Hacker API Service
After=network.target

[Service]
Type=simple
User=whois
WorkingDirectory=/opt/whois-hacker
ExecStart=/opt/whois-hacker/whois-hacker --host 0.0.0.0 --port 8080
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now whois-hacker
sudo systemctl status whois-hacker
journalctl -u whois-hacker -f
```

---

## ⚙️ 环境变量与配置文件

::: warning ⚠️ 不读取环境变量
`main.go` 当前**不读取环境变量**，所有配置通过 flag 或 YAML 配置文件（默认 `config/config.yaml`，`--config` 指定）传入。优先级：命令行 > YAML > 默认。
:::

YAML 配置示例（`config/config.yaml`）：

```yaml
server:
  host: "0.0.0.0"
  port: 8080
log:
  level: "info"
  format: "text"
cache:
  enabled: true
  type: "local"
  ttl: 3600
proxy:
  enabled: false
  file: "config/proxies.json"
metrics:
  enabled: true
  interval: 60
alerts:
  enabled: true
  interval: 60
```

---

## ✅ 健康检查验证

```bash
curl http://localhost:8080/api/health
# {"status":"ok","time":"..."}

# 查询示例
curl -X POST http://localhost:8080/api/whois \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com"}'
```

---

## 🔗 相关链接

- [Docker 部署](./docker.md)
- [GitHub Actions](./github-actions.md)
- [cmd 模块](../modules/cmd.md)
