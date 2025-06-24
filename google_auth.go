package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

const (
	credentialsFile = "credentials.json" // Downloaded from Google Cloud Console
	tokenFile       = "token.json"       // Saved OAuth2 token
)

// GoogleAuth wraps oauth2 configuration and helpers.
type GoogleAuth struct {
	config *oauth2.Config
}

// newGoogleAuth creates GoogleAuth by reading credentials.json.
func newGoogleAuth() (*GoogleAuth, error) {
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}
	// Adjust redirect URL if running on non-local environment
	if config.RedirectURL == "" {
		config.RedirectURL = "http://localhost:8080/auth/google/callback"
	}
	return &GoogleAuth{config: config}, nil
}

// AuthURL generates Google OAuth consent URL.
func (ga *GoogleAuth) AuthURL(state string) string {
	return ga.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange exchanges code to token and persist it.
func (ga *GoogleAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	tok, err := ga.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	if err := saveToken(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// saveToken writes token to file path tokenFile.
func saveToken(token *oauth2.Token) error {
	f, err := os.Create(tokenFile)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// tokenFromFile retrieves token from local file.
func tokenFromFile() (*oauth2.Token, error) {
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	err = json.NewDecoder(f).Decode(&tok)
	return &tok, err
}

// Middleware to inject gmail Service (future use)
func gmailServiceFromToken(ctx context.Context, token *oauth2.Token, config *oauth2.Config) (*gmail.Service, error) {
	client := config.Client(ctx, token)
	return gmail.New(client)
}

// Simple in-memory state store.
var oauthState = make(map[string]time.Time)

func generateState() string {
	// naive random string
	return fmt.Sprintf("st-%d", time.Now().UnixNano())
}

func validateState(state string) bool {
	if exp, ok := oauthState[state]; ok {
		// expire after 5 mins
		if time.Since(exp) < 5*time.Minute {
			delete(oauthState, state)
			return true
		}
		delete(oauthState, state)
	}
	return false
}

// RegisterOAuthRoutes adds /auth/google and /auth/google/callback
func RegisterOAuthRoutes(app *fiber.App) error {
	ga, err := newGoogleAuth()
	if err != nil {
		return err
	}

	app.Get("/auth/google", func(c *fiber.Ctx) error {
		st := generateState()
		oauthState[st] = time.Now()
		url := ga.AuthURL(st)
		return c.Redirect(url, http.StatusTemporaryRedirect)
	})

	app.Get("/auth/google/callback", func(c *fiber.Ctx) error {
		state := c.Query("state")
		if !validateState(state) {
			return c.Status(http.StatusBadRequest).SendString("invalid state")
		}
		code := c.Query("code")
		tok, err := ga.Exchange(c.Context(), code)
		if err != nil {
			return c.Status(http.StatusInternalServerError).SendString(err.Error())
		}
		return c.JSON(tok)
	})

	return nil
}
