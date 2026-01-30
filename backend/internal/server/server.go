package server

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"

	"github.com/halceonio/kubelens/backend/internal/api"
	"github.com/halceonio/kubelens/backend/internal/auth"
	"github.com/halceonio/kubelens/backend/internal/config"
	"github.com/halceonio/kubelens/backend/internal/storage"
)

type Server struct {
	cfg          atomic.Value
	auth         auth.VerifierProvider
	k8sClient    *kubernetes.Clientset
	metaClient   metadata.Interface
	sessionStore storage.SessionStore
	kubeHandler  *dynamicHandler
	kubeImpl     *api.KubeHandler
	httpServer   *http.Server
}

func New(cfg *config.Config, verifier auth.VerifierProvider, client *kubernetes.Clientset, meta metadata.Interface, sessions storage.SessionStore) *Server {
	s := &Server{
		auth:         verifier,
		k8sClient:    client,
		metaClient:   meta,
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
	mux.Handle("/api/v1/metrics", api.MetricsHandler(func() *api.ResourceStats {
		if s.kubeImpl == nil {
			return nil
		}
		return s.kubeImpl.Stats()
	}))

	kubeImpl := api.NewKubeHandler(cfg, client, meta)
	kubeDynamic := newDynamicHandler(auth.Middleware(verifier)(kubeImpl))
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
	s.kubeImpl = kubeImpl
	s.httpServer = server
	return s
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.kubeImpl != nil {
		s.kubeImpl.Stop()
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) UpdateConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	s.cfg.Store(cfg)
	if s.k8sClient != nil && s.kubeHandler != nil {
		if s.kubeImpl != nil {
			s.kubeImpl.Stop()
		}
		s.kubeImpl = api.NewKubeHandler(cfg, s.k8sClient, s.metaClient)
		s.kubeHandler.Update(auth.Middleware(s.auth)(s.kubeImpl))
	}
}
