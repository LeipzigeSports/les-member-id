// Package svc contains implementations of HTTP request handlers to enable
// the functionality of the member ID service.
package svc

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/LeipzigeSports/les-member-id/internal/secure"
	"github.com/LeipzigeSports/les-member-id/internal/store"
	"github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
)

// StateConfig is a wrapper for handling state in authentication requests.
type StateConfig struct {
	Generator secure.StateGenerator
	TTL       time.Duration
}

// OIDCConfig is a wrapper for OIDC-related functionality.
type OIDCConfig struct {
	Provider     *oidc.Provider
	OAuth2Config oauth2.Config
	Verifier     *oidc.IDTokenVerifier
}

//nolint:tagliatelle  // casing is enforced by oauth protocol
type idTokenClaims struct {
	EmailVerified bool   `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}

//nolint:revive
type AuthService struct {
	baseURL               string
	state                 StateConfig
	cookie                CookieConfig
	expiringKeyStore      store.ExpiringKeyStore
	symmetricClaimHandler *secure.SymmetricClaimHandler
	oidc                  OIDCConfig
}

// NewAuthService creates a new handler for handling OAuth requests.
func NewAuthService(
	baseURL string,
	state StateConfig,
	cookie CookieConfig,
	expiringKeyStore store.ExpiringKeyStore,
	memberClaimHandler *secure.SymmetricClaimHandler,
	oidc OIDCConfig,
) *AuthService {
	return &AuthService{
		baseURL:               baseURL,
		state:                 state,
		cookie:                cookie,
		expiringKeyStore:      expiringKeyStore,
		symmetricClaimHandler: memberClaimHandler,
		oidc:                  oidc,
	}
}

// HandleRedirect is a http request handler. It redirects to the identity provider.
func (__this *AuthService) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	state, err := __this.state.Generator.GetState()
	if err != nil {
		http.Error(w, "failed to create state", http.StatusInternalServerError)
		log.Printf("failed to create state: %v", err)

		return
	}

	// key = value because we only care about the key
	err = __this.expiringKeyStore.Set(r.Context(), state, state, __this.state.TTL)
	if err != nil {
		http.Error(w, "failed to persist state", http.StatusBadGateway)
		log.Printf("failed to persist state: %v", err)

		return
	}

	http.Redirect(w, r, __this.oidc.OAuth2Config.AuthCodeURL(state), http.StatusFound)
}

// HandleCallback is a http request handler. It handles OAuth callbacks from the identification
// provider.
//
//nolint:funlen
func (__this *AuthService) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// get state from url params
	state := r.URL.Query().Get("state")

	// check if it's present in the backend
	_, err := __this.expiringKeyStore.Get(r.Context(), state)
	if err != nil {
		http.Error(w, "failed to verify state", http.StatusBadGateway)
		log.Printf("failed to verify state: %v", err)

		return
	}

	// get code from url params
	code := r.URL.Query().Get("code")

	// exchange token
	oauth2Token, err := __this.oidc.OAuth2Config.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "failed to exchange token with identity provider", http.StatusBadGateway)
		log.Printf("failed to exchange token with identity provider: %v", err)

		return
	}

	// extract id_token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in oauth2 token", http.StatusBadGateway)

		return
	}

	// check validity
	idToken, err := __this.oidc.Verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "failed to verify id_token", http.StatusBadGateway)
		log.Printf("failed to verify id_token: %v", err)

		return
	}

	var claims idTokenClaims

	err = idToken.Claims(&claims)
	if err != nil {
		http.Error(w, "cannot extract claims from id_token", http.StatusBadGateway)
		log.Printf("cannot extract claims from id_token: %v", err)

		return
	}

	if !claims.EmailVerified {
		http.Error(w, "email is not verified", http.StatusBadRequest)

		return
	}

	memberState, err := __this.state.Generator.GetState()
	if err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		log.Printf("failed to generate state: %v", err)

		return
	}

	signedToken, err := __this.symmetricClaimHandler.Sign(secure.MemberIDClaims{
		VersionedClaims: __this.symmetricClaimHandler.NewVersionedClaims(),
		GivenName:       claims.GivenName,
		FamilyName:      claims.FamilyName,
		State:           memberState,
	})
	if err != nil {
		http.Error(w, "failed to sign token", http.StatusInternalServerError)

		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    __this.cookie.Name,
		Value:   signedToken,
		Expires: time.Now().Add(__this.cookie.ExpiresIn),
		Secure:  strings.HasPrefix(__this.baseURL, "https://"),
		Path:    "/",
	})

	http.Redirect(w, r, __this.baseURL, http.StatusFound)
}
