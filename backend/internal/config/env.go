package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
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

type DeepAgentConfig struct {
	Command     string
	WorkDir     string
	Config      string
	Registry    string
	Timeout     time.Duration
	Concurrency int
}

const (
	AgentRuntimeTransportLocal = "local"
	AgentRuntimeTransportGRPC  = "grpc"
)

type AgentRuntimeConfig struct {
	Transport       string
	GRPCAddress     string
	GRPCInsecure    bool
	ServerName      string
	CAFile          string
	ClientCertFile  string
	ClientKeyFile   string
	LLMTimeout      time.Duration
	DeepTimeout     time.Duration
	MaxRequestBytes int
	MaxEventBytes   int
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

func LoadDeepAgentConfig() DeepAgentConfig {
	command := strings.TrimSpace(os.Getenv("DEEPAGENT_COMMAND"))
	if command == "" {
		command = "uv"
	}
	workDir := strings.TrimSpace(os.Getenv("DEEPAGENT_WORKDIR"))
	if workDir == "" {
		workDir = "../deepagent"
	}
	configPath := strings.TrimSpace(os.Getenv("DEEPAGENT_CONFIG"))
	if configPath == "" {
		configPath = "deepagent.toml"
	}
	registryPath := strings.TrimSpace(os.Getenv("DEEPAGENT_REGISTRY"))
	if registryPath == "" {
		registryPath = "agents.json"
	}
	timeout := 5 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("DEEPAGENT_TIMEOUT_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	concurrency := 1
	if raw := strings.TrimSpace(os.Getenv("DEEPAGENT_CONCURRENCY")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			concurrency = parsed
		}
	}
	return DeepAgentConfig{
		Command:     command,
		WorkDir:     workDir,
		Config:      configPath,
		Registry:    registryPath,
		Timeout:     timeout,
		Concurrency: concurrency,
	}
}

func LoadAgentRuntimeConfig() (AgentRuntimeConfig, error) {
	config := AgentRuntimeConfig{
		Transport:       strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_TRANSPORT"))),
		GRPCAddress:     strings.TrimSpace(os.Getenv("AGENT_RUNTIME_GRPC_ADDRESS")),
		GRPCInsecure:    strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_GRPC_INSECURE")), "true"),
		ServerName:      strings.TrimSpace(os.Getenv("AGENT_RUNTIME_GRPC_SERVER_NAME")),
		CAFile:          strings.TrimSpace(os.Getenv("AGENT_RUNTIME_GRPC_CA_FILE")),
		ClientCertFile:  strings.TrimSpace(os.Getenv("AGENT_RUNTIME_GRPC_CLIENT_CERT_FILE")),
		ClientKeyFile:   strings.TrimSpace(os.Getenv("AGENT_RUNTIME_GRPC_CLIENT_KEY_FILE")),
		LLMTimeout:      envDurationSeconds("AGENT_RUNTIME_LLM_TIMEOUT_SECONDS", 45*time.Second),
		DeepTimeout:     envDurationSeconds("AGENT_RUNTIME_DEEPAGENT_TIMEOUT_SECONDS", 5*time.Minute),
		MaxRequestBytes: envPositiveInt("AGENT_RUNTIME_MAX_REQUEST_BYTES", 8*1024*1024),
		MaxEventBytes:   envPositiveInt("AGENT_RUNTIME_MAX_EVENT_BYTES", 4*1024*1024),
	}
	if config.Transport == "" {
		config.Transport = AgentRuntimeTransportLocal
	}
	if config.GRPCAddress == "" {
		config.GRPCAddress = "127.0.0.1:50051"
	}
	if err := config.Validate(); err != nil {
		return AgentRuntimeConfig{}, err
	}
	return config, nil
}

func (c AgentRuntimeConfig) Validate() error {
	if c.Transport != AgentRuntimeTransportLocal && c.Transport != AgentRuntimeTransportGRPC {
		return errors.New("AGENT_RUNTIME_TRANSPORT must be local or grpc")
	}
	if c.LLMTimeout <= 0 || c.DeepTimeout <= 0 {
		return errors.New("Agent Runtime deadlines must be positive")
	}
	if c.MaxRequestBytes <= 0 || c.MaxEventBytes <= 0 {
		return errors.New("Agent Runtime message limits must be positive")
	}
	if c.Transport == AgentRuntimeTransportLocal {
		return nil
	}
	if c.GRPCAddress == "" {
		return errors.New("AGENT_RUNTIME_GRPC_ADDRESS is required for grpc transport")
	}
	if c.GRPCInsecure {
		return nil
	}
	if c.CAFile == "" {
		return errors.New("AGENT_RUNTIME_GRPC_CA_FILE is required unless grpc insecure mode is explicit")
	}
	if (c.ClientCertFile == "") != (c.ClientKeyFile == "") {
		return errors.New("Agent Runtime client certificate and key must be configured together")
	}
	for _, path := range []string{c.CAFile, c.ClientCertFile, c.ClientKeyFile} {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return errors.New("Agent Runtime TLS file is missing or unreadable: " + path)
		}
	}
	return nil
}

func envDurationSeconds(name string, fallback time.Duration) time.Duration {
	seconds := envPositiveInt(name, int(fallback/time.Second))
	return time.Duration(seconds) * time.Second
}

func envPositiveInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
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
