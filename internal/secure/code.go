// Package secure wraps cryptographic primitives for ease of implementation in the member ID service.
package secure

import (
	"errors"
	"fmt"

	nanoid "github.com/matoous/go-nanoid/v2"
)

// ErrGenerateCode is a generic error that may be returned during code generation.
var ErrGenerateCode = errors.New("error during code generation")

// CodeGenerator defines functions that must be implemented to create short memorable
// and cryptograhically strong codes.
type CodeGenerator interface {
	GetCode() (string, error)
}

//nolint:revive
type NanoIDStateGenerator struct {
	length   int
	alphabet string
}

// NewNanoIDCodeGenerator return a code generator implementation based on the nanoid library.
func NewNanoIDCodeGenerator(length int, alphabet string) *NanoIDStateGenerator {
	return &NanoIDStateGenerator{length: length, alphabet: alphabet}
}

// GetCode returns a member ID code based on the nanoid library.
func (__this *NanoIDStateGenerator) GetCode() (string, error) {
	result, err := nanoid.Generate(__this.alphabet, __this.length)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGenerateCode, err)
	}

	return result, nil
}
