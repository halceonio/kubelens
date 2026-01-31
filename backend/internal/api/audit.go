package api

import (
	"net/http"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/halceonio/kubelens/backend/internal/auth"
)

func (h *KubeHandler) audit(r *http.Request, action, namespace, name string, extra map[string]any) {
	if h == nil || !h.cfg.Server.AuditLogs {
		return
	}

	fields := []any{
		"action", action,
		"namespace", namespace,
		"name", name,
		"path", r.URL.Path,
		"method", r.Method,
		"remote", remoteIP(r),
	}

	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		fields = append(fields, "sub", user.Subject)
		if len(user.Groups) > 0 {
			fields = append(fields, "groups", strings.Join(user.Groups, ","))
		}
		fields = append(fields, "secrets", user.AllowedSecrets)
	}

	if extra != nil {
		for k, v := range extra {
			fields = append(fields, k, v)
		}
	}

	log.Info("audit", fields...)
}

func remoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}
