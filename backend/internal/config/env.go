package config

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = trimEnvValue(strings.TrimSpace(value))
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// DBConfig holds database configuration loaded from environment variables.
type DBConfig struct {
	Driver      string
	DSN         string
	AutoMigrate bool
}

type SecurityConfig struct {
	AdminAPIKey    string
	AllowedOrigins []string
}

// LoadDBConfig reads database configuration from environment variables.
func LoadDBConfig() DBConfig {
	return DBConfig{
		Driver:      os.Getenv("DB_DRIVER"),
		DSN:         os.Getenv("MYSQL_DSN"),
		AutoMigrate: os.Getenv("DB_AUTO_MIGRATE") == "true",
	}
}

func LoadSecurityConfig() SecurityConfig {
	return SecurityConfig{
		AdminAPIKey:    strings.TrimSpace(os.Getenv("ADMIN_API_KEY")),
		AllowedOrigins: splitCommaList(os.Getenv("ALLOWED_ORIGINS")),
	}
}

func trimEnvValue(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func splitCommaList(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
