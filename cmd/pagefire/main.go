package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pagefire/pagefire/internal/app"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "pagefire",
		Short:   "On-call + monitoring + status pages in a single binary",
		Version: version,
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the PageFire server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			a, err := app.New(cfg)
			if err != nil {
				return fmt.Errorf("initializing app: %w", err)
			}

			return a.Run(context.Background())
		},
	}

	root.AddCommand(serve)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
