package server

import (
	"encoding/json"
	"net/http"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"aggregator/internal/config"
	"aggregator/internal/env"
	"aggregator/internal/loadbalance"
	"aggregator/internal/notify"
)

func rootHandler(ctx *fasthttp.RequestCtx) {
	ctx.WriteString("hello!")
}

func statusHandler(ctx *fasthttp.RequestCtx) {
	st := map[string]any{}
	st["status"] = "ok"
	data, _ := json.Marshal(st)
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Write(data)
}

func routeConfigHandler(ctx *fasthttp.RequestCtx) {
	data, _ := json.Marshal(config.Default())
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Write(data)
}

func routeUpdateConfigHandler(ctx *fasthttp.RequestCtx) {
	cfg := config.Config{}
	err := json.Unmarshal(ctx.Request.Body(), &cfg)
	if err != nil {
		ctx.Error("error parse config", fasthttp.StatusInternalServerError)
		return
	}

	defaultCfg := config.Default()

	dbs := defaultCfg.AuthorityDB
	for i := 0; i < len(dbs); i++ {
		for _, adb2 := range cfg.AuthorityDB {
			if dbs[i].Name == adb2.Name {
				dbs[i].Enable = adb2.Enable
			}
		}
	}

	cfg.AuthorityDB = dbs

	config.SetDefault(&cfg)
	loadbalance.LoadFromConfig()

	config.Save()

	data, _ := json.Marshal(cfg)
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Write(data)
}

func routeRestoreConfigHandler(ctx *fasthttp.RequestCtx) {
	config.LoadDefault()

}

func NewManageServer() error {
	r := router.New()
	r.PanicHandler = func(ctx *fasthttp.RequestCtx, err interface{}) {
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
	}

	r.GET("/", rootHandler)
	r.GET("/status", statusHandler)
	r.GET("/config", routeConfigHandler)
	r.POST("/config", routeUpdateConfigHandler)
	r.POST("/config/restore", routeRestoreConfigHandler)

	addr := ":8012"
	logger.Info("Starting management server", "addr", addr)
	server := fasthttp.Server{
		Name: "",
		Handler: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Token, Authorization")
			ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")

			if string(ctx.Method()) == "OPTIONS" {
				ctx.Response.Header.Set("Access-Control-Max-Age", "86400")
				ctx.SetStatusCode(http.StatusOK)
				ctx.SetBodyString("ok")
				return
			}
			path := string(ctx.Request.URI().Path())
			if path == "/status" {
				r.Handler(ctx)
				return
			}

			auth := ctx.Request.Header.Peek("Authorization")
			if string(auth) == env.Config.ApiKey {
				r.Handler(ctx)
				return
			}
			ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
		},
	}
	err := server.ListenAndServe(addr)
	if err != nil {
		notify.SendError("Error start manage server.", err.Error())
		return err
	}
	return nil
}
