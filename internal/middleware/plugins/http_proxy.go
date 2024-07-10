package plugins

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/timeout"
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
	policiesMap      cmap.ConcurrentMap[string, []failsafe.Policy[any]] // fault tolerant policies by node
	disableEndpoints cmap.ConcurrentMap[string, int64]
}

func NewHttpProxyMiddleware() *HttpProxyMiddleware {
	policiesMap := cmap.New[[]failsafe.Policy[any]]()
	disableEndpoints := cmap.New[int64]()

	return &HttpProxyMiddleware{
		enabled:          true,
		clientRenew:      time.Second * 60,
		mu:               sync.Mutex{},
		policiesMap:      policiesMap,
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

		policies, ok := m.policiesMap.Get(session.NodeName)
		if !ok {
			policies = make([]failsafe.Policy[any], 0)
			// circuit breaker opens after 3 failures, half-opens after 1 minute, closes after 2 successes
			circuitBreaker := circuitbreaker.Builder[any]().
				WithFailureThreshold(3).
				WithDelay(time.Minute).
				WithSuccessThreshold(2).
				Build()
			// timeout after 60 seconds
			timeoutPolicy := timeout.With[any](60 * time.Second)
			policies = append(policies, circuitBreaker, timeoutPolicy)
			m.policiesMap.Set(session.NodeName, policies)
		}

		err := failsafe.Run(func() error {
			return m.GetClient(session).Do(&ctx.Request, &ctx.Response)
		}, policies...)

		// TODO: add response headers
		//if ctx, ok := session.RequestCtx.(*fasthttp.RequestCtx); ok {
		//	ctx.Response.Header.Set("Access-Control-Max-Age", "86400")
		//	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		//	ctx.Response.Header.Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		//	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		//	ctx.Response.Header.Set("X-Do-Node", session.NodeName)
		//}

		var shouldDisableEndpoint = false
		// alert if error
		defer func() {
			if shouldDisableEndpoint {
				now := time.Now().UnixMilli()
				lastTime, ok := m.disableEndpoints.Get(session.NodeName)
				if !ok || now >= lastTime+60*60*1000 { // last alert time is more than 1 hour ago then re-alert
					m.disableEndpoints.Set(session.NodeName, now)
					alert.AlertDiscord(ctx, fmt.Sprintf("disable endpoint: %s, err: %v", session.NodeName, err))
				}
			}
		}()

		if err != nil {
			log.Error(err.Error(), "node", session.NodeName)
			shouldDisableEndpoint = true
			return err
		}

		statusCode := ctx.Response.StatusCode()
		if statusCode/100 != 2 {
			log.Error("error status code", "code", statusCode, "node", session.NodeName)
			err = fmt.Errorf("error status code %d", statusCode)
			shouldDisableEndpoint = true
			return err
		}

		// check response header
		contentType := ctx.Response.Header.Peek("Content-Type")
		if !strings.Contains(string(contentType), "application/json") {
			log.Error("invalid response content type", contentType, "node", session.NodeName)
			err = fmt.Errorf("invalid response content type %s", contentType)
			shouldDisableEndpoint = true
			return err
		}

		return nil
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
