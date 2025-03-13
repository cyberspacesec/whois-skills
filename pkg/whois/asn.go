package whois

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

func GetIPRangesByASN(asn string) ([]string, []string, error) {
	// 连接WHOIS服务器
	conn, err := net.Dial("tcp", "whois.radb.net:43")
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	// 发送查询命令
	query := fmt.Sprintf("!g%s\n", asn)
	_, err = conn.Write([]byte(query))
	if err != nil {
		return nil, nil, err
	}

	// 读取响应
	scanner := bufio.NewScanner(conn)
	var ipv4, ipv6 []string

	for scanner.Scan() {
		line := scanner.Text()
		// 提取IPv4网段（格式：route: 58.154.0.0/15）
		if strings.HasPrefix(line, "route:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ipv4 = append(ipv4, fields[1])
			}
		}
		// 提取IPv6网段（格式：route6: 2001:da8::/32）
		if strings.HasPrefix(line, "route6:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ipv6 = append(ipv6, fields[1])
			}
		}
	}

	return ipv4, ipv6, nil
}
