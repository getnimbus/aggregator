package plugins

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/timeout"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/valyala/fasthttp"

	"aggregator/internal/aggregator"
	"aggregator/internal/client"
	"aggregator/internal/loadbalance"
	"aggregator/internal/log"
	"aggregator/internal/middleware"
	"aggregator/internal/rpc"
	"aggregator/pkg/alert"
)

var DISABLED_NODE_STATUS_CODES = []int{401, 403, 502}

type HttpProxyMiddleware struct {
	nextMiddleware   middleware.Middleware
	enabled          bool
	client           *client.Client
	clientCreatedAt  time.Time
	clientRenew      time.Duration
	mu               sync.Mutex
	policiesMap      cmap.ConcurrentMap[string, []failsafe.Policy[any]] // fault tolerant policies by node
	alertEndpoints   cmap.ConcurrentMap[string, int64]
	disableEndpoints cmap.ConcurrentMap[string, int64]
}

func NewHttpProxyMiddleware() *HttpProxyMiddleware {
	policiesMap := cmap.New[[]failsafe.Policy[any]]()
	alertEndpoints := cmap.New[int64]()
	disableEndpoints := cmap.New[int64]()

	return &HttpProxyMiddleware{
		enabled:          true,
		clientRenew:      time.Second * 60,
		mu:               sync.Mutex{},
		policiesMap:      policiesMap,
		alertEndpoints:   alertEndpoints,
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
		log.Debug("relay rpc -> "+session.RpcMethod(), "sid", session.SId(), "node", session.NodeName, "isTx", session.IsWriteRpcMethod, "tries", session.Tries)

		if _, ok := m.disableEndpoints.Get(session.NodeName); ok {
			log.Debug("disabled endpoint", "node", session.NodeName)
			retries := 3
			for {
				if retries == 0 {
					return aggregator.ErrServerError
				}
				node := loadbalance.NextNode(session.Chain)
				if node != nil {
					session.NodeName = node.Name
					ctx.Request.SetRequestURI(node.Endpoint)
					log.Debug("retry to node", "node", node.Name, "endpoint", node.Endpoint)
					break
				}
				retries--
			}
		}

		policies, ok := m.policiesMap.Get(session.NodeName)
		if !ok {
			policies = make([]failsafe.Policy[any], 0)
			// circuit breaker opens after 3 failures, half-opens after 1 minute, closes after 2 successes
			circuitBreaker := circuitbreaker.Builder[any]().
				WithFailureThreshold(3).
				WithDelay(time.Minute).
				WithSuccessThreshold(2).
				Build()
			// timeout after 90 seconds
			timeoutPolicy := timeout.With[any](90 * time.Second)
			policies = append(policies, timeoutPolicy, circuitBreaker)
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
			if shouldDisableEndpoint && err != nil {
				now := time.Now().UnixMilli()
				lastTime, ok := m.alertEndpoints.Get(session.NodeName)
				if !ok || now >= lastTime+60*60*1000 { // last alert time is more than 1 hour ago then re-alert
					m.alertEndpoints.Set(session.NodeName, now)
					alert.AlertDiscord(ctx, fmt.Sprintf("disable endpoint: %s, err: %v", session.NodeName, err))
				}
			}
		}()

		if err != nil {
			log.Error(err.Error(), "node", session.NodeName)
			err = fmt.Errorf("request error %v", err)
			shouldDisableEndpoint = true
			ctx.SetStatusCode(500)
			return err
		}

		statusCode := ctx.Response.StatusCode()
		// block nodes that return invalid status code
		if slices.Contains(DISABLED_NODE_STATUS_CODES, statusCode) {
			now := time.Now().UnixMilli()
			_, ok := m.disableEndpoints.Get(session.NodeName)
			if !ok {
				m.disableEndpoints.Set(session.NodeName, now)
			}
		}
		if statusCode/100 != 2 || slices.Contains(DISABLED_NODE_STATUS_CODES, statusCode) || statusCode == 429 {
			log.Error("error status code", "code", statusCode, "node", session.NodeName)
			err = fmt.Errorf("error status code %d", statusCode)
			//shouldDisableEndpoint = true
			ctx.SetStatusCode(statusCode)
			return err
		}

		// check response header
		contentType := ctx.Response.Header.Peek("Content-Type")
		if !strings.Contains(string(contentType), "application/json") {
			log.Error("invalid response content type", contentType, "node", session.NodeName)
			err = fmt.Errorf("invalid response content type %s", contentType)
			shouldDisableEndpoint = true
			now := time.Now().UnixMilli()
			_, ok := m.disableEndpoints.Get(session.NodeName)
			if !ok {
				m.disableEndpoints.Set(session.NodeName, now)
			}
			ctx.SetStatusCode(500)
			return err
		}

		// check response body
		var response map[string]interface{}
		if err1 := json.Unmarshal(ctx.Response.Body(), &response); err1 == nil {
			if _, ok := response["error"]; ok {
				log.Error("error response", "node", session.NodeName, "response", string(ctx.Response.Body()))
				err = fmt.Errorf("error response %s", string(ctx.Response.Body()))
				shouldDisableEndpoint = true
				ctx.SetStatusCode(400)
				return err
			}
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
