package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentroom/backend/internal/config"
)

func clearCollaborationRuntimeEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"COLLABORATION_RUNTIME_MODE", "COLLABORATION_RUNTIME_GRPC_ADDRESS",
		"COLLABORATION_RUNTIME_GRPC_INSECURE", "COLLABORATION_RUNTIME_GRPC_SERVER_NAME",
		"COLLABORATION_RUNTIME_GRPC_CA_FILE", "COLLABORATION_RUNTIME_GRPC_CLIENT_CERT_FILE",
		"COLLABORATION_RUNTIME_GRPC_CLIENT_KEY_FILE", "COLLABORATION_RUNTIME_TIMEOUT_SECONDS",
		"COLLABORATION_RUNTIME_MAX_REQUEST_BYTES", "COLLABORATION_RUNTIME_MAX_EVENT_BYTES",
		"COLLABORATION_RUNTIME_MAX_CHECKPOINT_BYTES", "COLLABORATION_DEFAULT_ENGINE",
		"COLLABORATION_DEFAULT_TRIGGER_MODE", "COLLABORATION_MAX_CONCURRENCY", "COLLABORATION_MAX_PENDING",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadCollaborationRuntimeConfigDefaultsToLegacy(t *testing.T) {
	clearCollaborationRuntimeEnv(t)

	got, err := config.LoadCollaborationRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != config.CollaborationRuntimeModeLegacy || got.Timeout != 5*time.Minute {
		t.Fatalf("unexpected defaults: %#v", got)
	}
	if got.MaxConcurrent != 4 || got.MaxPending != 64 {
		t.Fatalf("unexpected capacity defaults: %#v", got)
	}
	if got.MaxCheckpointBytes != 1<<20 || got.DefaultEngine != "native" || got.DefaultTriggerMode != "mention_only" {
		t.Fatalf("unexpected collaboration policy defaults: %#v", got)
	}
}

func TestLoadCollaborationRuntimeConfigAllowsExplicitInsecureRemote(t *testing.T) {
	clearCollaborationRuntimeEnv(t)
	t.Setenv("COLLABORATION_RUNTIME_MODE", "remote")
	t.Setenv("COLLABORATION_RUNTIME_GRPC_ADDRESS", "collaboration-runtime:50051")
	t.Setenv("COLLABORATION_RUNTIME_GRPC_INSECURE", "true")
	t.Setenv("COLLABORATION_RUNTIME_TIMEOUT_SECONDS", "90")
	t.Setenv("COLLABORATION_RUNTIME_MAX_CHECKPOINT_BYTES", "4096")
	t.Setenv("COLLABORATION_DEFAULT_ENGINE", "autogen")
	t.Setenv("COLLABORATION_DEFAULT_TRIGGER_MODE", "automatic")
	t.Setenv("COLLABORATION_MAX_CONCURRENCY", "2")
	t.Setenv("COLLABORATION_MAX_PENDING", "0")

	got, err := config.LoadCollaborationRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !got.GRPCInsecure || got.Timeout != 90*time.Second || got.MaxConcurrent != 2 || got.MaxPending != 0 {
		t.Fatalf("unexpected remote config: %#v", got)
	}
	if got.MaxCheckpointBytes != 4096 || got.DefaultEngine != "autogen" || got.DefaultTriggerMode != "automatic" {
		t.Fatalf("unexpected policy config: %#v", got)
	}
}

func TestLoadCollaborationRuntimeConfigRejectsUnknownMode(t *testing.T) {
	clearCollaborationRuntimeEnv(t)
	t.Setenv("COLLABORATION_RUNTIME_MODE", "automatic")

	if _, err := config.LoadCollaborationRuntimeConfig(); err == nil {
		t.Fatal("expected unknown mode to fail")
	}
}

func TestLoadCollaborationRuntimeConfigRequiresReadableTLSMaterial(t *testing.T) {
	clearCollaborationRuntimeEnv(t)
	t.Setenv("COLLABORATION_RUNTIME_MODE", "remote")
	if _, err := config.LoadCollaborationRuntimeConfig(); err == nil {
		t.Fatal("expected missing CA to fail")
	}

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COLLABORATION_RUNTIME_GRPC_CA_FILE", caPath)
	t.Setenv("COLLABORATION_RUNTIME_GRPC_CLIENT_CERT_FILE", "only-cert.pem")
	if _, err := config.LoadCollaborationRuntimeConfig(); err == nil {
		t.Fatal("expected incomplete client identity to fail")
	}
}

func TestLoadCollaborationRuntimeConfigRejectsInvalidCapacity(t *testing.T) {
	clearCollaborationRuntimeEnv(t)
	t.Setenv("COLLABORATION_MAX_PENDING", "-1")

	if _, err := config.LoadCollaborationRuntimeConfig(); err == nil {
		t.Fatal("expected negative pending capacity to fail")
	}
}

func TestLoadCollaborationRuntimeConfigRejectsInvalidDefaults(t *testing.T) {
	clearCollaborationRuntimeEnv(t)
	t.Setenv("COLLABORATION_DEFAULT_ENGINE", "direct")
	if _, err := config.LoadCollaborationRuntimeConfig(); err == nil {
		t.Fatal("expected invalid default engine to fail")
	}

	clearCollaborationRuntimeEnv(t)
	t.Setenv("COLLABORATION_DEFAULT_TRIGGER_MODE", "fanout")
	if _, err := config.LoadCollaborationRuntimeConfig(); err == nil {
		t.Fatal("expected invalid default trigger mode to fail")
	}

	clearCollaborationRuntimeEnv(t)
	t.Setenv("COLLABORATION_RUNTIME_MAX_CHECKPOINT_BYTES", "0")
	if _, err := config.LoadCollaborationRuntimeConfig(); err == nil {
		t.Fatal("expected invalid checkpoint limit to fail")
	}
}
