<div align="center">

# 🔍 Whois Hacker

**一站式 WHOIS 域名情报查询工具 / All-in-one WHOIS domain intelligence toolkit**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/cyberspacesec/whois-skills/actions/workflows/ci.yml/badge.svg)](https://github.com/cyberspacesec/whois-skills/actions)
[![Docs](https://img.shields.io/badge/docs-vitepress-41b883.svg)](https://cyberspacesec.github.io/whois-skills/)
[![GitHub stars](https://img.shields.io/github/stars/cyberspacesec/whois-skills?style=social)](https://github.com/cyberspacesec/whois-skills/stargazers)

🚀 域名 / IP / ASN / RDAP / 反向 WHOIS · 批量处理 · 关联分析 · 监控告警 · MCP 协议

🚀 Domain / IP / ASN / RDAP / Reverse WHOIS · Batch · Correlation · Monitoring · MCP

**[📖 中文文档](./README.zh-CN.md)** ｜ **[📖 English Docs](./README.md#overview)** ｜ **[📚 完整文档站 / Full Docs](https://cyberspacesec.github.io/whois-skills/)**

</div>

---

## Overview

Whois Hacker is a Go toolkit and HTTP service for **WHOIS domain intelligence gathering**. It unifies scattered capabilities — domain, IP, ASN, RDAP, and reverse WHOIS — into a single programmable interface, layered with engineering capabilities like caching, proxying, rate limiting, scheduling, monitoring, alerting, and correlation analysis.

It upgrades intelligence collection from "manual patchwork" to an "engineered pipeline."

### 🎯 What problem does it solve

| Pain point | Traditional | Whois Hacker |
|------------|-------------|--------------|
| 🔁 Low efficiency | Query one by one | Priority queue + concurrent aggregation + streaming batch |
| 🌐 IDN complexity | Manual Punycode | Built-in IDN normalization & Punycode conversion |
| 📋 Inconsistent formats | Each RIR differs | Auto-detect ARIN/RIPE/APNIC/LACNIC/AFRINIC & unified parsing |
| 🚫 Easy to get banned | Direct connection | Proxy pool + token bucket + adaptive scheduling backoff |
| 🔍 Unstructured results | Raw text | Unified `WhoisInfo` + JSON/CSV/Markdown export |
| 🔗 No correlation | Isolated domains | Correlation engine — cluster by email/registrant/org |
| ⏰ No change tracking | Manual re-check | Domain monitor — expiry & status/registrant/NS change alerts |

### ✨ Features

- 🔍 **Domain WHOIS** — priority queue, concurrent aggregation, result validation, referral following, 130+ TLDs
- 🌐 **IP WHOIS** — IANA bootstrap → RIR, structured parsing of 5 RIR formats
- 🔢 **ASN** — RADB + RDAP dual sources, BGP relationships, prefixes, batch query
- 📡 **RDAP** — RFC 9083, domain/IP/ASN/Entity, built-in bootstrap
- 🔄 **Reverse WHOIS** — search by email/org/registrant (Provider interface)
- 📋 **Batch** — streaming processor, checkpoint resume, rate limiting, ETA
- 🔗 **Correlation** — cluster by email/registrant/org/NS/registrar, asset profiling
- 👁️ **Monitoring** — expiry & change detection, tiered alerts
- ⭐ **Quality scoring** — completeness/timeliness/reliability, privacy detection (13 rules)
- 💾 **Caching** — local memory & Redis, warmup, hit-rate stats
- 🔒 **Proxy pool** — SOCKS5/HTTP, round-robin, health check, circuit breaker
- ⏱️ **Rate limiting** — token bucket, global + per-server
- 🎛️ **Smart scheduler** — adaptive interval/backoff/concurrency
- 🚨 **Alerting** — CPU/memory/error-rate/failure-rate rules, Email/Slack/Webhook
- 🤖 **MCP protocol** — task planning/execution/approval state machine

### 📦 Three integration interfaces

| Interface | Use case |
|-----------|----------|
| 📦 **Go library** | Import `pkg/whois` as an SDK in your Go program |
| 🌐 **HTTP API** | REST endpoints, any language / web scenarios |
| 🤖 **MCP protocol** | Mission Control task flow, AI Agent scenarios |

### Quick start

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

### 📂 Project structure

```
whois-skills/
├── cmd/whois-hacker/     # 🚀 CLI entry & HTTP service
├── pkg/
│   ├── whois/            # 🔍 Core WHOIS library (24 files)
│   ├── api/              # 🌐 HTTP API service
│   ├── mcp/              # 🤖 MCP protocol service
│   ├── metrics/          # 📈 Metrics & alerting
│   ├── monitor/          # 👁️ Performance monitoring
│   └── security/         # 🔒 API key auth
├── config/               # ⚙️ Config files
├── website/              # 📚 Docs site (VitePress)
├── Dockerfile            # 🐳 Docker build
└── Makefile              # 🔧 Build/test/release
```

### 📚 Documentation

Full documentation site: **https://cyberspacesec.github.io/whois-skills/**

- 📖 [Quick Start](https://cyberspacesec.github.io/whois-skills/guide/getting-started)
- 🏗️ [Architecture](https://cyberspacesec.github.io/whois-skills/guide/architecture)
- 🌐 [HTTP API](https://cyberspacesec.github.io/whois-skills/api/http/overview)
- 🤖 [MCP Protocol](https://cyberspacesec.github.io/whois-skills/api/mcp/overview)
- 🚀 [Deployment](https://cyberspacesec.github.io/whois-skills/deploy/docker)

### 🐳 Docker

```bash
docker pull cyberspacesec/whois-skills:latest
docker run -d -p 8080:8080 -v whois_data:/app/data cyberspacesec/whois-skills:latest
```

### 🤝 Contributing

Contributions are welcome! Please fork, create a branch, and submit a PR with tests.

### 📄 License

[MIT License](LICENSE) © CyberSpaceSec

---

> 🌐 This README is also available in **[简体中文](./README.zh-CN.md)**.
