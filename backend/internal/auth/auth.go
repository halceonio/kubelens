package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/halceonio/kubelens/backend/internal/config"
)

var (
	ErrMissingToken  = errors.New("missing bearer token")
	ErrInvalidToken  = errors.New("invalid token")
	ErrForbidden     = errors.New("forbidden")
	ErrGroupsMissing = errors.New("required group missing")
)

type User struct {
	Subject        string
	Groups         []string
	AllowedSecrets bool
}

type Claims struct {
	Groups []string `json:"groups"`
	jwt.RegisteredClaims
}

type Verifier struct {
	jwks           keyfunc.Keyfunc
	issuer         string
	audience       string
	allowedGroups  map[string]struct{}
	allowedSecrets map[string]struct{}
}

type oidcConfig struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

func NewVerifier(ctx context.Context, cfg config.AuthConfig) (*Verifier, error) {
	issuerURL := strings.TrimRight(cfg.KeycloakURL, "/") + "/realms/" + cfg.Realm
	wellKnown := issuerURL + "/.well-known/openid-configuration"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(wellKnown)
	if err != nil {
		return nil, fmt.Errorf("fetch oidc config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch oidc config: status %d", resp.StatusCode)
	}

	var meta oidcConfig
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("decode oidc config: %w", err)
	}
	if meta.JWKSURI == "" {
		return nil, errors.New("oidc config missing jwks_uri")
	}
	if meta.Issuer != "" {
		issuerURL = meta.Issuer
	}

	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{meta.JWKSURI})
	if err != nil {
		return nil, fmt.Errorf("init jwks: %w", err)
	}

	allowed := make(map[string]struct{})
	for _, g := range cfg.AllowedGroups {
		allowed[g] = struct{}{}
	}
	allowedSecrets := make(map[string]struct{})
	for _, g := range cfg.AllowedSecretsGroups {
		allowedSecrets[g] = struct{}{}
	}

	return &Verifier{
		jwks:           jwks,
		issuer:         issuerURL,
		audience:       cfg.ClientID,
		allowedGroups:  allowed,
		allowedSecrets: allowedSecrets,
	}, nil
}

func (v *Verifier) AuthenticateRequest(r *http.Request) (*User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, ErrMissingToken
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, ErrMissingToken
	}
	tokenString := strings.TrimSpace(parts[1])
	if tokenString == "" {
		return nil, ErrMissingToken
	}

	claims := &Claims{}
	opts := []jwt.ParserOption{
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(5 * time.Second),
		jwt.WithValidMethods([]string{
			jwt.SigningMethodRS256.Alg(),
			jwt.SigningMethodRS384.Alg(),
			jwt.SigningMethodRS512.Alg(),
		}),
	}

	parsed, err := jwt.ParseWithClaims(tokenString, claims, v.jwks.Keyfunc, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !parsed.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Subject == "" {
		return nil, ErrInvalidToken
	}

	if !v.hasAnyGroup(claims.Groups, v.allowedGroups) {
		return nil, ErrGroupsMissing
	}

	user := &User{
		Subject:        claims.Subject,
		Groups:         claims.Groups,
		AllowedSecrets: v.hasAnyGroup(claims.Groups, v.allowedSecrets),
	}
	return user, nil
}

func (v *Verifier) hasAnyGroup(groups []string, allow map[string]struct{}) bool {
	for _, g := range groups {
		if _, ok := allow[g]; ok {
			return true
		}
	}
	return false
}

func Middleware(v *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := v.AuthenticateRequest(r)
			if err != nil {
				switch {
				case errors.Is(err, ErrGroupsMissing):
					writeError(w, http.StatusForbidden, "access denied")
				case errors.Is(err, ErrMissingToken):
					writeError(w, http.StatusUnauthorized, "missing token")
				default:
					writeError(w, http.StatusUnauthorized, "invalid token")
				}
				return
			}
			ctx := context.WithValue(r.Context(), userKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type userKey struct{}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userKey{}).(*User)
	return user, ok
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
