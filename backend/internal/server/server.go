package server

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/halceonio/kubelens/backend/internal/api"
	"github.com/halceonio/kubelens/backend/internal/auth"
	"github.com/halceonio/kubelens/backend/internal/config"
	"github.com/halceonio/kubelens/backend/internal/storage"
)

type Server struct {
	cfg          atomic.Value
	auth         auth.VerifierProvider
	k8sClient    *kubernetes.Clientset
	sessionStore storage.SessionStore
	kubeHandler  *dynamicHandler
	httpServer   *http.Server
}

func New(cfg *config.Config, verifier auth.VerifierProvider, client *kubernetes.Clientset, sessions storage.SessionStore) *Server {
	s := &Server{
		auth:         verifier,
		k8sClient:    client,
		sessionStore: sessions,
	}
	s.cfg.Store(cfg)
	configProvider := func() *config.Config { return s.cfg.Load().(*config.Config) }

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.HealthHandler)
	mux.Handle("/readyz", api.ReadyHandler(func(r *http.Request) error {
		if client == nil {
			return errors.New("k8s client not ready")
		}
		return nil
	}))

	sessionHandler := api.NewSessionHandler(sessions, cfg.Session.MaxBytes)
	authHandler := api.NewAuthHandler(configProvider)
	authConfigHandler := api.NewAuthConfigHandler(configProvider)
	configHandler := api.NewConfigHandler(configProvider)
	mux.Handle("/api/v1/session", auth.Middleware(verifier)(sessionHandler))
	mux.Handle("/api/v1/auth/token", authHandler)
	mux.Handle("/api/v1/auth/config", authConfigHandler)
	mux.Handle("/api/v1/config", auth.Middleware(verifier)(configHandler))

	kubeDynamic := newDynamicHandler(auth.Middleware(verifier)(api.NewKubeHandler(cfg, client)))
	mux.Handle("/api/v1/namespaces", kubeDynamic)
	mux.Handle("/api/v1/namespaces/", kubeDynamic)

	server := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
	}

	s.kubeHandler = kubeDynamic
	s.httpServer = server
	return s
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) UpdateConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	s.cfg.Store(cfg)
	if s.k8sClient != nil && s.kubeHandler != nil {
		s.kubeHandler.Update(auth.Middleware(s.auth)(api.NewKubeHandler(cfg, s.k8sClient)))
	}
}
