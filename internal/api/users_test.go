package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/testutil"
	"github.com/SaadShakeel1/TaskFlow/internal/users"
)

const databaseTestTimeout = 5 * time.Second

func TestCreateUserEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		strings.NewReader(`{"email":"saad@example.com","name":"Saad"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	NewRouter(db).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body userResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	if body.ID < 1 {
		t.Errorf("id = %d, want a positive value", body.ID)
	}

	if body.Email != "saad@example.com" {
		t.Errorf("email = %q, want %q", body.Email, "saad@example.com")
	}

	if body.Name != "Saad" {
		t.Errorf("name = %q, want %q", body.Name, "Saad")
	}
}

func TestGetUserEndpoint(t *testing.T) {
	db := testutil.OpenTestDB(t)

	testutil.ResetUsers(t, db)
	t.Cleanup(func() {
		testutil.ResetUsers(t, db)
	})

	ctx, cancel := context.WithTimeout(context.Background(), databaseTestTimeout)
	defer cancel()

	createdUser, err := users.Create(ctx, db, "saad@example.com", "Saad")
	if err != nil {
		t.Fatalf("create user for GET test: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/users/"+strconv.FormatInt(createdUser.ID, 10),
		nil,
	)
	recorder := httptest.NewRecorder()
	NewRouter(db).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", response.StatusCode, http.StatusOK)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body userResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	if body.ID != createdUser.ID {
		t.Errorf("id = %d, want %d", body.ID, createdUser.ID)
	}

	if body.Email != "saad@example.com" {
		t.Errorf("email = %q, want %q", body.Email, "saad@example.com")
	}

	if body.Name != "Saad" {
		t.Errorf("name = %q, want %q", body.Name, "Saad")
	}
}

func TestGetUserEndpointRejectsInvalidID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/users/not-a-number", nil)
	recorder := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	if got := recorder.Body.String(); got != "user id must be positive integer\n" {
		t.Errorf("response body = %q, want %q", got, "user id must be positive integer\n")
	}
}
