package main

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	Port           string
	MongoURI       string
	DBName         string
	JWTSecret      []byte
	AdminEmail     string
	AllowedOrigins []string
}

// loadConfig reads configuration from the environment. JWT_SECRET is required:
// the server refuses to start without it rather than falling back to a guessable value.
func loadConfig() (Config, error) {
	cfg := Config{
		Port:           envOr("PORT", "10000"),
		MongoURI:       envOr("MONGO_URI", "mongodb://localhost:27017"),
		DBName:         envOr("DB_NAME", "lumina_db"),
		JWTSecret:      []byte(os.Getenv("JWT_SECRET")),
		AdminEmail:     normalizeEmail(os.Getenv("ADMIN_EMAIL")),
		AllowedOrigins: parseOrigins(envOr("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
	}

	if len(cfg.JWTSecret) < 32 {
		return cfg, errors.New("JWT_SECRET must be set to a random string of at least 32 characters")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// parseOrigins splits a comma-separated origin list, trimming whitespace and trailing slashes.
func parseOrigins(raw string) []string {
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSuffix(strings.TrimSpace(origin), "/")
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
