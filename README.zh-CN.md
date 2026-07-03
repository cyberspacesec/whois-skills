<div align="center">

# 🔍 Whois Hacker

**一站式 WHOIS 域名情报查询工具**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/cyberspacesec/whois-skills/actions/workflows/ci.yml/badge.svg)](https://github.com/cyberspacesec/whois-skills/actions)
[![文档](https://img.shields.io/badge/文档-vitepress-41b883.svg)](https://cyberspacesec.github.io/whois-skills/)
[![GitHub stars](https://img.shields.io/github/stars/cyberspacesec/whois-skills?style=social)](https://github.com/cyberspacesec/whois-skills/stargazers)

🚀 域名 / IP / ASN / RDAP / 反向 WHOIS · 批量处理 · 关联分析 · 监控告警 · MCP 协议

**[📖 English Docs](./README.md)** ｜ **[📖 中文文档](./README.zh-CN.md)** ｜ **[📚 完整文档站](https://cyberspacesec.github.io/whois-skills/)**

</div>

---

## 概览

Whois Hacker 是一个用 Go 编写的 **WHOIS 域名情报查询工具**与 HTTP 服务。它将域名、IP、ASN、RDAP、反向 WHOIS 等分散的能力整合为统一的可编程接口，并叠加缓存、代理、限流、调度、监控、告警、关联分析等工程化能力。

让情报收集从"手工拼凑"升级为"工程化流水线"。

### 🎯 解决什么问题

| 痛点 | 传统方式 | Whois Hacker 方案 |
|------|---------|-------------------|
| 🔁 单点查询效率低 | 逐个手动查询 | 优先级队列 + 并发聚合 + 流式批量 |
| 🌐 国际域名编码复杂 | 手工 Punycode | 内置 IDN 规范化与 Punycode 互转 |
| 📋 各注册局格式不统一 | 各 RIR 格式各异 | 自动识别 ARIN/RIPE/APNIC/LACNIC/AFRINIC 并统一解析 |
| 🚫 高频查询易被封禁 | 直连易触发限速 | 代理池 + 令牌桶 + 自适应调度退避 |
| 🔍 结果难以结构化 | 原始文本 | 统一 `WhoisInfo` + JSON/CSV/Markdown 导出 |
| 🔗 多域名无关联视角 | 单域名孤立查看 | 关联分析引擎，按邮箱/注册人/组织聚类 |
| ⏰ 域名变更无感知 | 无法持续跟踪 | 域名监控器，到期与变更告警 |

### ✨ 核心特性

- 🔍 **域名 WHOIS** — 优先级队列、并发聚合、结果校验、引导跟随，覆盖 130+ TLD
- 🌐 **IP WHOIS** — IANA 引导定位 RIR，5 大 RIR 格式结构化解析
- 🔢 **ASN 查询** — RADB + RDAP 双数据源，BGP 关系、前缀、批量查询
- 📡 **RDAP** — RFC 9083，域名/IP/ASN/Entity，内置 bootstrap
- 🔄 **反向 WHOIS** — 按邮箱/组织/注册人反查（Provider 接口）
- 📋 **批量查询** — 流式处理器，断点续查、限速、剩余时间预估
- 🔗 **关联分析** — 按邮箱/注册人/组织/NS/注册商聚类，资产画像
- 👁️ **域名监控** — 到期与变更检测，分级告警
- ⭐ **质量评分** — 完整性/时效性/可信度，隐私检测（13 条规则）
- 💾 **多层缓存** — 本地内存与 Redis，预热、命中率统计
- 🔒 **代理池** — SOCKS5/HTTP，轮询、健康检查、故障熔断
- ⏱️ **速率限制** — 令牌桶，全局 + 每服务器双维度
- 🎛️ **智能调度** — 自适应间隔/退避/并发
- 🚨 **监控告警** — CPU/内存/错误率/失败率规则，Email/Slack/Webhook
- 🤖 **MCP 协议** — 任务规划/执行/审批状态机

### 📦 三套集成接口

| 接口 | 适用场景 |
|------|---------|
| 📦 **Go 库** | 直接 import `pkg/whois` 作为 SDK 嵌入 Go 程序 |
| 🌐 **HTTP API** | REST 端点，任意语言 / Web 场景 |
| 🤖 **MCP 协议** | 任务规划流，AI Agent 场景 |

### 快速开始

```bash
git clone https://github.com/cyberspacesec/whois-skills.git
cd whois-skills
go mod tidy
make build
./bin/whois-hacker
```

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

result, err := whois.ExecuteQueryWithResult(&whois.QueryOptions{
    Domain: "example.com",
})
```

```bash
# HTTP API
curl -X POST http://127.0.0.1:8080/api/whois \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com"}'
```

### 📂 项目结构

```
whois-skills/
├── cmd/whois-hacker/     # 🚀 命令行入口与 HTTP 服务
├── pkg/
│   ├── whois/            # 🔍 WHOIS 核心库（24 个文件）
│   ├── api/              # 🌐 HTTP API 服务
│   ├── mcp/              # 🤖 MCP 协议服务
│   ├── metrics/          # 📈 指标与告警
│   ├── monitor/          # 👁️ 性能监控
│   └── security/         # 🔒 API Key 认证
├── config/               # ⚙️ 配置文件
├── website/              # 📚 文档站（VitePress）
├── Dockerfile            # 🐳 Docker 构建
└── Makefile              # 🔧 构建/测试/发布
```

### 📚 文档

完整文档站：**https://cyberspacesec.github.io/whois-skills/**

- 📖 [快速开始](https://cyberspacesec.github.io/whois-skills/guide/getting-started)
- 🏗️ [架构总览](https://cyberspacesec.github.io/whois-skills/guide/architecture)
- 🌐 [HTTP API](https://cyberspacesec.github.io/whois-skills/api/http/overview)
- 🤖 [MCP 协议](https://cyberspacesec.github.io/whois-skills/api/mcp/overview)
- 🚀 [部署](https://cyberspacesec.github.io/whois-skills/deploy/docker)

### 🐳 Docker

```bash
docker pull cyberspacesec/whois-skills:latest
docker run -d -p 8080:8080 -v whois_data:/app/data cyberspacesec/whois-skills:latest
```

### 🤝 贡献

欢迎贡献！请 fork 仓库、创建分支、提交带测试的 PR。

### 📄 协议

[MIT 协议](LICENSE) © CyberSpaceSec

---

> 🌐 本文档也提供 **[English](./README.md)** 版本。
