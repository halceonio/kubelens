package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"

	"github.com/halceonio/kubelens/backend/internal/auth"
	"github.com/halceonio/kubelens/backend/internal/config"
	"github.com/halceonio/kubelens/backend/internal/k8s"
	"github.com/halceonio/kubelens/backend/internal/server"
	"github.com/halceonio/kubelens/backend/internal/storage"
)

func main() {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "kubelens-backend",
		Level:           log.InfoLevel,
	})
	logger.SetTimeFunction(log.NowUTC)
	log.SetDefault(logger)

	cfg, path, err := config.Load()
	if err != nil {
		logger.Fatal("config error", "err", err)
	}
	logger.Info("loaded config", "path", path)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	verifier, err := auth.NewVerifier(ctx, cfg.Auth)
	if err != nil {
		logger.Fatal("auth setup error", "err", err)
	}
	dynamicVerifier := auth.NewDynamicVerifier(verifier)

	k8sClient, metaClient, err := k8s.NewClients(cfg.Kubernetes)
	if err != nil {
		logger.Fatal("k8s client error", "err", err)
	}

	sessionStore, backend, err := storage.NewSessionStoreFromConfig(ctx, cfg)
	if err != nil {
		logger.Fatal("session store error", "err", err)
	}
	logger.Info("session store ready", "backend", backend)

	srv := server.New(cfg, dynamicVerifier, k8sClient, metaClient, sessionStore)

	go watchConfig(ctx, logger, path, func(updated *config.Config) {
		newVerifier, err := auth.NewVerifier(ctx, updated.Auth)
		if err != nil {
			logger.Error("config reload: auth verifier update failed", "err", err)
		} else {
			dynamicVerifier.Update(newVerifier)
		}
		srv.UpdateConfig(updated)
	})

	go func() {
		logger.Info("server listening", "address", cfg.Server.Address)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", "err", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
}

func watchConfig(ctx context.Context, logger *log.Logger, path string, onReload func(cfg *config.Config)) {
	if path == "" {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("config watcher error", "err", err)
		return
	}
	defer watcher.Close()

	dir := filepath.Dir(path)
	if err := watcher.Add(dir); err != nil {
		logger.Error("config watcher error", "err", err)
		return
	}
	if err := watcher.Add(path); err != nil {
		logger.Error("config watcher error", "err", err)
	}

	var mu sync.Mutex
	var timer *time.Timer

	scheduleReload := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(500*time.Millisecond, func() {
			updated, err := config.LoadFromPath(path)
			if err != nil {
				logger.Error("config reload error", "err", err)
				return
			}
			logger.Info("config reloaded", "path", path)
			onReload(updated)
		})
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				scheduleReload()
			}
		case err := <-watcher.Errors:
			if err != nil {
				logger.Error("config watcher error", "err", err)
			}
		}
	}
}
