package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/halceonio/kubelens/backend/internal/config"
)

func NewClient(cfg config.KubernetesConfig) (*kubernetes.Clientset, error) {
	clientset, _, err := NewClients(cfg)
	return clientset, err
}

func NewClients(cfg config.KubernetesConfig) (*kubernetes.Clientset, metadata.Interface, error) {
	restCfg, err := loadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("k8s config: %w", err)
	}
	if cfg.API.QPS > 0 {
		restCfg.QPS = cfg.API.QPS
	}
	if cfg.API.Burst > 0 {
		restCfg.Burst = cfg.API.Burst
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("k8s clientset: %w", err)
	}
	metaClient, err := metadata.NewForConfig(restCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("k8s metadata client: %w", err)
	}
	return clientset, metaClient, nil
}

func loadConfig() (*rest.Config, error) {
	if path := os.Getenv("KUBELENS_KUBECONFIG"); path != "" {
		return clientcmd.BuildConfigFromFlags("", path)
	}
	if path := os.Getenv("KUBECONFIG"); path != "" {
		return clientcmd.BuildConfigFromFlags("", path)
	}
	return rest.InClusterConfig()
}
