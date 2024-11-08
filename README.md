

# Whois Package README

## Overview

The `whois` package is designed to query WHOIS information for a given domain name. It handles the intricacies of domain resolution, including punycode encoding for internationalized domain names, and provides a robust mechanism for handling errors and retries.

## Installation

To use this package, ensure you have the following dependencies installed:

- `github.com/cyberspacesec/go-domain-util`
- `github.com/likexian/whois`
- `github.com/likexian/whois-parser`
- `golang.org/x/net/idna`

You can install these packages using `go get`:

```bash
go get github.com/cyberspacesec/go-domain-util
go get github.com/likexian/whois
go get github.com/likexian/whois-parser
go get golang.org/x/net/idna
```

## Usage

### Query Struct

The `Query` struct is used to configure the parameters for the WHOIS query.

- `Domain`: The domain name to query.
- `IntervalMils`: The interval in milliseconds to wait between retries. Defaults to 1000ms if not set.

### Execute Function

The `Execute` function performs the WHOIS query and returns a `*whoisparser.WhoisInfo` struct containing the parsed WHOIS information or an error if the query fails.

```go
package main

import (
	"fmt"
	"your_package_path/whois" // Replace with the actual path to the whois package
)

func main() {
	query := &whois.Query{
		Domain: "example.com",
	}
	info, err := whois.Execute(query)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Printf("WHOIS Info: %+v\n", info)
	}
}
```

### Error Handling

The `Execute` function includes error handling for various scenarios:

- Connection issues will be retried up to 5 times.
- If the error is due to querying too frequently, it will wait for the specified interval before retrying.

### TODOs

There are a few TODOs in the code that you might want to address:

1. Error handling for punycode conversion.
2. Consider adding more sophisticated error handling and retry logic based on the specific error types.

## Contributing

Contributions to this package are welcome. Please ensure that your contributions adhere to the existing code style and include appropriate tests.

## License

This package is released under the [MIT License](LICENSE).




目标：
1. icp新增的域名能够立刻解析whois信息
2. icp的域名的whois信息能够定期更新一遍（比如一周一次？）
3. 在条件允许的情况下，把之前的whois表整个跑一遍
4. 能够提供api供其它服务推送任务 


# whois服务器列表 
https://github.com/whois-server-list/whois-server-list




```text
Domain ID:D556987-MOBI        ‘域名在域名库中的ID编号
Domain Name:SHUXIANG.MOBI       ‘域名
Created On:23-Oct-2006 12:54:26 UTC        ‘域名创建时间，即域名首次注册时间
Last Updated On:23-Oct-2006 12:54:27 UTC    ‘域名最后一次更新时间，域名注册生效时间，或者域名续费的时间；
Expiration Date:23-Oct-2008 12:54:26 UTC    ‘域名到期时间
Sponsoring Registrar:Beijing Innovative Linkage Technology Ltd dba dns.com.cn (633)  ‘域名由哪家注册机构提起注册
Created by Registrar:Beijing Innovative Linkage Technology Ltd dba dns.com.cn (633)  ’域名被哪家注册机构注册
Last Updated by Registrar:Beijing Innovative Linkage Technology Ltd dba dns.com.cn (633) ’域名最后注册生效的机构
Status:CLIENT TRANSFER PROHIBITED    ‘域名当前状态1：转移锁定
Status:TRANSFER PROHIBITED  ’域名当前状态2：转移锁定
Registrant ID:CTMQPGDWU397O3C   ‘登记者ID号 
Registrant Name:shuxiang   ’注册人名称
Registrant Organization:fuzhou shuxiang network technology co.,ltd     ‘注册人单位名称
Registrant Street1:fuzhou shuxiang network technology co.,ltd          ‘注册人地址 
Registrant City:Fuzhou          ‘注册人所在城市
Registrant State/Province:FJ    ‘注册人所在省份
Registrant Postal Code:350005   ‘邮政编码
Registrant Country:CN           ’所在城市
Registrant Phone:+86.59128350600     ’注册人联系电话
Registrant FAX:+86.59128350800       ’注册人传真号码
Registrant Email:abcd@shuxiang.org       ’注册人邮箱地址
Admin ID:CTOLU7EWEH01J4Y             ‘域名管理人ID
Admin Name:maofeng Huang             ‘域名管理人姓名
Admin Organization:fuzhou shuxiang network technology co.,ltd     ‘域名管理人单位名称
Admin Street1:fuzhou shuxiang network technology co.,ltd          ‘域名管理人街道地址
Admin City:Fuzhou           ‘域名管理人所在城市
Admin State/Province:FJ       ‘域名管理人所在省份
Admin Postal Code:350005       ‘域名管理人的邮政编码
Admin Country:CN             ‘域名管理人所在国家
Admin Phone:+86.59128350600      ‘域名管理人的联系电话
Admin FAX:+86.59128350800        ‘域名管理人传真号码 
Admin Email:sx@shuxiang.org        ‘域名管理人邮箱地址
Tech ID:CT6SDQ7KVFPFK3B          ‘域名技术支持ID号
Tech Name:lehui zheng            ‘域名技术支持联系人
Tech Organization:fuzhou shuxiang network technology co.,ltd    ‘域名技术支持单位名称
Tech Street1:fuzhou shuxiang network technology co.,ltd         ‘域名技术支持人所在地址
Tech City:fuzhou      ‘域名技术支持人所在城市
Tech State/Province:FJ     ‘域名技术支持人所在省份
Tech Postal Code:350005    ‘域名技术支持人的邮政编码
Tech Country:CN     ’技术支持人所在国家
Tech Phone:+86.59187794618     ’技术支持的联系电话
Tech FAX:+86.59128350802       ‘技术支持的传真号码
Tech Email:163@shuxiang.org       ‘技术支持的邮箱地址
Name Server:NS1.DNS.COM.CN     ‘域名的解析服务器1
Name Server:NS2.DNS.COM.CN     ‘域名的解析服务器2
```





