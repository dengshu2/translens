package main

import (
	"context"
	"net/http"
	"strings"
)

// corsMiddleware adds CORS headers, restricting access to the given origin
// (configured via ALLOWED_ORIGIN) plus localhost for development.
func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Vary must be set unconditionally so caches never serve a
			// response with CORS headers to a different Origin.
			w.Header().Add("Vary", "Origin")

			if origin == allowedOrigin ||
				origin == "http://localhost:8080" ||
				origin == "http://localhost:3000" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// jwtMiddleware extracts and validates the Bearer token, injecting claims into the context.
func jwtMiddleware(auth *AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				respondError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := auth.ValidateToken(tokenStr)
			if err != nil {
				respondError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), contextKeyUser, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// claimsFromCtx extracts JWT claims from the request context.
func claimsFromCtx(r *http.Request) *Claims {
	c, _ := r.Context().Value(contextKeyUser).(*Claims)
	return c
}
