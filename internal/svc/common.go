package svc

import "time"

// CookieConfig is a wrapper for working with expiring cookies.
type CookieConfig struct {
	Name      string
	ExpiresIn time.Duration
}
