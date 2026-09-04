package models

import "errors"

var (
	ErrNoRecord                      = errors.New("models: no matching record found")
	ErrDuplicateEmail                = errors.New("models: duplicate email")
	ErrInvalidEmailVerificationToken = errors.New("models: invalid or expired email verification token")
)
