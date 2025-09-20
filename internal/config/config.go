package config

import (
	"os"
	"path/filepath"
)

// Config holds application-wide configuration populated from environment variables.
type Config struct {
	OpenAIAPIKey    string
	GmailTokenPath  string
	CredentialsPath string
	AudioDir        string
	SecretsDir      string
}

// Load reads environment variables and returns Config with defaults applied.
func Load() *Config {
	cfg := &Config{
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		GmailTokenPath:  getEnv("GMAIL_TOKEN", ""),
		CredentialsPath: getEnv("GOOGLE_CREDENTIALS", ""),
		AudioDir:        getEnv("AUDIO_DIR", "audio"),
		SecretsDir:      getEnv("SECRETS_DIR", "secrets"),
	}
	if cfg.GmailTokenPath == "" {
		cfg.GmailTokenPath = filepath.Join(cfg.SecretsDir, "token.json")
	}
	if cfg.CredentialsPath == "" {
		cfg.CredentialsPath = filepath.Join(cfg.SecretsDir, "credentials.json")
	}
	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
