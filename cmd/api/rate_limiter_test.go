package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
	"golang.org/x/time/rate"
)

func TestRateLimiterAllow(t *testing.T) {
	currentTime := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)

	limiter := newRateLimiter()
	limiter.now = func() time.Time {
		return currentTime
	}

	policy := rate.Every(time.Minute)
	key := "login:127.0.0.1"

	if !limiter.allow(key, policy, 2) {
		t.Fatal("first request was rejected")
	}

	if !limiter.allow(key, policy, 2) {
		t.Fatal("second request was rejected")
	}

	if limiter.allow(key, policy, 2) {
		t.Error("third request was allowed after the burst was exhausted")
	}

	currentTime = currentTime.Add(time.Minute)

	if !limiter.allow(key, policy, 2) {
		t.Error("request was rejected after a token should have refilled")
	}

	if !limiter.allow("login:192.0.2.10", policy, 2) {
		t.Error("different client key was affected by the first client")
	}
}

func TestRateLimiterRemovesInactiveClients(t *testing.T) {
	currentTime := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)

	limiter := newRateLimiter()
	limiter.now = func() time.Time {
		return currentTime
	}

	policy := rate.Every(time.Minute)
	oldKey := "login:192.0.2.1"

	if !limiter.allow(oldKey, policy, 1) {
		t.Fatal("initial request was rejected")
	}

	currentTime = currentTime.Add(inactiveClientTTL + time.Second)

	if !limiter.allow("login:192.0.2.2", policy, 1) {
		t.Fatal("new client request was rejected")
	}

	if _, exists := limiter.clients[oldKey]; exists {
		t.Error("inactive client was not removed")
	}
}

func TestRequestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "IPv4 with port", remoteAddr: "192.0.2.10:54321", want: "192.0.2.10"},
		{name: "IPv6 with port", remoteAddr: "[2001:db8::1]:54321", want: "2001:db8::1"},
		{name: "host without port", remoteAddr: "192.0.2.10", want: "192.0.2.10"},
		{name: "empty address", remoteAddr: "", want: "unknown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := requestClientIP(test.remoteAddr); got != test.want {
				t.Errorf("requestClientIP(%q) = %q, want %q", test.remoteAddr, got, test.want)
			}
		})
	}
}

func TestRateLimitMiddlewareRejectsExcessRequestsAndSeparatesScopes(t *testing.T) {
	app := &application{rateLimiter: newRateLimiter()}
	policy := rateLimitPolicy{limit: rate.Every(time.Hour), burst: 1}

	loginCalls := 0
	loginHandler := app.rateLimit("login", policy, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginCalls++
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/sessions", nil)
		request.RemoteAddr = "198.51.100.10:5000"
		recorder := httptest.NewRecorder()

		loginHandler.ServeHTTP(recorder, request)

		wantStatus := http.StatusNoContent
		if i == 1 {
			wantStatus = http.StatusTooManyRequests
		}
		if recorder.Code != wantStatus {
			t.Errorf("login request %d status = %d, want %d", i+1, recorder.Code, wantStatus)
		}
	}

	if loginCalls != 1 {
		t.Errorf("login handler calls = %d, want 1", loginCalls)
	}

	verificationCalls := 0
	verificationHandler := app.rateLimit("verification", policy, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verificationCalls++
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/users/verify-email", nil)
	request.RemoteAddr = "198.51.100.10:5000"
	recorder := httptest.NewRecorder()

	verificationHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Errorf("verification status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if verificationCalls != 1 {
		t.Errorf("verification handler calls = %d, want 1", verificationCalls)
	}
}

func TestRegistrationRouteIsRateLimited(t *testing.T) {
	db := testutil.OpenTestDB(t)
	testutil.ResetDatabase(t, db)
	t.Cleanup(func() {
		testutil.ResetDatabase(t, db)
	})

	app := newTestApplication(db)
	handler := app.routes()

	for i := 1; i <= registrationRateLimit.burst; i++ {
		request := httptest.NewRequest(
			http.MethodPost,
			"/users",
			strings.NewReader(fmt.Sprintf(
				`{"email":"rate-limit-%d@example.com","name":"Rate Limit","password":"correct-password"}`,
				i,
			)),
		)
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "203.0.113.10:5000"

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("registration request %d status = %d, want %d", i, recorder.Code, http.StatusCreated)
		}
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		strings.NewReader(`{"email":"blocked@example.com","name":"Blocked","password":"correct-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.10:5000"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("fourth registration status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}
