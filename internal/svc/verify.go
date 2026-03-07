package svc

import (
	"errors"
	"html/template"
	"log"
	"net/http"

	"github.com/LeipzigeSports/les-member-id/internal/store"
)

//nolint:revive
type VerifierService struct {
	siteTitle        string
	template         *template.Template
	expiringKeyStore store.ExpiringKeyStore
	codeLength       int
	pathName         string
}

type verifierTemplate struct {
	Title string
	Valid bool
}

// NewVerifierService creates a new handler for member ID verification requests.
func NewVerifierService(
	siteTitle string,
	template *template.Template,
	expiringKeyStore store.ExpiringKeyStore,
	codeLength int,
	pathName string,
) *VerifierService {
	return &VerifierService{
		siteTitle:        siteTitle,
		template:         template,
		expiringKeyStore: expiringKeyStore,
		codeLength:       codeLength,
		pathName:         pathName,
	}
}

// Handle is a http request handler. It checks whether the code in the query parameters is assigned to an
// active member and returns a respective response.
func (__this *VerifierService) Handle(w http.ResponseWriter, r *http.Request) {
	value := r.PathValue(__this.pathName)
	if value == "" {
		http.Error(w, "no value provided", http.StatusBadRequest)

		return
	}

	if len(value) != __this.codeLength {
		http.Error(w, "invalid code provided", http.StatusBadRequest)

		return
	}

	// check if code is valid
	isCodeValid := true

	_, err := __this.expiringKeyStore.Get(r.Context(), value)
	if err != nil {
		if !errors.Is(err, store.ErrKeyNotPresent) {
			http.Error(w, "error when accessing backend", http.StatusBadGateway)
			log.Printf("error when accessing backend: %v", err)

			return
		}

		// code is not present, therefore scanned code is invalid
		isCodeValid = false
	}

	err = __this.template.ExecuteTemplate(w, "base.html", verifierTemplate{
		Title: __this.siteTitle,
		Valid: isCodeValid,
	})
	if err != nil {
		http.Error(w, "failed to render website", http.StatusInternalServerError)
		log.Printf("failed to render website: %v", err)
	}
}
