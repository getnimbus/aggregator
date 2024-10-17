package server

import (
	"time"

	libredis "github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	mfasthttp "github.com/ulule/limiter/v3/drivers/middleware/fasthttp"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
	"github.com/valyala/fasthttp"
	fasthttptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/valyala/fasthttp.v1"

	"aggregator/internal/config"
	"aggregator/internal/env"
	"aggregator/internal/log"
	"aggregator/internal/middleware"
	"aggregator/internal/rpc"
)

var (
	logger = log.Module("server")
)

var requestHandler = func(ctx *fasthttp.RequestCtx) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("error", "msg", err)
		}
	}()

	// TODO: may need check api key if public RPC to users

	var err error

	session := &rpc.Session{RequestCtx: ctx}
	err = session.Init()
	if err != nil {
		ctx.Error(string(session.NewJsonRpcError(err).Marshal()), fasthttp.StatusOK)
		return
	}
	for {
		session.Tries++
		err = middleware.OnRequest(session)
		if err != nil {
			if session.IsMaxRetriesExceeded() {
				ctx.Error(string(session.NewJsonRpcError(err).Marshal()), fasthttp.StatusOK)
				return
			}
			continue
		}

		err = middleware.OnProcess(session)
		if err != nil {
			if session.IsMaxRetriesExceeded() {
				ctx.Error(string(session.NewJsonRpcError(err).Marshal()), fasthttp.StatusOK)
				return
			}
			continue
		}

		err = middleware.OnResponse(session)
		if err != nil {
			if session.IsMaxRetriesExceeded() {
				ctx.Error(string(session.NewJsonRpcError(err).Marshal()), fasthttp.StatusOK)
				return
			}
			continue
		}
		return
	}
}

func NewRateLimiter() *mfasthttp.Middleware {
	// define a limit rate to 20 reqs/s
	rate := limiter.Rate{
		Period: 1 * time.Second,
		Limit:  20,
	}

	// create redis client
	option, err := libredis.ParseURL(env.Config.RedisUrl)
	if err != nil {
		panic(err)
	}
	client := libredis.NewClient(option)

	// create a store with the redis client
	store, err := sredis.NewStoreWithOptions(client, limiter.StoreOptions{
		Prefix:   "aggregator_limiter",
		MaxRetry: 3,
	})
	if err != nil {
		panic(err)
	}

	return mfasthttp.NewMiddleware(limiter.New(store, rate, limiter.WithTrustForwardHeader(true)))
}

// RateLimitMiddleware is a middleware to limit the rate of requests
func RateLimitMiddleware(rateLimiter *mfasthttp.Middleware, next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		auth := ctx.QueryArgs().Peek("key")
		if len(auth) == 0 {
			auth = ctx.Request.Header.Peek("X-Api-Key")
		}
		if len(auth) > 0 && string(auth) == env.Config.ApiKey {
			next(ctx)
			return
		}

		rateLimiter.Handle(next)(ctx)
	}
}

func NewServer() error {
	var err error
	addr := ":8011"
	logger.Info("Starting proxy server", "addr", addr)

	for _, chain := range config.Chains() {
		logger.Info("Registered RPC", "endpoint", "http://localhost:8011/"+chain)
	}

	handler := fasthttp.CompressHandlerLevel(requestHandler, fasthttp.CompressDefaultCompression)
	if env.Config.IsRateLimit() {
		rateLimiter := NewRateLimiter()
		handler = RateLimitMiddleware(rateLimiter, handler)
	}
	s := &fasthttp.Server{
		Handler:            fasthttptrace.WrapHandler(handler),
		MaxRequestBodySize: fasthttp.DefaultMaxRequestBodySize * 10,
	}

	err = s.ListenAndServe(addr)
	if err != nil {
		return err
	}
	return nil
}
