package client

import (
	"crypto/tls"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

type Client struct {
	client     fasthttp.Client
	timeout    int64
	maxRetries int64
	proxy      string
}

func DefaultClient() *Client {
	return NewClient(30, "")
}

func NewClient(timeout int64, proxy string) *Client {
	cli := &Client{
		client: fasthttp.Client{
			MaxConnsPerHost: 65000,
			TLSConfig:       &tls.Config{InsecureSkipVerify: true}, // only use for read-only rpc, if rpc is used for write transactions, please remove this line
			//Dial: func(addr string) (net.Conn, error) {
			//	return nil, nil
			//},
		},
		timeout: timeout,
		proxy:   proxy,
	}
	if proxy != "" {
		if strings.HasPrefix(proxy, "socks5://") || strings.HasPrefix(proxy, "socks5h://") {
			cli.client.Dial = fasthttpproxy.FasthttpSocksDialer(proxy)
		} else {
			cli.client.Dial = fasthttpproxy.FasthttpHTTPDialer(proxy)
		}
	}
	return cli
}

func (cli *Client) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	req.Header.Del("Connection")
	defer resp.Header.Del("Connection")
	return cli.client.DoTimeout(req, resp, time.Second*time.Duration(cli.timeout))
}
