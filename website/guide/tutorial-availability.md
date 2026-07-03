# ✅ 域名可用性教程

> 📖 检测域名是否可注册，区分 available/registered/reserved/premium/blocked。

---

## 1️⃣ 可用性状态

`CheckDomainAvailability` 返回 6 种状态：

| 状态 | 含义 | 是否可注册 |
|------|------|-----------|
| 🟢 `available` | 域名可注册 | ✅ 是 |
| 🔵 `registered` | 域名已注册 | ❌ 否 |
| 🟡 `reserved` | 域名被保留 | ❌ 否 |
| 🟠 `premium` | 域名为溢价域名 | ⚠️ 需高价 |
| 🔴 `blocked` | 域名被屏蔽 | ❌ 否 |
| ⚪ `rate_limited` | 触发限速，稍后再试 | ❓ 未知 |
| ⚪ `unknown` | 无法判断 | ❓ 未知 |

---

## 2️⃣ 基础检测

```go
package main

import (
	"fmt"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

func main() {
	avail, err := whois.CheckDomainAvailability("some-likely-free-name-xyz123.com")
	if err != nil {
		fmt.Println("检测出错:", err)
		return
	}

	fmt.Printf("域名: %s\n", avail.Domain)
	fmt.Printf("可注册: %v\n", avail.Available)
	fmt.Printf("状态: %s\n", avail.Status)
	fmt.Printf("说明: %s\n", avail.Message)

	if avail.Available {
		fmt.Println("🎉 该域名可注册！")
	}
}
```

---

## 3️⃣ Context 版本

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

avail, err := whois.CheckDomainAvailabilityWithContext(ctx, "example.com")
```

---

## 4️⃣ 批量检测可注册域名

```go
candidates := []string{
	"my-new-idea-123.com",
	"another-name-xyz.org",
	"some-test-name-789.io",
}

var available []string
for _, d := range candidates {
	avail, err := whois.CheckDomainAvailability(d)
	if err != nil {
		continue
	}
	if avail.Available {
		available = append(available, d)
		fmt.Printf("🟢 %s 可注册\n", d)
	} else {
		fmt.Printf("🔵 %s 已被占用 (%s)\n", d, avail.Status)
	}
}
fmt.Println("可注册域名:", available)
```

::: tip ⏱️ 批量限速
批量检测时建议加域间延迟，避免触发注册局限速（返回 `rate_limited`）。可结合 [智能调度](../api/whois/scheduler.md) 或 [批量处理器](../api/whois/batch.md)。
:::

---

## 5️⃣ 状态判断逻辑

`CheckDomainAvailability` 内部调用 `ExecuteQueryWithContext`，按解析器错误分类：

| 错误类型 | 状态 |
|---------|------|
| `ErrNotFoundDomain` | `available` |
| `ErrReservedDomain` | `reserved` |
| `ErrPremiumDomain` | `premium` |
| `ErrBlockedDomain` | `blocked` |
| `ErrDomainLimitExceed` | `rate_limited` |
| 成功获取 `info.Domain` | `registered` |

::: details 🔍 实现细节
错误分类通过比较 `err.Error()` 字符串匹配（非 `errors.Is`），因为 `whoisparser` 的错误类型有限。详见 [errors.go](../api/whois/errors.md)。
:::

---

## 6️⃣ HTTP API 调用

```bash
curl -X POST http://127.0.0.1:8080/api/availability \
  -H "Content-Type: application/json" \
  -d '{"domain":"some-likely-free-name-xyz123.com"}'
```

响应：

```json
{
  "success": true,
  "data": {
    "domain": "some-likely-free-name-xyz123.com",
    "available": true,
    "status": "available",
    "message": "域名可注册"
  }
}
```

📖 详见 [可用性端点](../api/http/endpoint-availability.md)。

---

## 7️⃣ IDN 域名可用性

国际化域名先规范化再检测：

```go
normalized, _ := whois.NormalizeDomain("我的想法.公司")
avail, _ := whois.CheckDomainAvailability(normalized)
```

📖 详见 [IDN 教程](./tutorial-idn.md)。

---

## ✅ 小结

| 需求 | 方法 |
|------|------|
| 单域名检测 | `CheckDomainAvailability` |
| 带超时 | `CheckDomainAvailabilityWithContext` |
| 批量筛选 | 循环 + 限速 |
| HTTP | `POST /api/availability` |

---

## 🔗 相关

- ✅ [availability.go API](../api/whois/availability.md)
- ❌ [errors.go](../api/whois/errors.md)
- 🌍 [IDN 教程](./tutorial-idn.md)
