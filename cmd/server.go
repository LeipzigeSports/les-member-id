package cmd

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/LeipzigeSports/les-member-id/internal/secure"
	"github.com/LeipzigeSports/les-member-id/internal/shared"
	"github.com/LeipzigeSports/les-member-id/internal/store"
	"github.com/LeipzigeSports/les-member-id/internal/svc"
	"github.com/coreos/go-oidc"
	"github.com/redis/go-redis/v9"
	altsrc "github.com/urfave/cli-altsrc/v3"
	"github.com/urfave/cli-altsrc/v3/yaml"
	"github.com/urfave/cli/v3"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"golang.org/x/oauth2"
)

const (
	defaultConfigFilePath = "config.yaml"
	defaultBaseURL        = "http://localhost:8080"
	defaultHost           = "0.0.0.0"
	defaultPort           = 8080
	defaultCodeLength     = 8
	defaultCodeAlphabet   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	defaultCodeTTL        = 5 * time.Minute
	defaultQrFgColorHex   = "000000"
	defaultQrBgColorHex   = "ffffff"
	defaultQrBlockWidth   = 16
	defaultOAuthTimeout   = 2 * time.Minute
	defaultOAuthCookieTTL = 1 * time.Hour
	defaultSiteTitle      = "Digital Member ID"
)

// ErrServerCommand is a generic error for issues during the execution of the server run command.
var ErrServerCommand = errors.New("error during server command execution")

type serverConfig struct {
	configFilePath    string
	baseURL           string
	host              string
	port              int
	codeLength        int
	codeAlphabet      string
	codeTTL           time.Duration
	secretKey         string
	qrFgColorHex      string
	qrBgColorHex      string
	qrBlockWidth      uint8
	oauthRealmURL     string
	oauthClientID     string
	oauthClientSecret string
	oauthTimeout      time.Duration
	oauthCookieTRL    time.Duration
	redisURL          string
	siteTitle         string
}

const (
	pathLogin     = "/oauth/login"
	pathCallback  = "/oauth/callback"
	pathVerify    = "/v"
	pathParamCode = "code"
)

const authCookieName = "member_id_token"

const (
	serverShutdownTimeout   = 10 * time.Second
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 5 * time.Second
	serverReadHeaderTimeout = 5 * time.Second
	serverIdleTimeout       = 30 * time.Second
)

const (
	redisAuthStatePrefix = "auth"
	redisCodePrefix      = "code"
)

