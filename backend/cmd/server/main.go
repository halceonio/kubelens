package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/halceonio/kubelens/backend/internal/auth"
	"github.com/halceonio/kubelens/backend/internal/config"
	"github.com/halceonio/kubelens/backend/internal/k8s"
	"github.com/halceonio/kubelens/backend/internal/server"
	"github.com/halceonio/kubelens/backend/internal/storage"
)

func main() {
	logger := log.New(os.Stdout, "kubelens-backend ", log.LstdFlags|log.LUTC)

	cfg, path, err := config.Load()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}
	logger.Printf("loaded config from %s", path)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	verifier, err := auth.NewVerifier(ctx, cfg.Auth)
	if err != nil {
		logger.Fatalf("auth setup error: %v", err)
	}

	k8sClient, err := k8s.NewClient(cfg.Kubernetes)
	if err != nil {
		logger.Fatalf("k8s client error: %v", err)
	}

	sessionStore, backend, err := storage.NewSessionStoreFromConfig(ctx, cfg)
	if err != nil {
		logger.Fatalf("session store error: %v", err)
	}
	logger.Printf("session store: %s", backend)

	srv := server.New(cfg, verifier, k8sClient, sessionStore)

	go func() {
		logger.Printf("server listening on %s", cfg.Server.Address)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Printf("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
}
