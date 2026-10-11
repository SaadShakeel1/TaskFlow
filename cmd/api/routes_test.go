package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	newTestApplication(nil).routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body healthResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
}

func TestGreetingEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/greet", strings.NewReader(`{"name":"Saad"}`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newTestApplication(nil).routes().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body greetingResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	if body.Message != "Hello, Saad!" {
		t.Errorf("message = %q, want %q", body.Message, "Hello, Saad!")
	}
}

func TestGreetingEndpointRejectsBlankName(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/greet", strings.NewReader(`{"name":"   "}`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newTestApplication(nil).routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	if got := recorder.Body.String(); got != "name is required\n" {
		t.Errorf("response body = %q, want %q", got, "name is required\n")
	}
}

func TestGreetingEndpointRejectsInvalidJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/greet", strings.NewReader(`{"name":`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	newTestApplication(nil).routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	if got := recorder.Body.String(); got != "request body contains badly-formed JSON\n" {
		t.Errorf("response body = %q, want %q", got, "request body contains badly-formed JSON\n")
	}
}

func TestVerifyEmailPageEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/verify-email?token=test-token", nil)
	recorder := httptest.NewRecorder()

	newTestApplication(nil).routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/html; charset=utf-8")
	}

	if !strings.Contains(recorder.Body.String(), `fetch("/users/verify-email"`) {
		t.Error("verification page does not call the verification API")
	}
}
