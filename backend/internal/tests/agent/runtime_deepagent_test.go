package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
)

func TestDeepAgentRuntimeRunsCLIAndReturnsReportArtifact(t *testing.T) {
	workDir := t.TempDir()
	runtime := agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestDeepAgentRuntimeHelperProcess", "--"},
		Env:     []string{"AGENTROOM_DEEPAGENT_HELPER=1"},
		WorkDir: workDir,
		Config:  "deepagent.toml",
		Timeout: 5 * time.Second,
	})

	response, err := runtime.Respond(context.Background(), agent.AgentRuntimeRequest{
		RunID:   "run_test",
		Agent:   model.Agent{Runtime: model.AgentRuntimeDeepAgent},
		Trigger: model.Message{Content: "@Research current model parameter counts"},
	})
	if err != nil {
		t.Fatalf("expected deepagent runtime to succeed, got %v", err)
	}
	if strings.Contains(response.Content, "# Report") {
		t.Fatalf("expected short room message, got full report %q", response.Content)
	}
	if len(response.Artifacts) != 1 || response.Artifacts[0].Type != "markdown_report" || response.Artifacts[0].MIMEType != "text/markdown" {
		t.Fatalf("expected markdown report artifact, got %#v", response.Artifacts)
	}
	if response.Artifacts[0].ID != "report" || response.Artifacts[0].Content != "# Report\n\nQuestion: @Research current model parameter counts" {
		t.Fatalf("expected report artifact content, got %#v", response.Artifacts[0])
	}
}

func TestDeepAgentRuntimeStripsAgentMentionFromQuestion(t *testing.T) {
	workDir := t.TempDir()
	runtime := agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestDeepAgentRuntimeHelperProcess", "--"},
		Env:     []string{"AGENTROOM_DEEPAGENT_HELPER=1"},
		WorkDir: workDir,
		Config:  "deepagent.toml",
		Timeout: 5 * time.Second,
	})

	response, err := runtime.Respond(context.Background(), agent.AgentRuntimeRequest{
		RunID: "run_strip_mention",
		Agent: model.Agent{
			Runtime: model.AgentRuntimeDeepAgent,
			Mention: "@Research",
		},
		Trigger: model.Message{Content: "@Research current model parameter counts"},
	})
	if err != nil {
		t.Fatalf("expected deepagent runtime to succeed, got %v", err)
	}
	if len(response.Artifacts) != 1 || response.Artifacts[0].Content != "# Report\n\nQuestion: current model parameter counts" {
		t.Fatalf("expected stripped question in report artifact, got %#v", response.Artifacts)
	}
}

func TestDeepAgentRuntimeReturnsCommandFailure(t *testing.T) {
	workDir := t.TempDir()
	runtime := agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestDeepAgentRuntimeHelperProcess", "--", "--fail"},
		Env:     []string{"AGENTROOM_DEEPAGENT_HELPER=1"},
		WorkDir: workDir,
		Config:  "deepagent.toml",
		Timeout: 5 * time.Second,
	})

	_, err := runtime.Respond(context.Background(), agent.AgentRuntimeRequest{
		RunID:   "run_fail",
		Trigger: model.Message{Content: "@Research fail"},
	})
	if err == nil || !strings.Contains(err.Error(), "deepagent command failed") {
		t.Fatalf("expected deepagent command failure, got %v", err)
	}
}

func TestDeepAgentRuntimeHelperProcess(t *testing.T) {
	if os.Getenv("AGENTROOM_DEEPAGENT_HELPER") != "1" {
		return
	}
	args := os.Args
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
		}
	}
	if separator == -1 {
		os.Exit(2)
	}
	cliArgs := args[separator+1:]
	for _, arg := range cliArgs {
		if arg == "--fail" {
			_, _ = os.Stderr.WriteString("simulated deepagent failure")
			os.Exit(7)
		}
	}
	runID := ""
	question := ""
	for i := 0; i < len(cliArgs); i++ {
		switch cliArgs[i] {
		case "--run-id":
			i++
			if i < len(cliArgs) {
				runID = cliArgs[i]
			}
		case "--config":
			i++
		default:
			question = cliArgs[i]
		}
	}
	if runID == "" {
		os.Exit(3)
	}
	reportDir := filepath.Join("runs", runID)
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		os.Exit(4)
	}
	content := "# Report\n\nQuestion: " + question + "\n"
	if err := os.WriteFile(filepath.Join(reportDir, "report.md"), []byte(content), 0o644); err != nil {
		os.Exit(5)
	}
	os.Exit(0)
}

