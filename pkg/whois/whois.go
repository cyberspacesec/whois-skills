package whois

import (
	domain_util "github.com/cyberspacesec/go-domain-util"
	"github.com/likexian/whois"
	"time"
)

import (
	whoisparser "github.com/likexian/whois-parser"
	"golang.org/x/net/idna"
)

// Query 查询参数
type Query struct {

	// 域名是哪个
	Domain string `json:"domain,omitempty"`

	IntervalMils int `json:"interval_mils,omitempty"`
}

func (x *Query) GetIntervalMilsOrDefault() int {
	if x.IntervalMils == 0 {
		x.IntervalMils = 1000
	}
	return x.IntervalMils
}

// Execute 执行根域名
func Execute(query *Query) (*whoisparser.WhoisInfo, error) {

	// 兼容传进来的域名不是一个根域名的情况，会先自动解析域名所属的根域名
	fldDomain, err := domain_util.FldDomain(query.Domain)
	if err != nil {
		return nil, err
	}

	// 查询之前需要punycode编码，否则可能会查询不到
	domainForQuery, err := idna.ToASCII(fldDomain)
	if err != nil {
		domainForQuery = fldDomain
	} else {
		// TODO 2024-11-09 01:07:43 需要做一些错误处理吗
	}

	// 向whois服务器发出查询
	var whoisResponse string
	for tryTimes := 1; tryTimes <= 5; tryTimes++ {

		whoisResponse, err = whois.Whois(domainForQuery)
		if err != nil {

			// TODO 也许在被ban的时候直接将其丢到内存队列可能会更快一些
			// 如果是因为连接问题的话则重试
			checkError := CheckError(err)
			switch checkError.ErrorType {
			case QueriedIntervalTooShort, AccessTooFastPleaseTryAgainLater:
				// 请求太快了
				time.Sleep(time.Millisecond * time.Duration(query.IntervalMils))
				break
			case ConnectionResetByPeer, ConnectToWhoisServerFailed:
				// 服务器连接不通，不需要休眠
			default:
				//
			}
		}
	}
	if err != nil {
		return nil, err
	}
	info, err := whoisparser.Parse(whoisResponse)
	if err != nil {
		return nil, err
	} else {
		return &info, nil
	}
}