//nolint:funlen
func runServer(ctx context.Context, serverConfig serverConfig) error {
	// set up secret key for jws
	secretKey := []byte(serverConfig.secretKey)
	claimHandler := secure.NewSymmetricClaimHandler(secretKey, serverConfig.oauthCookieTRL)

	// set up oidc provider
	provider, err := oidc.NewProvider(ctx, serverConfig.oauthRealmURL)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrServerCommand, err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     serverConfig.oauthClientID,
		ClientSecret: serverConfig.oauthClientSecret,
		RedirectURL:  shared.MustJoinURLPath(serverConfig.baseURL, pathCallback),
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	oidcConfig := &oidc.Config{
		ClientID: serverConfig.oauthClientID,
	}

	verifier := provider.Verifier(oidcConfig)

	// set up redis
	rdbOpts, err := redis.ParseURL(serverConfig.redisURL)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrServerCommand, err)
	}

	rdb := redis.NewClient(rdbOpts)

	// set up services
	qrCodeService := svc.NewQrCodeService(
		shared.MustJoinURLPath(serverConfig.baseURL, pathVerify),
		pathParamCode,
		serverConfig.codeLength,
		serverConfig.codeTTL,
		[]qrcode.EncodeOption{
			qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionLow),
		},
		[]standard.ImageOption{
			standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
			standard.WithFgColorRGBHex("#" + serverConfig.qrFgColorHex),
			standard.WithBgColorRGBHex("#" + serverConfig.qrBgColorHex),
			standard.WithQRWidth(serverConfig.qrBlockWidth),
		},
	)

	authService := svc.NewAuthService(
		serverConfig.baseURL,
		svc.StateConfig{
			Generator: secure.NewCSPRNGStateGenerator(),
			TTL:       serverConfig.oauthTimeout,
		},
		svc.CookieConfig{
			Name:      authCookieName,
			ExpiresIn: serverConfig.oauthCookieTRL,
		},
		store.NewRedisExpiringKeyStore(rdb, redisAuthStatePrefix),
		claimHandler,
		svc.OIDCConfig{
			Provider:     provider,
			OAuth2Config: oauth2Config,
			Verifier:     verifier,
		},
	)

	verifierTemplate, err := template.New("").ParseFiles("./views/base.html", "./views/verify.html")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrServerCommand, err)
	}

	verifierService := svc.NewVerifierService(
		serverConfig.siteTitle,
		verifierTemplate,
		store.NewRedisExpiringKeyStore(rdb, redisCodePrefix),
		serverConfig.codeLength,
		pathParamCode,
	)

	memberTemplate, err := template.New("").ParseFiles("./views/base.html", "./views/member.html")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrServerCommand, err)
	}

	memberIDService := svc.NewMemberIDService(
		serverConfig.siteTitle,
		memberTemplate,
		shared.MustJoinURLPath(serverConfig.baseURL, pathLogin),
		store.NewRedisExpiringKeyStore(rdb, redisCodePrefix),
		secure.NewNanoIDCodeGenerator(serverConfig.codeLength, serverConfig.codeAlphabet),
		serverConfig.codeTTL,
		claimHandler,
		svc.CookieConfig{
			Name:      authCookieName,
			ExpiresIn: serverConfig.oauthCookieTRL,
		},
	)

	// set up mux
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc(fmt.Sprintf("/qr/{%s}", pathParamCode), qrCodeService.Handle)
	mux.HandleFunc(fmt.Sprintf("/v/{%s}", pathParamCode), verifierService.Handle)
	mux.HandleFunc(pathCallback, authService.HandleCallback)
	mux.HandleFunc(pathLogin, authService.HandleRedirect)
	mux.HandleFunc("/", memberIDService.Handle)

	// set up server
	addr := fmt.Sprintf("%s:%d", serverConfig.host, serverConfig.port)
	server := http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       serverReadTimeout,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	// run server
	go func() {
		log.Printf("running server at %s", addr)

		err = server.ListenAndServe()
		if err != nil {
			// check if shutdown was intended
			if errors.Is(err, http.ErrServerClosed) {
				log.Println("server closed gracefully")

				return
			}

			log.Printf("server closed unexpectedly: %v", err)
		}
	}()

	// set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// wait for signal
	<-sigChan

	// shut down server
	shutdownCtx, cancelShutdownCtx := context.WithTimeout(ctx, serverShutdownTimeout)
	defer cancelShutdownCtx()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrServerCommand, err)
	}

	return nil
}

func yamlSource(name string, ptr *string) *altsrc.ValueSource {
	return yaml.YAML(name, altsrc.NewStringPtrSourcer(ptr))
}

