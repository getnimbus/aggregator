package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gogf/gf/v2/os/gfile"

	"aggregator/cmd/aggregator/commands"
	"aggregator/internal/entity"
	"aggregator/internal/env"
)

func main() {
	// load env
	if err := env.LoadConfig("."); err != nil {
		panic(fmt.Errorf("cannot load config: %v", err))
	}

	println(entity.Version)

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
