package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentroom/backend/internal/config"
)

func clearAgentRuntimeEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"AGENT_RUNTIME_TRANSPORT", "AGENT_RUNTIME_GRPC_ADDRESS",
		"AGENT_RUNTIME_GRPC_INSECURE", "AGENT_RUNTIME_GRPC_SERVER_NAME",
		"AGENT_RUNTIME_GRPC_CA_FILE", "AGENT_RUNTIME_GRPC_CLIENT_CERT_FILE",
		"AGENT_RUNTIME_GRPC_CLIENT_KEY_FILE", "AGENT_RUNTIME_LLM_TIMEOUT_SECONDS",
		"AGENT_RUNTIME_DEEPAGENT_TIMEOUT_SECONDS", "AGENT_RUNTIME_MAX_REQUEST_BYTES",
		"AGENT_RUNTIME_MAX_EVENT_BYTES",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadAgentRuntimeConfigDefaultsToLocal(t *testing.T) {
	clearAgentRuntimeEnv(t)

	got, err := config.LoadAgentRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Transport != config.AgentRuntimeTransportLocal || got.LLMTimeout != 45*time.Second || got.DeepTimeout != 5*time.Minute {
		t.Fatalf("unexpected defaults: %#v", got)
	}
}

func TestLoadAgentRuntimeConfigAllowsExplicitInsecureGRPC(t *testing.T) {
	clearAgentRuntimeEnv(t)
	t.Setenv("AGENT_RUNTIME_TRANSPORT", "grpc")
	t.Setenv("AGENT_RUNTIME_GRPC_ADDRESS", "agent-runtime:50051")
	t.Setenv("AGENT_RUNTIME_GRPC_INSECURE", "true")
	t.Setenv("AGENT_RUNTIME_LLM_TIMEOUT_SECONDS", "12")

	got, err := config.LoadAgentRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !got.GRPCInsecure || got.LLMTimeout != 12*time.Second {
		t.Fatalf("unexpected grpc config: %#v", got)
	}
}

func TestLoadAgentRuntimeConfigRequiresReadableTLSMaterial(t *testing.T) {
	clearAgentRuntimeEnv(t)
	t.Setenv("AGENT_RUNTIME_TRANSPORT", "grpc")
	if _, err := config.LoadAgentRuntimeConfig(); err == nil {
		t.Fatal("expected missing CA to fail")
	}

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_RUNTIME_GRPC_CA_FILE", caPath)
	t.Setenv("AGENT_RUNTIME_GRPC_CLIENT_CERT_FILE", "only-cert.pem")
	if _, err := config.LoadAgentRuntimeConfig(); err == nil {
		t.Fatal("expected incomplete client identity to fail")
	}
}
