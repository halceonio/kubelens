package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/halceonio/kubelens/backend/internal/auth"
	"github.com/halceonio/kubelens/backend/internal/storage"
)

const sessionVersion = 1

func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type ReadinessFunc func(r *http.Request) error

func ReadyHandler(check ReadinessFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := check(r); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not-ready"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}

type SessionHandler struct {
	Store    storage.SessionStore
	MaxBytes int64
}

func NewSessionHandler(store storage.SessionStore, maxBytes int) *SessionHandler {
	return &SessionHandler{
		Store:    store,
		MaxBytes: int64(maxBytes),
	}
}

func (h *SessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPut:
		h.handlePut(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *SessionHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	rec, err := h.Store.Get(r.Context(), user.Subject)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"version": sessionVersion})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load session")
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rec.Data)
}

func (h *SessionHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}

	maxBytes := h.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "session payload too large")
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "empty payload")
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	if err := validateSessionPayload(payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	payload["version"] = sessionVersion
	payload["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)

	encoded, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode session")
		return
	}

	if int64(len(encoded)) > maxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "session payload too large")
		return
	}

	if err := h.Store.Put(r.Context(), user.Subject, encoded); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save session")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *SessionHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}
	if err := h.Store.Delete(r.Context(), user.Subject); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear session")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func validateSessionPayload(payload map[string]any) error {
	if themeVal, ok := payload["theme"]; ok {
		theme, ok := themeVal.(string)
		if !ok {
			return errors.New("theme must be a string")
		}
		if theme != "" && theme != "light" && theme != "dark" {
			return errors.New("theme must be light or dark")
		}
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
