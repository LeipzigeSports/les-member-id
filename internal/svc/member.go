package svc

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/LeipzigeSports/les-member-id/internal/secure"
	"github.com/LeipzigeSports/les-member-id/internal/store"
)

var errGenerateNewCode = errors.New("error during generation of new code")

type memberTemplate struct {
	Title          string
	Code           string
	CodeTTLSecsMax int
	CodeTTLSecs    int
	GivenName      string
	FamilyName     string
}

//nolint:revive
type MemberIDService struct {
	siteTitle             string
	template              *template.Template
	oauthLoginRedirectURL string
	expiringKeyStore      store.ExpiringKeyStore
	codeTTL               time.Duration
	codeGenerator         secure.CodeGenerator
	symmetricClaimHandler *secure.SymmetricClaimHandler
	cookie                CookieConfig
}

// NewMemberIDService creates a new handler for users to interact with their member ID.
func NewMemberIDService(
	siteTitle string,
	template *template.Template,
	oauthLoginRedirectURL string,
	expiringKeyStore store.ExpiringKeyStore,
	codeGenerator secure.CodeGenerator,
	codeTTL time.Duration,
	symmetricClaimHandler *secure.SymmetricClaimHandler,
	cookie CookieConfig,
) *MemberIDService {
	return &MemberIDService{
		siteTitle:             siteTitle,
		template:              template,
		oauthLoginRedirectURL: oauthLoginRedirectURL,
		expiringKeyStore:      expiringKeyStore,
		codeGenerator:         codeGenerator,
		codeTTL:               codeTTL,
		symmetricClaimHandler: symmetricClaimHandler,
		cookie:                cookie,
	}
}

// Handle is a http request handler. It checks for user authentication, redirects if none is provided,
// checks for a current member ID code, creates it if necessary, and presents the user with their member ID.
func (__this *MemberIDService) Handle(w http.ResponseWriter, r *http.Request) {
	// check for auth cookie
	cookie, err := r.Cookie(__this.cookie.Name)
	if err != nil {
		// if it doesn't exist, redirect to login
		if errors.Is(err, http.ErrNoCookie) {
			__this.redirectToLogin(w, r)

			return
		}

		// otherwise return error
		http.Error(w, "error fetching cookie", http.StatusInternalServerError)
		log.Printf("error fetching cookie: %v", err)

		return
	}

	// verify jwt
	signedToken := cookie.Value
	memberClaims := &secure.MemberIDClaims{}

	err = __this.symmetricClaimHandler.Verify(signedToken, memberClaims)
	if err != nil {
		// if validation failed, redirect to login
		__this.redirectToLogin(w, r)

		return
	}

	// check if a code is already present in the backend
	memberCode, memberCodeTTL, err := __this.expiringKeyStore.GetWithTTL(r.Context(), memberClaims.State)
	if err != nil {
		if !errors.Is(err, store.ErrKeyNotPresent) {
			http.Error(w, "error in connection to backend", http.StatusBadGateway)
			log.Printf("error in connection to backend: %v", err)

			return
		}

		// if no code is found, create a new one
		memberCode, err = __this.generateNewMemberCode(r.Context(), memberClaims.State)
		if err != nil {
			http.Error(w, "error in code generation", http.StatusBadGateway)
			log.Printf("error in code generation: %v", err)

			return
		}

		memberCodeTTL = __this.codeTTL
	}

	err = __this.template.ExecuteTemplate(w, "base.html", memberTemplate{
		Title:          __this.siteTitle,
		Code:           memberCode,
		CodeTTLSecsMax: int(__this.codeTTL.Seconds()),
		CodeTTLSecs:    int(memberCodeTTL.Seconds()),
		GivenName:      memberClaims.GivenName,
		FamilyName:     memberClaims.FamilyName,
	})
	if err != nil {
		http.Error(w, "failed to render website", http.StatusInternalServerError)
		log.Printf("failed to render website: %v", err)
	}
}

func (__this *MemberIDService) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, __this.oauthLoginRedirectURL, http.StatusFound)
}

func (__this *MemberIDService) generateNewMemberCode(ctx context.Context, state string) (string, error) {
	memberCode, err := __this.codeGenerator.GetCode()
	if err != nil {
		return "", fmt.Errorf("%w: %w", errGenerateNewCode, err)
	}

	// write code to store for verifier service (will only need the code, not the user state)
	err = __this.expiringKeyStore.Set(ctx, memberCode, memberCode, __this.codeTTL)
	if err != nil {
		return "", fmt.Errorf("%w, %w", errGenerateNewCode, err)
	}

	// write code to store for member service (will need store to look up from user cookie)
	err = __this.expiringKeyStore.Set(ctx, state, memberCode, __this.codeTTL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errGenerateNewCode, err)
	}

	return memberCode, nil
}
