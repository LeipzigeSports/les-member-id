package secure_test

import (
	"errors"
	"testing"

	"github.com/LeipzigeSports/les-member-id/internal/secure"
)

func TestNanoIdStateGeneratorNegativeLength(t *testing.T) {
	t.Parallel()

	gen := secure.NewNanoIDCodeGenerator(-1, "abcdefg")

	_, err := gen.GetCode()
	if !errors.Is(err, secure.ErrGenerateCode) {
		t.Errorf("want ErrGenerateCode when using negative length, got %v", err)
	}
}
