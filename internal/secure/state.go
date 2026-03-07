package secure

import "crypto/rand"

// StateGenerator defines functions for a generator of long random unique cryptographically generated values.
type StateGenerator interface {
	GetState() (string, error)
}

//nolint:revive
type CSPRNGStateGenerator struct{}

// NewCSPRNGStateGenerator is an implementation of StateGenerator based on Go's crypto/rnad module.
func NewCSPRNGStateGenerator() *CSPRNGStateGenerator {
	return &CSPRNGStateGenerator{}
}

// GetState returns a new random string based on Go's native crypto/rand module.
func (csg *CSPRNGStateGenerator) GetState() (string, error) {
	return rand.Text(), nil
}
