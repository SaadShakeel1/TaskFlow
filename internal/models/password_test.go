package models_test

import (
	"testing"

	"github.com/SaadShakeel1/TaskFlow/internal/models"
)

func TestPasswordSetAndMatches(t *testing.T) {
	var password models.Password

	if err := password.Set("correct-password"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	matches, err := password.Matches("correct-password")

	if err != nil {
		t.Fatalf("match correct password: %v", err)
	}

	if !matches {
		t.Error("correct password did not match")
	}

	matches, err = password.Matches("wrong-password")

	if err != nil {
		t.Fatalf("match incorrect password: %v", err)
	}

	if matches {
		t.Errorf("Incorrect password matches")
	}

}

func TestEmptyPasswordHashDoesNotMatch(t *testing.T) {
	var password models.Password

	matches, err := password.Matches("any-password")

	if err != nil {
		t.Fatalf("match empty hash: %v", err)
	}

	if matches {
		t.Error("password matched any empty hash")
	}
}
