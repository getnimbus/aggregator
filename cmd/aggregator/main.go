package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gogf/gf/v2/os/gfile"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"aggregator/cmd/aggregator/commands"
	"aggregator/internal/env"
)

func main() {
	// load env
	if err := env.LoadConfig("."); err != nil {
		panic(fmt.Errorf("cannot load config: %v", err))
	}

	// start the tracer
	tracer.Start(
		tracer.WithService("rpc-aggregator"),
		tracer.WithEnv(env.Config.Env),
	)
	defer tracer.Stop()

	// set timezone
	time.Local = time.UTC
	initPath()

	app := commands.RootApp()
	err := app.Run(os.Args)
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
