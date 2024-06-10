package commands

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

	"aggregator/internal/config"
	"aggregator/internal/env"
	"aggregator/internal/loadbalance"
	"aggregator/internal/middleware"
	"aggregator/internal/middleware/plugins"
	"aggregator/internal/server"
)

func RunCommand() *cli.Command {
	return &cli.Command{
		Name:    "run",
		Aliases: []string{"start"},
		Flags:   append([]cli.Flag{}, InitCommand().Flags...),
		Before: func(cli *cli.Context) error {
			err := runCommand(cli, "init")
			if err != nil {
				return err
			}

			config.Load()

			loadbalance.LoadFromConfig()

			middleware.Append(
				plugins.NewRequestValidatorMiddleware(),
				plugins.NewSafetyMiddleware(),
				plugins.NewLoadBalanceMiddleware(),
				plugins.NewHttpProxyMiddleware(),
				plugins.NewCorsMiddleware(),
			)

			return nil
		},
		Action: func(context *cli.Context) error {
			if err := env.LoadConfig("."); err != nil {
				panic(fmt.Errorf("cannot load config: %v", err))
			}

			wg := errgroup.Group{}
			wg.Go(func() error {
				return server.NewManageServer()
			})
			wg.Go(func() error {
				return server.NewServer()
			})
			return wg.Wait()
		},
		Subcommands: []*cli.Command{},
	}

}
