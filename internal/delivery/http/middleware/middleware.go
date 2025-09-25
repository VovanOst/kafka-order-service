package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"kafka-order-service/pkg/logger"

	"github.com/google/uuid"
)

// RequestIDKey - key for request ID in context
type RequestIDKey struct{}

// Logger logs HTTP requests
func Logger(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)
			r = r.WithContext(ctx)
			w.Header().Set("X-Request-ID", requestID)

			wrapper := &responseWrapper{ResponseWriter: w, statusCode: http.StatusOK}
			log.Info("HTTP request started", "method", r.Method, "path", r.URL.Path, "request_id", requestID)

			next.ServeHTTP(wrapper, r)

			duration := time.Since(start)
			log.Info("HTTP request completed", "method", r.Method, "path", r.URL.Path, "status", wrapper.statusCode, "duration_ms", duration.Milliseconds(), "request_id", requestID)
		})
	}
}

// Recovery recovers from panics
func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					reqID, _ := r.Context().Value(RequestIDKey{}).(string)
					log.Error("Panic recovered", "error", fmt.Sprintf("%v", err), "request_id", reqID)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, `{"error":"Internal server error","request_id":"%s"}`, reqID)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS sets CORS headers
func CORS() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout sets a timeout for requests
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, duration, "request timeout")
	}
}

// Security adds security headers
func Security() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			next.ServeHTTP(w, r)
		})
	}
}

// Metrics logs basic metrics
func Metrics(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapper := &responseWrapper{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapper, r)

			duration := time.Since(start)
			log.Debug("HTTP metrics", "method", r.Method, "path", r.URL.Path, "status", wrapper.statusCode, "duration_ms", duration.Milliseconds())
		})
	}
}

// Chain combines multiple middleware
func Chain(mw ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(mw) - 1; i >= 0; i-- {
			final = mw[i](final)
		}
		return final
	}
}

func JSONOnly() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnsupportedMediaType)
					fmt.Fprintf(w, `{"error":"Content-Type must be application/json","received":"%s","timestamp":"%s"}`, contentType, time.Now().Format(time.RFC3339))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// responseWrapper captures status code for metrics
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(status int) {
	rw.statusCode = status
	rw.ResponseWriter.WriteHeader(status)
}
