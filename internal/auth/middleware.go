package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

type contextKey string

const (
	organizationIDKey contextKey = "organizationID"
)

type APIKeyValidator interface {
	ValidateAPIKey(ctx context.Context, keyHash string) (string, error)
}

func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func APIKeyMiddleware(validator APIKeyValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
			if apiKey == "" {
				http.Error(w, "missing X-API-Key header", http.StatusUnauthorized)
				return
			}

			keyHash := hashAPIKey(apiKey)
			organizationID, err := validator.ValidateAPIKey(r.Context(), keyHash)
			if err != nil {
				http.Error(w, "invalid API key", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), organizationIDKey, organizationID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetOrganizationID(ctx context.Context) string {
	orgID, _ := ctx.Value(organizationIDKey).(string)
	return orgID
}

func LocalModeMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), organizationIDKey, "local-org")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
