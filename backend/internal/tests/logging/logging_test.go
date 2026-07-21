package logging_test

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"

	backendlogging "agentroom/backend/internal/logging"
)

func TestInitRedactsSensitiveFieldsAndEnvironmentCredentials(t *testing.T) {
	secret := "model-api-key-must-not-leak"
	t.Setenv("MODEL_API_KEY", secret)
	t.Setenv("LOG_FORMAT", "json")

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = original })
	backendlogging.Init()
	slog.Default().Error("provider failed", "error", "upstream echoed "+secret, "authorization", secret)
	_ = writer.Close()
	var output bytes.Buffer
	_, _ = output.ReadFrom(reader)

	if strings.Contains(output.String(), secret) {
		t.Fatalf("credential leaked in log: %s", output.String())
	}
	if !strings.Contains(output.String(), "[REDACTED]") {
		t.Fatalf("expected redaction marker, got %s", output.String())
	}
}
