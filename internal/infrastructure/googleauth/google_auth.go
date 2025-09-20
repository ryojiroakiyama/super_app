package googleauth

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "math/rand"
    "net"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/gmail/v1"
)

const (
	credentialsFile = "credentials.json" // default file path, can be overridden by env GOOGLE_CREDENTIALS
	tokenFile       = "token.json"       // default token path, can be overridden by env GMAIL_TOKEN
)

// GoogleAuth wraps oauth2 configuration and helpers.
type GoogleAuth struct {
	config *oauth2.Config
}

// NewGoogleAuth creates GoogleAuth by reading credentials.json (or env-specified path).
func NewGoogleAuth() (*GoogleAuth, error) {
	credPath := os.Getenv("GOOGLE_CREDENTIALS")
	if credPath == "" {
		secretsDir := os.Getenv("SECRETS_DIR")
		if secretsDir == "" {
			secretsDir = "secrets"
		}
		credPath = filepath.Join(secretsDir, credentialsFile)
	}
    b, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}
    log.Printf("[auth] using credentials: %s", credPath)
    log.Printf("[auth] client_id: %s", config.ClientID)
    // Restore default server-style redirect URL if not specified in credentials
    if config.RedirectURL == "" {
        config.RedirectURL = "http://localhost:8080/auth/google/callback"
    }
	return &GoogleAuth{config: config}, nil
}

// AuthURL generates Google OAuth consent URL.
func (ga *GoogleAuth) AuthURL(state string) string {
	return ga.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// SetRedirectURL overrides the redirect URL (useful for loopback CLI flow).
func (ga *GoogleAuth) SetRedirectURL(redirect string) {
    ga.config.RedirectURL = redirect
}

// ObtainTokenInteractive starts a temporary local HTTP server (loopback flow),
// opens the browser URL (printed to stdout), captures the auth code and saves token.
func ObtainTokenInteractive(ctx context.Context) error {
    ga, err := NewGoogleAuth()
    if err != nil {
        return err
    }

    // Fixed loopback port (8080) for redirect handling
    ln, err := net.Listen("tcp", "127.0.0.1:8080")
    if err != nil {
        return fmt.Errorf("listen 127.0.0.1:8080: %w", err)
    }
    defer ln.Close()
    // Use host for redirect URL to match OAuth client configuration (default: localhost)
    host := os.Getenv("GOOGLE_LOOPBACK_HOST")
    if host == "" {
        host = "localhost"
    }
    addr := host + ":8080"

    // Keep path consistent with previous server callback (allow override)
    callbackPath := "/auth/google/callback"
    if p := os.Getenv("GOOGLE_CALLBACK_PATH"); p != "" {
        callbackPath = p
    }
    redirectURL := fmt.Sprintf("http://%s%s", addr, callbackPath)
    ga.SetRedirectURL(redirectURL)

    // Generate state
    rand.Seed(time.Now().UnixNano())
    state := fmt.Sprintf("st-%d", rand.Int63())

    // Build URL
    url := ga.AuthURL(state)
    log.Printf("[auth] Open this URL to authorize:\n%s", url)

    // Minimal HTTP handler to receive code
    codeCh := make(chan string, 1)
    srv := &http.Server{}
    mux := http.NewServeMux()
    mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Query().Get("state") != state {
            http.Error(w, "state mismatch", http.StatusBadRequest)
            return
        }
        code := r.URL.Query().Get("code")
        if code == "" {
            http.Error(w, "code missing", http.StatusBadRequest)
            return
        }
        fmt.Fprint(w, "Authorization received. You can close this tab.")
        codeCh <- code
    })
    srv.Handler = mux

    go func() {
        _ = srv.Serve(ln)
    }()

    // Wait for code or timeout
    var code string
    select {
    case code = <-codeCh:
        // proceed
    case <-time.After(5 * time.Minute):
        _ = srv.Close()
        return fmt.Errorf("authorization timeout")
    }

    // Exchange and save token
    if _, err := ga.Exchange(ctx, code); err != nil {
        _ = srv.Close()
        return err
    }
    _ = srv.Close()
    return nil
}

// Exchange exchanges code to token and persist it.
func (ga *GoogleAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	tok, err := ga.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	if err := SaveToken(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// SaveToken writes token to file path tokenFile.
func SaveToken(token *oauth2.Token) error {
	tokenPath := os.Getenv("GMAIL_TOKEN")
	if tokenPath == "" {
		secretsDir := os.Getenv("SECRETS_DIR")
		if secretsDir == "" {
			secretsDir = "secrets"
		}
		tokenPath = filepath.Join(secretsDir, tokenFile)
	}
	f, err := os.Create(tokenPath)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// TokenFromFile retrieves token from local file.
func TokenFromFile() (*oauth2.Token, error) {
	tokenPath := os.Getenv("GMAIL_TOKEN")
	if tokenPath == "" {
		secretsDir := os.Getenv("SECRETS_DIR")
		if secretsDir == "" {
			secretsDir = "secrets"
		}
		tokenPath = filepath.Join(secretsDir, tokenFile)
	}
	f, err := os.Open(tokenPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	err = json.NewDecoder(f).Decode(&tok)
	return &tok, err
}

// GmailServiceFromToken returns Gmail service from token and config.
func GmailServiceFromToken(ctx context.Context, token *oauth2.Token, config *oauth2.Config) (*gmail.Service, error) {
	client := config.Client(ctx, token)
	return gmail.New(client)
}

// BuildGmailService is a helper that loads token and credentials then builds Gmail API service.
func BuildGmailService(ctx context.Context) (*gmail.Service, error) {
	tok, err := TokenFromFile()
	if err != nil {
		return nil, err
	}
	ga, err := NewGoogleAuth()
	if err != nil {
		return nil, err
	}
	return GmailServiceFromToken(ctx, tok, ga.config)
}

// (Config) accessor is unused in CLI mode and intentionally omitted.
