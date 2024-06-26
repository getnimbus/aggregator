package plugins

import (
	"strings"

	"github.com/valyala/fasthttp"

	"aggregator/internal/aggregator"
	"aggregator/internal/middleware"
	"aggregator/internal/rpc"
)

var (
	defaultWriteMethods = []string{
		// for EVM
		strings.ToLower("_call"),
		strings.ToLower("_sendRawTransaction"),
		strings.ToLower("_sendTransaction"),
		strings.ToLower("_sendTransactionAsFeePayer"),
		// for SUI
		strings.ToLower("sui_devInspectTransactionBlock"),
		strings.ToLower("sui_dryRunTransactionBlock"),
		strings.ToLower("sui_executeTransactionBlock"),
	}
)

type RequestValidatorMiddleware struct {
	nextMiddleware middleware.Middleware
	enabled        bool
}

func NewRequestValidatorMiddleware() *RequestValidatorMiddleware {
	return &RequestValidatorMiddleware{enabled: true}
}

func (m *RequestValidatorMiddleware) Name() string {
	return "RequestValidatorMiddleware"
}

func (m *RequestValidatorMiddleware) Enabled() bool {
	return m.enabled
}

func (m *RequestValidatorMiddleware) Next() middleware.Middleware {
	return m.nextMiddleware
}

func (m *RequestValidatorMiddleware) SetNext(middleware middleware.Middleware) {
	m.nextMiddleware = middleware
}

func (m *RequestValidatorMiddleware) OnRequest(session *rpc.Session) error {
	if session.Method == "OPTIONS" {
		return aggregator.ErrMustReturn
	}

	//if session.Method != "POST" {
	//	return aggregator.ErrInvalidMethod
	//}

	if ctx, ok := session.RequestCtx.(*fasthttp.RequestCtx); ok {
		ctx.Request.Header.Set("Accept", "application/json")
	}

	session.IsWriteRpcMethod = m.isWriteMethod(session.RpcMethod())

	return nil
}

func (m *RequestValidatorMiddleware) OnProcess(session *rpc.Session) error {
	if session.Method == "OPTIONS" {
		//if ctx, ok := session.RequestCtx.(*fasthttp.RequestCtx); ok {
		//
		//}

		return aggregator.ErrMustReturn
	}
	return nil
}

func (m *RequestValidatorMiddleware) OnResponse(session *rpc.Session) error {
	return nil
}

func (m *RequestValidatorMiddleware) isWriteMethod(method string) bool {
	if len(method) > 0 {
		method := strings.ToLower(method)
		for _, m := range defaultWriteMethods {
			if strings.HasSuffix(method, m) {
				return true
			}
		}
	}
	return false
}
