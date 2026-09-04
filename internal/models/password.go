package models

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type Password struct {
	hash []byte
}

func (p *Password) Set(plaintext string) error {

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	p.hash = hash

	return nil
}

func (p *Password) Matches(plaintext string) (bool, error) {

	if len(p.hash) == 0 {
		return false, nil
	}

	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintext))

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("compare password hash: %w", err)
	}

	return true, nil
}
