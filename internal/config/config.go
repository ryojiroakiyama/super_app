package config

import (
    "os"
    "path/filepath"

    "github.com/joho/godotenv"
)

// Config holds application-wide configuration populated from environment variables.
type Config struct {
	OpenAIAPIKey    string
	GmailTokenPath  string
	CredentialsPath string
	AudioDir        string
	SecretsDir      string
    DriveUploadEnabled bool
    DriveFolderID      string
}

// Load reads environment variables and returns Config with defaults applied.
func Load() *Config {
    // ルートの.envを読み込む（存在しなければ無視）
    _ = godotenv.Load()
	cfg := &Config{
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		GmailTokenPath:  getEnv("GMAIL_TOKEN", ""),
		CredentialsPath: getEnv("GOOGLE_CREDENTIALS", ""),
		AudioDir:        getEnv("AUDIO_DIR", "audio"),
		SecretsDir:      getEnv("SECRETS_DIR", "secrets"),
        DriveUploadEnabled: getEnvBool("DRIVE_UPLOAD_ENABLED", false),
        DriveFolderID:      getEnv("DRIVE_FOLDER_ID", ""),
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

func getEnvBool(key string, def bool) bool {
    v := getEnv(key, "")
    if v == "" {
        return def
    }
    switch v {
    case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
        return true
    case "0", "false", "FALSE", "False", "no", "NO", "off", "OFF":
        return false
    default:
        return def
    }
}
