package middleware

import (
	"net/http"
	"os"
	"strings"
)

// EnableCORS adds CORS headers to responses
func EnableCORS(next http.HandlerFunc) http.HandlerFunc {
	// Get allowed origins from environment variable
	// Example: ALLOWED_ORIGINS="http://localhost:3000,http://localhost:5173,https://myapp.com"
	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")

	// Default allowed origins if not set
	var allowedOrigins []string
	if allowedOriginsEnv != "" {
		allowedOrigins = strings.Split(allowedOriginsEnv, ",")
		// Trim whitespace from each origin
		for i := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
		}
	} else {
		// Default for development if env var not set
		allowedOrigins = []string{"http://localhost:5173", "http://localhost:3000"}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if the request origin is in the allowed list
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if os.Getenv("CORS_ALLOW_ALL") == "true" {
			// Optional: Allow all origins in development
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			// Don't set Access-Control-Allow-Origin header if origin not allowed
			// This will cause CORS to block the request
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}
