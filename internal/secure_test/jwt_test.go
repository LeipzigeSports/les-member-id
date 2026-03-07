package secure_test

import (
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/LeipzigeSports/les-member-id/internal/secure"
	"github.com/golang-jwt/jwt/v5"
)

const defaultKeySize = 32

type FooClaims struct {
	secure.VersionedClaims

	Foo string `json:"foo"`
}

func randomKey(t *testing.T) []byte {
	t.Helper()

	key := make([]byte, defaultKeySize)

	nBytes, err := rand.Read(key)
	if err != nil {
		t.Fatalf("failed to generate cryptographic key: %v", err)
	}

	if nBytes != defaultKeySize {
		t.Fatalf("not enough bytes: expected %d, got %d", defaultKeySize, nBytes)
	}

	return key
}

func sign(
	t *testing.T,
	claims jwt.Claims,
	secretKey []byte,
	signingMethod jwt.SigningMethod,
) string {
	t.Helper()

	token := jwt.NewWithClaims(signingMethod, claims)

	signedToken, err := token.SignedString(secretKey)
	if err != nil {
		t.Fatalf("signing token failed: %v", err)
	}

	return signedToken
}

func TestSignAndVerify(t *testing.T) {
	t.Parallel()

	cch := secure.NewSymmetricClaimHandler(randomKey(t), 1*time.Minute)
	fooValue := "bar"

	signedToken, err := cch.Sign(FooClaims{
		VersionedClaims: cch.NewVersionedClaims(),

		Foo: fooValue,
	})
	if err != nil {
		t.Fatalf("Generate() returned an error: %v", err)
	}

	fooClaims := &FooClaims{}

	err = cch.Verify(signedToken, fooClaims)
	if err != nil {
		t.Fatalf("Verify() returned an error: %v", err)
	}

	if fooClaims.Foo != fooValue {
		t.Errorf("fooClaims.Foo=%s, expected %s", fooClaims.Foo, fooValue)
	}
}

func TestVerifyRejectSigningMethod(t *testing.T) {
	t.Parallel()

	key := randomKey(t)
	handler := secure.NewSymmetricClaimHandler(key, 1*time.Minute)
	methods := []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512}

	for _, method := range methods {
		signedToken := sign(t, FooClaims{
			VersionedClaims: handler.NewVersionedClaims(),
			Foo:             "bar",
		}, key, method)

		fooClaims := &FooClaims{}

		err := handler.Verify(signedToken, fooClaims)
		if !errors.Is(err, secure.ErrSymmetricClaimOperation) {
			t.Errorf("want ErrSymmetricClaimOperation, got %v", err)
		}
	}
}

func TestVerifyRejectVersion(t *testing.T) {
	t.Parallel()

	key := randomKey(t)
	handler := secure.NewSymmetricClaimHandler(key, 1*time.Minute)

	signedToken := sign(t, FooClaims{
		VersionedClaims: secure.VersionedClaims{
			RegisteredClaims: handler.NewRegisteredClaims(),
			Version:          secure.CurrentVersion - 1,
		},
		Foo: "bar",
	}, key, jwt.SigningMethodHS256)

	fooClaims := &FooClaims{}

	err := handler.Verify(signedToken, fooClaims)
	if !errors.Is(err, secure.ErrSymmetricClaimOperation) {
		t.Errorf("want ErrSymmetricClaimOperation, got %v", err)
	}
}

//nolint:funlen
func TestVerifyRejectRegisteredClaims(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		registeredClaims jwt.RegisteredClaims
	}{
		{
			name: "no issuer (iss)",
			registeredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
		},
		{
			name: "incorrect issuer (iss)",
			registeredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
				Issuer:    secure.Issuer + "a",
			},
		},
		{
			name: "no expires at (exp)",
			registeredClaims: jwt.RegisteredClaims{
				IssuedAt: jwt.NewNumericDate(time.Now()),
				Issuer:   secure.Issuer,
			},
		},
		{
			name: "expired",
			registeredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour + time.Minute)),
				Issuer:    secure.Issuer,
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			key := randomKey(t)
			handler := secure.NewSymmetricClaimHandler(key, 1*time.Minute)

			signedToken := sign(t, FooClaims{
				VersionedClaims: secure.VersionedClaims{
					RegisteredClaims: testCase.registeredClaims,
					Version:          secure.CurrentVersion,
				},
				Foo: "bar",
			}, key, jwt.SigningMethodHS256)

			fooClaims := &FooClaims{}

			err := handler.Verify(signedToken, fooClaims)
			if !errors.Is(err, secure.ErrSymmetricClaimOperation) {
				t.Errorf("want ErrSymmetricClaimOperation, got %v", err)
			}
		})
	}
}
