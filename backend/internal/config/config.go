package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Auth       AuthConfig       `yaml:"auth"`
	Logs       LogsConfig       `yaml:"logs"`
	Session    SessionConfig    `yaml:"session"`
	Storage    StorageConfig    `yaml:"storage"`
	Cache      CacheConfig      `yaml:"cache"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
}

type ServerConfig struct {
	Address             string `yaml:"address"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
	IdleTimeoutSeconds  int    `yaml:"idle_timeout_seconds"`
}

type AuthConfig struct {
	KeycloakURL          string   `yaml:"keycloak_url"`
	Realm                string   `yaml:"realm"`
	ClientID             string   `yaml:"client_id"`
	ClientSecret         string   `yaml:"client_secret"`
	AllowedGroups        []string `yaml:"allowed_groups"`
	LegacyAllowsGroups   []string `yaml:"allows_groups"`
	AllowedSecretsGroups []string `yaml:"allowed_secrets_groups"`
}

type LogsConfig struct {
	DefaultTailLines int `yaml:"default_tail_lines"`
	MaxTailLines     int `yaml:"max_tail_lines"`
	MaxLineLength    int `yaml:"max_line_length"`
}

type SessionConfig struct {
	MaxBytes int `yaml:"max_bytes"`
}

type StorageConfig struct {
	DatabaseURL string `yaml:"database_url"`
}

type CacheConfig struct {
	Enabled  bool   `yaml:"enabled"`
	RedisURL string `yaml:"redis_url"`
}

type KubernetesConfig struct {
	ClusterName       string          `yaml:"cluster_name"`
	TerminatedLogTTL  int             `yaml:"terminated_log_ttl"`
	API               KubernetesAPI   `yaml:"api"`
	AllowedNamespaces []string        `yaml:"allowed_namespaces"`
	AppGroups         AppGroupsConfig `yaml:"app_groups"`
	PodFilters        ResourceFilters `yaml:"pod_filters"`
	AppFilters        ResourceFilters `yaml:"app_filters"`
	LabelPrefix       string          `yaml:"label_prefix"`
}

type KubernetesAPI struct {
	Burst int     `yaml:"burst"`
	QPS   float32 `yaml:"qps"`
}

type AppGroupsConfig struct {
	Enabled bool           `yaml:"enabled"`
	Labels  AppGroupLabels `yaml:"labels"`
}

type AppGroupLabels struct {
	Selector    string `yaml:"selector"`
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
	Version     string `yaml:"version"`
}

type ResourceFilters struct {
	IncludeRegex  string   `yaml:"include_regex"`
	ExcludeLabels []string `yaml:"exclude_labels"`
}

func Load() (*Config, string, error) {
	path := os.Getenv("KUBELENS_CONFIG")
	if path == "" {
		path = os.Getenv("KUBELENS_CONFIG_PATH")
	}

	candidates := []string{}
	if path != "" {
		candidates = append(candidates, path)
	}
	candidates = append(candidates,
		"/etc/kubelens/config.yaml",
		"./config.yaml",
		"./backend/config.yaml",
	)

	var selected string
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			selected = candidate
			break
		}
	}
	if selected == "" {
		return nil, "", errors.New("config file not found")
	}

	data, err := os.ReadFile(filepath.Clean(selected))
	if err != nil {
		return nil, "", fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, "", fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(cfg)
	if err := validate(cfg); err != nil {
		return nil, "", err
	}

	return cfg, selected, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":8080"
	}
	if cfg.Server.ReadTimeoutSeconds == 0 {
		cfg.Server.ReadTimeoutSeconds = 10
	}
	if cfg.Server.WriteTimeoutSeconds == 0 {
		cfg.Server.WriteTimeoutSeconds = 30
	}
	if cfg.Server.IdleTimeoutSeconds == 0 {
		cfg.Server.IdleTimeoutSeconds = 60
	}

	if len(cfg.Auth.AllowedGroups) == 0 && len(cfg.Auth.LegacyAllowsGroups) > 0 {
		cfg.Auth.AllowedGroups = cfg.Auth.LegacyAllowsGroups
	}

	if cfg.Logs.DefaultTailLines == 0 {
		cfg.Logs.DefaultTailLines = 10000
	}
	if cfg.Logs.MaxTailLines == 0 {
		cfg.Logs.MaxTailLines = 10000
	}
	if cfg.Logs.MaxLineLength == 0 {
		cfg.Logs.MaxLineLength = 10000
	}
	if cfg.Session.MaxBytes == 0 {
		cfg.Session.MaxBytes = 256 * 1024
	}
	if cfg.Kubernetes.TerminatedLogTTL == 0 {
		cfg.Kubernetes.TerminatedLogTTL = int((time.Minute * 60).Seconds())
	}
}

func validate(cfg *Config) error {
	if cfg.Auth.KeycloakURL == "" {
		return errors.New("auth.keycloak_url is required")
	}
	if cfg.Auth.Realm == "" {
		return errors.New("auth.realm is required")
	}
	if cfg.Auth.ClientID == "" {
		return errors.New("auth.client_id is required")
	}
	if len(cfg.Auth.AllowedGroups) == 0 {
		return errors.New("auth.allowed_groups is required")
	}
	if len(cfg.Kubernetes.AllowedNamespaces) == 0 {
		return errors.New("kubernetes.allowed_namespaces is required")
	}
	return nil
}
