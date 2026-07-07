package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agentroom/backend/internal/model"
)

type DeepAgentRuntimeConfig struct {
	Command     string
	Args        []string
	Env         []string
	WorkDir     string
	Config      string
	Timeout     time.Duration
	Concurrency int
}

type DeepAgentRuntime struct {
	config DeepAgentRuntimeConfig
	sem    chan struct{}
}

func NewDeepAgentRuntime(config DeepAgentRuntimeConfig) *DeepAgentRuntime {
	if strings.TrimSpace(config.Command) == "" {
		config.Command = "uv"
	}
	if len(config.Args) == 0 {
		config.Args = []string{"run", "deepagent-research"}
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 1
	}
	return &DeepAgentRuntime{
		config: config,
		sem:    make(chan struct{}, config.Concurrency),
	}
}

func (r *DeepAgentRuntime) Name() string {
	return model.AgentRuntimeDeepAgent
}

func (r *DeepAgentRuntime) Respond(ctx context.Context, request AgentRuntimeRequest) (AgentRuntimeResponse, error) {
	runID := strings.TrimSpace(request.RunID)
	if runID == "" {
		runID = model.NewID("deepagent")
	}
	question := deepAgentQuestion(request.Agent, request.Trigger)
	if question == "" {
		return AgentRuntimeResponse{}, fmt.Errorf("deepagent question cannot be empty")
	}

	requestCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	args := append([]string{}, r.config.Args...)
	if strings.TrimSpace(r.config.Config) != "" {
		args = append(args, "--config", r.config.Config)
	}
	args = append(args, "--run-id", runID, "--", question)

	cmd := exec.CommandContext(requestCtx, r.config.Command, args...)
	if strings.TrimSpace(r.config.WorkDir) != "" {
		cmd.Dir = r.config.WorkDir
	}
	if len(r.config.Env) > 0 {
		cmd.Env = append(os.Environ(), r.config.Env...)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := r.acquire(requestCtx); err != nil {
		return AgentRuntimeResponse{}, err
	}
	defer r.release()

	if err := cmd.Run(); err != nil {
		return AgentRuntimeResponse{}, fmt.Errorf("deepagent command failed: %w: %s", err, shortCommandOutput(output.String()))
	}

	reportPath := filepath.Join(r.config.WorkDir, "runs", runID, "report.md")
	reportBytes, err := os.ReadFile(reportPath)
	if err != nil {
		return AgentRuntimeResponse{}, fmt.Errorf("read deepagent report: %w", err)
	}
	report := strings.TrimSpace(string(reportBytes))
	if report == "" {
		return AgentRuntimeResponse{}, fmt.Errorf("deepagent report is empty")
	}

	return AgentRuntimeResponse{
		Content: "Research report is ready. You can download the Markdown report below.",
		Artifacts: []AgentRuntimeArtifact{
			{
				ID:       "report",
				Type:     "markdown_report",
				Path:     reportPath,
				MIMEType: "text/markdown",
				Title:    "DeepAgent Research Report",
				FileName: fmt.Sprintf("deepagent-research-%s.md", runID),
				Content:  report,
			},
		},
		Metadata: map[string]string{
			"runtime": model.AgentRuntimeDeepAgent,
			"run_id":  runID,
		},
	}, nil
}

func (r *DeepAgentRuntime) acquire(ctx context.Context) error {
	select {
	case r.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *DeepAgentRuntime) release() {
	<-r.sem
}

func deepAgentQuestion(agent model.Agent, trigger model.Message) string {
	question := strings.TrimSpace(trigger.Content)
	mention := strings.TrimSpace(agent.Mention)
	if mention != "" && strings.HasPrefix(question, mention) {
		question = strings.TrimSpace(strings.TrimPrefix(question, mention))
	}
	return question
}

func shortCommandOutput(value string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if len(cleaned) > 240 {
		return cleaned[:237] + "..."
	}
	return cleaned
}
