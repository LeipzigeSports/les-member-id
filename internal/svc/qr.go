package svc

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/LeipzigeSports/les-member-id/internal/shared"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

//nolint:revive
type QrCodeService struct {
	verifierBaseURL string
	pathName        string
	codeLength      int
	codeTTL         time.Duration
	encodeOptions   []qrcode.EncodeOption
	imageOptions    []standard.ImageOption
}

// NewQrCodeService creates a new handler for QR code creation.
func NewQrCodeService(
	verifierBaseURL string,
	pathName string,
	codeLength int,
	codeTTL time.Duration,
	encodeOptions []qrcode.EncodeOption,
	imageOptions []standard.ImageOption,
) *QrCodeService {
	return &QrCodeService{
		verifierBaseURL: verifierBaseURL,
		pathName:        pathName,
		codeLength:      codeLength,
		codeTTL:         codeTTL,
		encodeOptions:   encodeOptions,
		imageOptions:    imageOptions,
	}
}

type writeCloseAdapter struct {
	http.ResponseWriter
}

func (__this *writeCloseAdapter) Close() error {
	return nil
}

// Handle is a http request handler. It responds with a QR code that encodes a link
// to the verification page for the code in the query parameters.
func (__this *QrCodeService) Handle(w http.ResponseWriter, r *http.Request) {
	value := r.PathValue(__this.pathName)
	if value == "" {
		http.Error(w, "no value to encode provided", http.StatusBadRequest)

		return
	}

	if len(value) != __this.codeLength {
		http.Error(w, "invalid code provided", http.StatusBadRequest)

		return
	}

	fullURL := shared.MustJoinURLPath(__this.verifierBaseURL, value)

	qrCode, err := qrcode.NewWith(fullURL, __this.encodeOptions...)
	if err != nil {
		http.Error(w, "failed to generate qr code", http.StatusInternalServerError)
		log.Printf("failed to generate qr code: %v", err)

		return
	}

	responseWriteCloser := &writeCloseAdapter{w}
	qrCodeWriter := standard.NewWithWriter(responseWriteCloser, __this.imageOptions...)

	// before writing, send cache control header
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int(__this.codeTTL.Seconds())))

	err = qrCode.Save(qrCodeWriter)
	if err != nil {
		http.Error(w, "failed to write qr code", http.StatusInternalServerError)
		log.Printf("failed to write qr code: %v", err)

		return
	}
}
