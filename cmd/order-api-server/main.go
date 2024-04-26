package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/temporalio/orders-reference-app-go/app/order"
)

var port int

var rootCmd = &cobra.Command{
	Use:   "billing-api-server",
	Short: "API Server for Billing",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		errCh := make(chan error, 1)
		go func() { errCh <- order.RunServer(ctx, port) }()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)

		select {
		case <-sigCh:
			log.Printf("Interrupt signal received, shutting down...")
			cancel()
		case err := <-errCh:
			return err
		}

		return nil
	},
}

func main() {
	rootCmd.Flags().IntVar(&port, "port", 8083, "Port to listen on")

	cobra.CheckErr(rootCmd.Execute())
}