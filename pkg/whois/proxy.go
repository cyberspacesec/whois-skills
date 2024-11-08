package whois

//import (
//	"github.com/jcmturner/gokrb5/v8/config"
//	"github.com/likexian/whois"
//	"golang.org/x/net/proxy"
//	"net"
//	"time"
//)
//
//type Socket5ProxyDialer struct {
//
//}
//
//type ProxyConfig struct {
//
//}
//
//// 设置whois查询的时候使用代理，这样的话就不用担心被ban了
//func setWhoisProxy() error {
//	proxyDialer, err := proxy.SOCKS5("tcp", config.Config.GetString("whois.proxy.address"),
//		&proxy.Auth{User: config.Config.GetString("whois.proxy.username"), Password: config.Config.GetString("whois.proxy.passwd")},
//		&net.Dialer{
//			Timeout:   30 * time.Second,
//			KeepAlive: 30 * time.Second,
//		},
//	)
//	if err != nil {
//		log.Errorf("初始化whois代理时创建dialer失败：%s", err.Error())
//		return err
//	}
//	whois.DefaultClient.SetDialer(proxyDialer)
//	return nil
//}

