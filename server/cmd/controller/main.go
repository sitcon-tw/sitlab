package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"example.com/project-template/internal/controller/app"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application, err := app.New(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		return 1
	}
	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		application.Log.Error("shutdown failed", zap.Error(err))
		return 1
	}
	return 0
}
