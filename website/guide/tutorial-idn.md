# 🌍 IDN 教程

> 📖 国际化域名（IDN）的 Punycode 转换与规范化。

---

## 1️⃣ 什么是 IDN

IDN（Internationalized Domain Name）指包含非 ASCII 字符的域名，如 `例.测试`、`münchen.de`。

WHOIS 协议只接受 Punycode 编码（`xn--` 前缀），因此查询前必须转换：

| Unicode | Punycode |
|---------|----------|
| `例.测试` | `xn--fsq.xn--3est` |
| `münchen.de` | `xn--mnchen-3ya.de` |
| `中文.com` | `xn--fiq228c.com` |

---

## 2️⃣ 规范化域名（推荐入口）

`NormalizeDomain` 一步完成所有预处理：

```go
normalized, err := whois.NormalizeDomain("https://例.测试/path?query=1")
// normalized == "xn--fsq.xn--3est"
```

它做了：
1. ✂️ 去除协议前缀（`http://` / `https://`）
2. ✂️ 去除路径（`/path?query=1`）
3. ✂️ 去除尾部点（`example.com.`）
4. 🔡 转小写
5. 🌐 非 ASCII 转 Punycode

```go
result, _ := whois.ExecuteQueryWithResult(&whois.QueryOptions{
	Domain: normalized,
})
```

::: tip 💡 永远先规范化
用户输入的域名可能带协议、路径、大写、Unicode，直接查询会失败。始终先用 `NormalizeDomain`。
:::

---

## 3️⃣ 检测是否为 IDN

```go
whois.IsIDN("xn--fsq.xn--3est")  // true
whois.IsIDN("例.测试")            // true
whois.IsIDN("example.com")        // false
```

`IsIDN` 判断 `xn--` 前缀或包含非 ASCII 字符。

---

## 4️⃣ Punycode 互转

```go
// Unicode → Punycode
puny, _ := whois.UnicodeToPunycode("例.测试")
fmt.Println(puny) // xn--fsq.xn--3est

// Punycode → Unicode
uni, _ := whois.PunycodeToUnicode("xn--fsq.xn--3est")
fmt.Println(uni) // 例.测试
```

底层调用 `golang.org/x/net/idna`。

---

## 5️⃣ 完整查询示例

```go
package main

import (
	"fmt"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

func main() {
	input := "https://中文.公司/about"

	// 1. 规范化
	domain, _ := whois.NormalizeDomain(input)
	fmt.Println("规范化:", domain) // xn--fiq228c.xn--55qx5d

	// 2. 查询
	result, err := whois.ExecuteQueryWithResult(&whois.QueryOptions{
		Domain: domain,
	})
	if err != nil {
		panic(err)
	}

	// 3. 转回 Unicode 显示
	uni, _ := whois.PunycodeToUnicode(domain)
	fmt.Printf("%s 注册商: %s\n", uni, result.Info.Registrar.Name)
}
```

---

## 6️⃣ HTTP API 调用

```bash
# 规范化（默认 action）
curl -X POST http://127.0.0.1:8080/api/idn \
  -H "Content-Type: application/json" \
  -d '{"domain":"https://例.测试/path","action":"normalize"}'
# 返回 {original, result, is_idn, action}

# 转 Punycode
curl -X POST http://127.0.0.1:8080/api/idn \
  -H "Content-Type: application/json" \
  -d '{"domain":"例.测试","action":"to_punycode"}'

# 转 Unicode
curl -X POST http://127.0.0.1:8080/api/idn \
  -H "Content-Type: application/json" \
  -d '{"domain":"xn--fsq.xn--3est","action":"to_unicode"}'

# 仅检测
curl -X POST http://127.0.0.1:8080/api/idn \
  -H "Content-Type: application/json" \
  -d '{"domain":"例.测试","action":"check"}'
```

`action` 支持：`normalize`（默认）/ `to_punycode` / `to_unicode` / `check`。始终返回 `original` 与 `is_idn`。

📖 详见 [IDN 端点](../api/http/endpoint-idn.md)。

---

## ✅ 小结

| 需求 | 方法 |
|------|------|
| 查询前预处理 | `NormalizeDomain` |
| 检测 IDN | `IsIDN` |
| 转 Punycode | `UnicodeToPunycode` |
| 转 Unicode | `PunycodeToUnicode` |

---

## 🔗 相关

- 🌍 [idn.go API](../api/whois/idn.md)
- 🔎 [域名查询教程](./tutorial-domain.md)
- 📡 [IDN 端点](../api/http/endpoint-idn.md)
