package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/halceonio/kubelens/backend/internal/api"
	"github.com/halceonio/kubelens/backend/internal/auth"
	"github.com/halceonio/kubelens/backend/internal/config"
	"github.com/halceonio/kubelens/backend/internal/storage"
)

type Server struct {
	cfg          *config.Config
	auth         *auth.Verifier
	k8sClient    *kubernetes.Clientset
	sessionStore storage.SessionStore
	httpServer   *http.Server
}

func New(cfg *config.Config, verifier *auth.Verifier, client *kubernetes.Clientset, sessions storage.SessionStore) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.HealthHandler)
	mux.Handle("/readyz", api.ReadyHandler(func(r *http.Request) error {
		if client == nil {
			return errors.New("k8s client not ready")
		}
		return nil
	}))

	sessionHandler := api.NewSessionHandler(sessions, cfg.Session.MaxBytes)
	mux.Handle("/api/v1/session", auth.Middleware(verifier)(sessionHandler))
	mux.Handle("/api/v1/namespaces/", auth.Middleware(verifier)(api.NewKubeHandler(cfg, client)))

	server := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
	}

	return &Server{
		cfg:          cfg,
		auth:         verifier,
		k8sClient:    client,
		sessionStore: sessions,
		httpServer:   server,
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
