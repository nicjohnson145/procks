package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nicjohnson145/procks/internal/client"
	"github.com/nicjohnson145/procks/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	if err := root().Execute(); err != nil {
		os.Exit(1)
	}
}

func root() *cobra.Command {
	cobra.EnableTraverseRunHooks = true

	var port string

	cmd := &cobra.Command{
		Use:   "procks [ID?]",
		Short: "procks client",
		Args:  cobra.RangeArgs(0, 1),
		Long:  "client for configuring temporary development proxies",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.SilenceUsage = true
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client.InitConfig(); err != nil {
				fmt.Printf("error initializing config: %v\n", err)
				return err
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			logger := logging.Init(&logging.LoggingConfig{
				Level:  logging.LogLevel(viper.GetString(client.LogLevel)),
				Format: logging.LogFormat("human"),
			})

			url := viper.GetString(client.ServerUrl)
			if url == "" {
				logger.Error().Msg("url not configured")
				return fmt.Errorf("url not configured")
			}

			procksClient := client.NewClient(client.ClientConfig{
				Logger: logger,
				Url:    url,
			})

			opts := client.ProxyOpts{
				Port: port,
			}
			if len(args) > 0 {
				opts.ID = args[0]
			}

			if err := procksClient.Proxy(ctx, opts); err != nil {
				logger.Err(err).Msg("error proxying")
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&port, "port", "p", "3000", "what port to proxy requests to on localhost")

	return cmd
}
