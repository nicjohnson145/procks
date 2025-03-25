package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/grpcreflect"
	"github.com/nicjohnson145/procks/gen/procks/v1/procksv1connect"
	"github.com/nicjohnson145/procks/internal/logging"
	"github.com/nicjohnson145/procks/internal/server"
	"github.com/spf13/viper"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	server.InitConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Reflection
	reflector := grpcreflect.NewStaticReflector(
		procksv1connect.ProcksServiceName,
	)

	logger := logging.Init(&logging.LoggingConfig{
		Level:  logging.LogLevel(viper.GetString(server.LogLevel)),
		Format: logging.LogFormat(viper.GetString(server.LogFormat)),
	})

	url := viper.GetString(server.Url)
	if url == "" {
		logger.Error().Msg("url not set, cannot continue")
		return fmt.Errorf("url is required")
	}

	srv := server.NewServer(server.ServerConfig{
		Logger: logger,
		Url:    url,
	})

	mux := http.NewServeMux()
	mux.Handle(procksv1connect.NewProcksServiceHandler(srv))
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	mux.Handle(srv.RequestHandler())

	port := viper.GetString(server.Port)
	lis, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		logger.Err(err).Msg("error listening")
		return err
	}

	httpServer := http.Server{
		Addr:              ":" + port,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 3 * time.Second,
	}

	// Setup signal handlers so we can gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		s := <-sigChan
		logger.Info().Msgf("got signal %v, attempting graceful shutdown", s)
		dieCtx, dieCancel := context.WithTimeout(ctx, 10*time.Second)
		defer dieCancel()
		_ = httpServer.Shutdown(dieCtx)
	}()

	logger.Info().Msgf("starting server on port %v", port)
	if err := httpServer.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Err(err).Msg("error serving")
		return err
	}

	return nil
}
