package plugins

import (
	"fmt"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/valyala/fasthttp"

	"aggregator/internal/client"
	"aggregator/internal/log"
	"aggregator/internal/middleware"
	"aggregator/internal/rpc"
	"aggregator/pkg/alert"
)

type HttpProxyMiddleware struct {
	nextMiddleware   middleware.Middleware
	enabled          bool
	client           *client.Client
	clientCreatedAt  time.Time
	clientRenew      time.Duration
	mu               sync.Mutex
	cbs              cmap.ConcurrentMap[string, circuitbreaker.CircuitBreaker[any]]
	disableEndpoints cmap.ConcurrentMap[string, int64]
}

func NewHttpProxyMiddleware() *HttpProxyMiddleware {
	cbs := cmap.New[circuitbreaker.CircuitBreaker[any]]()
	disableEndpoints := cmap.New[int64]()

	return &HttpProxyMiddleware{
		enabled:          true,
		clientRenew:      time.Second * 60,
		mu:               sync.Mutex{},
		cbs:              cbs,
		disableEndpoints: disableEndpoints,
	}
}

func (m *HttpProxyMiddleware) Name() string {
	return "HttpProxyMiddleware"
}

func (m *HttpProxyMiddleware) Enabled() bool {
	return m.enabled
}

func (m *HttpProxyMiddleware) Next() middleware.Middleware {
	return m.nextMiddleware
}

func (m *HttpProxyMiddleware) SetNext(middleware middleware.Middleware) {
	m.nextMiddleware = middleware
}

func (m *HttpProxyMiddleware) OnRequest(session *rpc.Session) error {
	return nil
}

func (m *HttpProxyMiddleware) OnProcess(session *rpc.Session) error {
	if ctx, ok := session.RequestCtx.(*fasthttp.RequestCtx); ok {
		logger.Debug("relay rpc -> "+session.RpcMethod(), "sid", session.SId(), "node", session.NodeName, "isTx", session.IsWriteRpcMethod, "tries", session.Tries)

		// circuit breaker opens after 5 failures, half-opens after 1 minute, closes after 2 successes
		cb, ok := m.cbs.Get(session.NodeName)
		if !ok {
			cb = circuitbreaker.Builder[any]().
				WithFailureThreshold(5).
				WithDelay(time.Minute).
				WithSuccessThreshold(2).
				Build()
			m.cbs.Set(session.NodeName, cb)
		}
		err := failsafe.Run(func() error {
			return m.GetClient(session).Do(&ctx.Request, &ctx.Response)
		}, cb)

		// TODO: add response headers
		//if ctx, ok := session.RequestCtx.(*fasthttp.RequestCtx); ok {
		//	ctx.Response.Header.Set("Access-Control-Max-Age", "86400")
		//	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		//	ctx.Response.Header.Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		//	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		//	ctx.Response.Header.Set("X-Do-Node", session.NodeName)
		//}

		shouldDisableEndpoint := false
		if err != nil {
			log.Error(err.Error(), "node", session.NodeName)
			shouldDisableEndpoint = true
		}

		statusCode := ctx.Response.StatusCode()
		if statusCode/100 != 2 {
			log.Error("error status code", "code", statusCode, "node", session.NodeName)
			err = fmt.Errorf("error status code %d - node %s", statusCode, session.NodeName)
			shouldDisableEndpoint = true
		}

		if shouldDisableEndpoint {
			// TODO: disable endpoint
			now := time.Now().UnixMilli()
			lastTime, ok := m.disableEndpoints.Get(session.NodeName)
			if !ok || now >= lastTime+60*60*1000 { // last alert time is more than 1 hour ago then re-alert
				m.disableEndpoints.Set(session.NodeName, now)
				alert.AlertDiscord(ctx, fmt.Sprintf("disable endpoint %s - status code %d - err %v", session.NodeName, statusCode, err))
			}
		}

		return err
	}

	return nil
}

func (m *HttpProxyMiddleware) OnResponse(session *rpc.Session) error {
	return nil
}

func (m *HttpProxyMiddleware) GetClient(session *rpc.Session) *client.Client {
	m.mu.Lock()
	defer m.mu.Unlock()

	if time.Since(m.clientCreatedAt) <= m.clientRenew {
		if m.client != nil {
			return m.client
		}
	}

	log.Debug("renew proxy http client")
	m.client = client.NewClient(session.Cfg.RequestTimeout, session.Cfg.Proxy)
	m.clientCreatedAt = time.Now()

	return m.client
}
