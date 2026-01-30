package api

import (
	"net/http"

	"github.com/halceonio/kubelens/backend/internal/config"
)

type ConfigResponse struct {
	Kubernetes KubernetesResponse `json:"kubernetes"`
	Logs       LogsResponse       `json:"logs"`
}

type KubernetesResponse struct {
	AllowedNamespaces []string          `json:"allowed_namespaces"`
	LabelPrefix       string            `json:"label_prefix"`
	AppGroups         AppGroupsResponse `json:"app_groups"`
}

type AppGroupsResponse struct {
	Enabled bool                   `json:"enabled"`
	Labels  AppGroupLabelsResponse `json:"labels"`
}

type AppGroupLabelsResponse struct {
	Selector    string `json:"selector"`
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
}

type LogsResponse struct {
	DefaultTailLines int `json:"default_tail_lines"`
	MaxTailLines     int `json:"max_tail_lines"`
	MaxLineLength    int `json:"max_line_length"`
}

func NewConfigHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		resp := ConfigResponse{
			Kubernetes: KubernetesResponse{
				AllowedNamespaces: cfg.Kubernetes.AllowedNamespaces,
				LabelPrefix:       cfg.Kubernetes.LabelPrefix,
				AppGroups: AppGroupsResponse{
					Enabled: cfg.Kubernetes.AppGroups.Enabled,
					Labels: AppGroupLabelsResponse{
						Selector:    cfg.Kubernetes.AppGroups.Labels.Selector,
						Name:        cfg.Kubernetes.AppGroups.Labels.Name,
						Environment: cfg.Kubernetes.AppGroups.Labels.Environment,
						Version:     cfg.Kubernetes.AppGroups.Labels.Version,
					},
				},
			},
			Logs: LogsResponse{
				DefaultTailLines: cfg.Logs.DefaultTailLines,
				MaxTailLines:     cfg.Logs.MaxTailLines,
				MaxLineLength:    cfg.Logs.MaxLineLength,
			},
		}

		writeJSON(w, resp)
	}
}
