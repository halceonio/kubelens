package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/halceonio/kubelens/backend/internal/config"
)

type AuthHandler struct {
	cfg        *config.Config
	httpClient *http.Client
}

type tokenRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.cfg.Auth.ClientSecret == "" {
		writeError(w, http.StatusBadRequest, "auth.client_secret is required for code exchange")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid token request payload")
		return
	}

	if req.Code == "" || req.RedirectURI == "" {
		writeError(w, http.StatusBadRequest, "code and redirect_uri are required")
		return
	}

	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", h.cfg.Auth.KeycloakURL, h.cfg.Auth.Realm)
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", h.cfg.Auth.ClientID)
	form.Set("client_secret", h.cfg.Auth.ClientSecret)
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to build token request")
		return
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "token exchange failed")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, string(body))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
