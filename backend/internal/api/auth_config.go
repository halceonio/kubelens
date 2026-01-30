package api

import (
	"net/http"

	"github.com/halceonio/kubelens/backend/internal/config"
)

type AuthConfigResponse struct {
	KeycloakURL          string   `json:"keycloak_url"`
	Realm                string   `json:"realm"`
	ClientID             string   `json:"client_id"`
	AllowedGroups        []string `json:"allowed_groups"`
	AllowedSecretsGroups []string `json:"allowed_secrets_groups,omitempty"`
}

func NewAuthConfigHandler(getConfig func() *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		cfg := getConfig()
		if cfg == nil {
			writeError(w, http.StatusServiceUnavailable, "config unavailable")
			return
		}

		resp := AuthConfigResponse{
			KeycloakURL:          cfg.Auth.KeycloakURL,
			Realm:                cfg.Auth.Realm,
			ClientID:             cfg.Auth.ClientID,
			AllowedGroups:        cfg.Auth.AllowedGroups,
			AllowedSecretsGroups: cfg.Auth.AllowedSecretsGroups,
		}
		writeJSON(w, resp)
	}
}
