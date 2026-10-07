package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
	"golang.org/x/time/rate"
)

type contextKey string

const userContextKey contextKey = "user"

type rateLimitPolicy struct {
	limit rate.Limit
	burst int
}

var (
	registrationRateLimit = rateLimitPolicy{
		limit: rate.Every(20 * time.Minute),
		burst: 3,
	}

	loginRateLimit = rateLimitPolicy{
		limit: rate.Every(2 * time.Minute),
		burst: 5,
	}

	emailVerificationRateLimit = rateLimitPolicy{
		limit: rate.Every(6 * time.Second),
		burst: 10,
	}
)

func requestClientIP(remoteAddr string) string {
	if remoteAddr == "" {
		return "unknown"
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}

func (app *application) rateLimit(scope string, policy rateLimitPolicy, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := requestClientIP(r.RemoteAddr)
		key := scope + ":" + clientIP

		if !app.rateLimiter.allow(key, policy.limit, policy.burst) {
			app.clientError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			app.clientError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		user, err := app.sessions.GetUserByToken(r.Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, models.ErrNoRecord) {
				app.clientError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			app.serverError(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverError(w, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.infoLog.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