// BuildServerCommand returns a sub-command for starting a server that exposes the member ID service.
//
//nolint:lll,funlen
func BuildServerCommand() *cli.Command {
	serverConfig := serverConfig{}
	configFilePath := &serverConfig.configFilePath

	return &cli.Command{
		Name:  "run",
		Usage: "runs the member id server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Value:       defaultConfigFilePath,
				Usage:       "path to config file",
				Destination: configFilePath,
			},
			&cli.StringFlag{
				Name:        "base-url",
				Value:       defaultBaseURL,
				Usage:       "url where server will be reachable",
				Destination: &serverConfig.baseURL,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("BASE_URL"), yamlSource("baseUrl", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "host",
				Value:       defaultHost,
				Aliases:     []string{"h"},
				Usage:       "server host",
				Destination: &serverConfig.host,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("SERVER_HOST"), yamlSource("server.host", configFilePath)),
			},
			&cli.IntFlag{
				Name:        "port",
				Value:       defaultPort,
				Aliases:     []string{"p"},
				Usage:       "server port",
				Destination: &serverConfig.port,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("SERVER_PORT"), yamlSource("server.port", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "secret-key",
				Usage:       "secret key for signing JWTs",
				Destination: &serverConfig.secretKey,
				Required:    true,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("SECRET_KEY"), yamlSource("secretKey", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "qr-foreground-color-hex",
				Value:       defaultQrFgColorHex,
				Usage:       "QR code foreground color in hex notation",
				Destination: &serverConfig.qrFgColorHex,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("QR_FOREGROUND_COLOR_HEX"), yamlSource("qr.fgColorHex", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "qr-background-color-hex",
				Value:       defaultQrBgColorHex,
				Usage:       "QR code background color in hex notation",
				Destination: &serverConfig.qrBgColorHex,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("QR_BACKGROUND_COLOR_HEX"), yamlSource("qr.bgColorHex", configFilePath)),
			},
			&cli.Uint8Flag{
				Name:        "qr-block-width",
				Value:       defaultQrBlockWidth,
				Usage:       "QR code block width",
				Destination: &serverConfig.qrBlockWidth,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("QR_BLOCK_WIDTH"), yamlSource("qr.blockWidth", configFilePath)),
			},
			&cli.IntFlag{
				Name:        "code-length",
				Value:       defaultCodeLength,
				Usage:       "validation code length",
				Destination: &serverConfig.codeLength,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("CODE_LENGTH"), yamlSource("code.length", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "code-alphabet",
				Value:       defaultCodeAlphabet,
				Usage:       "validation code characters",
				Destination: &serverConfig.codeAlphabet,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("CODE_ALPHABET"), yamlSource("code.alphabet", configFilePath)),
			},
			&cli.DurationFlag{
				Name:        "code-ttl",
				Value:       defaultCodeTTL,
				Usage:       "validation code time to live",
				Destination: &serverConfig.codeTTL,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("CODE_TTL"), yamlSource("code.ttl", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "oauth-realm-url",
				Usage:       "url to realm of oauth provider",
				Required:    true,
				Destination: &serverConfig.oauthRealmURL,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("OAUTH_REALM_URL"), yamlSource("oauth.realmUrl", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "oauth-client-id",
				Usage:       "oauth client id",
				Required:    true,
				Destination: &serverConfig.oauthClientID,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("OAUTH_CLIENT_ID"), yamlSource("oauth.clientId", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "oauth-client-secret",
				Usage:       "oauth client secret",
				Required:    true,
				Destination: &serverConfig.oauthClientSecret,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("OAUTH_CLIENT_SECRET"), yamlSource("oauth.clientSecret", configFilePath)),
			},
			&cli.DurationFlag{
				Name:        "oauth-timeout",
				Usage:       "duration until oauth login attempt is invalidated",
				Value:       defaultOAuthTimeout,
				Destination: &serverConfig.oauthTimeout,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("OAUTH_TIMEOUT"), yamlSource("oauth.timeout", configFilePath)),
			},
			&cli.DurationFlag{
				Name:        "oauth-cookie-ttl",
				Usage:       "duration until user session expires",
				Value:       defaultOAuthCookieTTL,
				Destination: &serverConfig.oauthCookieTRL,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("OAUTH_COOKIE_TTL"), yamlSource("oauth.cookieTtl", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "redis-url",
				Usage:       "url to redis database",
				Required:    true,
				Destination: &serverConfig.redisURL,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("REDIS_URL"), yamlSource("redis.url", configFilePath)),
			},
			&cli.StringFlag{
				Name:        "site-title",
				Usage:       "site title to show in browser",
				Value:       defaultSiteTitle,
				Destination: &serverConfig.siteTitle,
				Sources:     cli.NewValueSourceChain(cli.EnvVar("SITE_TITLE"), yamlSource("siteTitle", configFilePath)),
			},
		},
		Action: func(ctx context.Context, _ *cli.Command) error {
			err := runServer(ctx, serverConfig)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrServerCommand, err)
			}

			return nil
		},
	}
}
