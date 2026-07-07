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

func TestDeepAgentRuntimeSeparatesQuestionFromCLIOptions(t *testing.T) {
	workDir := t.TempDir()
	runtime := agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestDeepAgentRuntimeHelperProcess", "--"},
		Env:     []string{"AGENTROOM_DEEPAGENT_HELPER=1", "AGENTROOM_DEEPAGENT_REQUIRE_QUESTION_SEPARATOR=1"},
		WorkDir: workDir,
		Config:  "deepagent.toml",
		Timeout: 5 * time.Second,
	})

	response, err := runtime.Respond(context.Background(), agent.AgentRuntimeRequest{
		RunID:   "run_option_question",
		Agent:   model.Agent{Runtime: model.AgentRuntimeDeepAgent},
		Trigger: model.Message{Content: "--offline-smoke should be a question"},
	})
	if err != nil {
		t.Fatalf("expected option-like question to be positional, got %v", err)
	}
	if len(response.Artifacts) != 1 || response.Artifacts[0].Content != "# Report\n\nQuestion: --offline-smoke should be a question" {
		t.Fatalf("expected option-like question in report, got %#v", response.Artifacts)
	}
}

func TestDeepAgentRuntimeConcurrencyLimitWaitsAndHonorsCancellation(t *testing.T) {
	workDir := t.TempDir()
	runtime := agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
		Command:     os.Args[0],
		Args:        []string{"-test.run=TestDeepAgentRuntimeHelperProcess", "--"},
		Env:         []string{"AGENTROOM_DEEPAGENT_HELPER=1", "AGENTROOM_DEEPAGENT_HOLD=1"},
		WorkDir:     workDir,
		Config:      "deepagent.toml",
		Timeout:     5 * time.Second,
		Concurrency: 1,
	})

	firstStarted := make(chan error, 1)
	firstDone := make(chan error, 1)
	go func() {
		firstStarted <- nil
		_, err := runtime.Respond(context.Background(), agent.AgentRuntimeRequest{
			RunID:   "run_hold",
			Trigger: model.Message{Content: "hold the runtime"},
		})
		firstDone <- err
	}()
	<-firstStarted
	time.Sleep(200 * time.Millisecond)

	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := runtime.Respond(waitCtx, agent.AgentRuntimeRequest{
		RunID:   "run_waiter",
		Trigger: model.Message{Content: "must not start"},
	})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected waiting call to honor context deadline, got %v", err)
	}
	if err := <-firstDone; err != nil {
		t.Fatalf("expected first deepagent call to complete, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(workDir, "runs", "run_waiter", "report.md")); !os.IsNotExist(statErr) {
		t.Fatalf("expected canceled waiter not to start subprocess, stat err=%v", statErr)
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
			break
		}
	}
	if separator == -1 {
		os.Exit(2)
	}
	cliArgs := args[separator+1:]
	if os.Getenv("AGENTROOM_DEEPAGENT_REQUIRE_QUESTION_SEPARATOR") == "1" {
		foundQuestionSeparator := false
		for _, arg := range cliArgs {
			if arg == "--" {
				foundQuestionSeparator = true
				break
			}
		}
		if !foundQuestionSeparator {
			os.Exit(8)
		}
	}
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
		case "--":
			if i+1 < len(cliArgs) {
				question = cliArgs[i+1]
				i = len(cliArgs)
			}
		default:
			question = cliArgs[i]
		}
	}
	if os.Getenv("AGENTROOM_DEEPAGENT_HOLD") == "1" {
		time.Sleep(500 * time.Millisecond)
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
