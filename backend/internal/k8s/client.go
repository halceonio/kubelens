package k8s

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/halceonio/kubelens/backend/internal/config"
)

func NewClient(cfg config.KubernetesConfig) (*kubernetes.Clientset, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	if cfg.API.QPS > 0 {
		restCfg.QPS = cfg.API.QPS
	}
	if cfg.API.Burst > 0 {
		restCfg.Burst = cfg.API.Burst
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("k8s clientset: %w", err)
	}
	return clientset, nil
}
