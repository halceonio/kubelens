package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

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
	dynamicVerifier := auth.NewDynamicVerifier(verifier)

	k8sClient, metaClient, err := k8s.NewClients(cfg.Kubernetes)
	if err != nil {
		logger.Fatalf("k8s client error: %v", err)
	}

	sessionStore, backend, err := storage.NewSessionStoreFromConfig(ctx, cfg)
	if err != nil {
		logger.Fatalf("session store error: %v", err)
	}
	logger.Printf("session store: %s", backend)

	srv := server.New(cfg, dynamicVerifier, k8sClient, metaClient, sessionStore)

	go watchConfig(ctx, logger, path, func(updated *config.Config) {
		newVerifier, err := auth.NewVerifier(ctx, updated.Auth)
		if err != nil {
			logger.Printf("config reload: auth verifier update failed: %v", err)
		} else {
			dynamicVerifier.Update(newVerifier)
		}
		srv.UpdateConfig(updated)
	})

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

func watchConfig(ctx context.Context, logger *log.Logger, path string, onReload func(cfg *config.Config)) {
	if path == "" {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Printf("config watcher error: %v", err)
		return
	}
	defer watcher.Close()

	dir := filepath.Dir(path)
	if err := watcher.Add(dir); err != nil {
		logger.Printf("config watcher error: %v", err)
		return
	}
	if err := watcher.Add(path); err != nil {
		logger.Printf("config watcher error: %v", err)
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
				logger.Printf("config reload error: %v", err)
				return
			}
			logger.Printf("config reloaded from %s", path)
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
				logger.Printf("config watcher error: %v", err)
			}
		}
	}
}
