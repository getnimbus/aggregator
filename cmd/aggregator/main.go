package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gogf/gf/v2/os/gfile"
	"github.com/hyperdxio/opentelemetry-go/otelzap"
	"github.com/hyperdxio/opentelemetry-logs-go/exporters/otlp/otlplogs"
	sdk "github.com/hyperdxio/opentelemetry-logs-go/sdk/logs"
	"github.com/hyperdxio/otel-config-go/otelconfig"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"aggregator/cmd/aggregator/commands"
	"aggregator/internal/entity"
	"aggregator/internal/env"
)

func main() {
	// start the tracer
	tracer.Start()
	defer tracer.Stop()

	// load env
	if err := env.LoadConfig("."); err != nil {
		panic(fmt.Errorf("cannot load config: %v", err))
	}

	println(entity.Version)

	time.Local = time.UTC
	initPath()

	// Initialize otel config and use it across the entire app
	otelShutdown, err := otelconfig.ConfigureOpenTelemetry()
	if err != nil {
		log.Fatalf("error setting up OTel SDK - %e", err)
	}
	defer otelShutdown()

	ctx := context.Background()

	// configure opentelemetry logger provider
	logExporter, _ := otlplogs.NewExporter(ctx)
	loggerProvider := sdk.NewLoggerProvider(
		sdk.WithBatcher(logExporter),
	)
	// gracefully shutdown logger to flush accumulated signals before program finish
	defer loggerProvider.Shutdown(ctx)

	// create new logger with opentelemetry zap core and set it globally
	logger := zap.New(otelzap.NewOtelCore(loggerProvider))
	zap.ReplaceGlobals(logger)

	app := commands.RootApp()
	err = app.Run(os.Args)
	if err != nil {
		panic(err)
	}

}

func initPath() {
	dir, err := gfile.Home(".rpchub/aggregator")
	if err != nil {
		panic(err)
	}

	if !gfile.Exists(dir) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			panic(err)
		}
	}

	if !gfile.IsDir(dir) {
		panic(errors.New(fmt.Sprintf("%s is not a dir", dir)))
	}

	err = os.Chdir(dir)
	if err != nil {
		panic(err)
	}
}
