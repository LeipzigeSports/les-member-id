package secure

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// CurrentVersion is the current version that JWTs with this module should be signed with.
	CurrentVersion = 1
	// Issuer is the name of this module that JWTs signed by this module should include.
	Issuer = "memberid"
)

// ErrSymmetricClaimOperation is a generic error that may be raised when issues arise during JWT validation
// or creation.
var ErrSymmetricClaimOperation = errors.New("error during jwt creation or validation")

// VersionedClaimsTracker defines functions that must be implemented by a set of versioned claims.
type VersionedClaimsTracker interface {
	jwt.Claims
	GetVersion() int
}

// VersionedClaims is a base set of claims with an additional version field that indicates this
// module version.
type VersionedClaims struct {
	jwt.RegisteredClaims

	Version int `json:"version"`
}

// GetVersion returns the version of the module with which these claims have been signed.
func (v *VersionedClaims) GetVersion() int {
	return v.Version
}

// MemberIDClaims is a set of claims to identify an active member.
type MemberIDClaims struct {
	VersionedClaims

	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
	State      string `json:"state"`
}

//nolint:revive
type SymmetricClaimHandler struct {
	secretKey []byte
	expiresIn time.Duration
}

// NewSymmetricClaimHandler returns a utility for singing and verifying JWTs using a secret key and a
// pre-defined expiration duration.
func NewSymmetricClaimHandler(secretKey []byte, expiresIn time.Duration) *SymmetricClaimHandler {
	return &SymmetricClaimHandler{
		secretKey: secretKey,
		expiresIn: expiresIn,
	}
}

// NewRegisteredClaims returns a base set of claims with iss, exp and iat.
func (__this *SymmetricClaimHandler) NewRegisteredClaims() jwt.RegisteredClaims {
	now := time.Now()

	return jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(__this.expiresIn)),
		Issuer:    Issuer,
	}
}

// NewVersionedClaims returns a base set of claims that contains iss, exp and iat claims, as well as
// a version field that indicates the version of this module.
func (__this *SymmetricClaimHandler) NewVersionedClaims() VersionedClaims {
	return VersionedClaims{
		RegisteredClaims: __this.NewRegisteredClaims(),
		Version:          CurrentVersion,
	}
}

// Sign creates a new signed JWT using HS256 with its own symmetric key.
func (__this *SymmetricClaimHandler) Sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(__this.secretKey)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrSymmetricClaimOperation, err)
	}

	return signedToken, nil
}

// Verify checks the signature on a signed JWT using its own symmetric key. If successful, its contents are read
// into destClaims.
func (__this *SymmetricClaimHandler) Verify(signedToken string, destClaims VersionedClaimsTracker) error {
	token, err := jwt.ParseWithClaims(signedToken, destClaims, func(_ *jwt.Token) (any, error) {
		return __this.secretKey, nil
	}, jwt.WithIssuer(Issuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithValidMethods([]string{
		jwt.SigningMethodHS256.Alg(),
	}))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSymmetricClaimOperation, err)
	}

	if !token.Valid {
		return fmt.Errorf("%w: invalid token", ErrSymmetricClaimOperation)
	}

	// check for version
	if destClaims.GetVersion() != CurrentVersion {
		return fmt.Errorf(
			"%w: version mismatch: expected %d, got %d",
			ErrSymmetricClaimOperation, CurrentVersion, destClaims.GetVersion(),
		)
	}

	return nil
}
