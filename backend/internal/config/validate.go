package config

import (
	"fmt"
	"strings"
)

type ValidationResult struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

func Validate(cfg *Config) ValidationResult {
	if cfg == nil {
		return ValidationResult{Errors: []string{"config is nil"}}
	}

	var errs []string
	var warns []string

	if cfg.Auth.KeycloakURL == "" {
		errs = append(errs, "auth.keycloak_url is required")
	}
	if cfg.Auth.Realm == "" {
		errs = append(errs, "auth.realm is required")
	}
	if cfg.Auth.ClientID == "" {
		errs = append(errs, "auth.client_id is required")
	}
	if len(cfg.Auth.AllowedGroups) == 0 && len(cfg.Auth.LegacyAllowsGroups) == 0 {
		errs = append(errs, "auth.allowed_groups is required")
	}

	if len(cfg.Kubernetes.AllowedNamespaces) == 0 {
		warns = append(warns, "kubernetes.allowed_namespaces is empty (no namespaces will be accessible)")
	}

	if cfg.Kubernetes.AppGroups.Enabled {
		if cfg.Kubernetes.AppGroups.Labels.Selector == "" {
			warns = append(warns, "kubernetes.app_groups.labels.selector is empty while app_groups.enabled is true")
		}
	}

	if cfg.Logs.MaxTailLines <= 0 {
		warns = append(warns, "logs.max_tail_lines should be > 0")
	}
	if cfg.Logs.MaxLineLength <= 0 {
		warns = append(warns, "logs.max_line_length should be > 0")
	}

	if cfg.Logs.UseRedisStreams {
		redisURL := cfg.Logs.RedisURLOverride
		if redisURL == "" {
			redisURL = cfg.Cache.RedisURL
		}
		if redisURL == "" {
			errs = append(errs, "logs.use_redis_streams requires cache.redis_url or logs.redis_url")
		}
	}

	if cfg.Server.WriteTimeoutSeconds > 0 {
		warns = append(warns, "server.write_timeout_seconds should be 0 for long-lived SSE connections")
	}

	for i, crd := range cfg.Kubernetes.CustomResources {
		if !crd.Enabled {
			continue
		}
		prefix := fmt.Sprintf("kubernetes.custom_resources[%d]", i)
		if crd.Group == "" || crd.Version == "" || crd.Resource == "" || crd.Kind == "" {
			errs = append(errs, fmt.Sprintf("%s requires group, version, resource, and kind", prefix))
		}
		if crd.PodLabelKey == "" {
			warns = append(warns, fmt.Sprintf("%s has no pod_label_key (log streaming may be unavailable)", prefix))
		}
	}

	if cfg.Logs.RateLimitPerMinute < 0 || cfg.Logs.RateLimitBurst < 0 {
		errs = append(errs, "logs.rate_limit_per_minute and logs.rate_limit_burst must be >= 0")
	}

	if cfg.Logs.RateLimitPerMinute > 0 && cfg.Logs.RateLimitBurst == 0 {
		warns = append(warns, "logs.rate_limit_burst is 0; using rate_limit_per_minute as burst is recommended")
	}

	if cfg.Auth.ClientSecret == "" {
		warns = append(warns, "auth.client_secret is empty (public client)")
	}

	if strings.TrimSpace(cfg.Server.Address) == "" {
		errs = append(errs, "server.address is required")
	}

	return ValidationResult{Errors: errs, Warnings: warns}
}
