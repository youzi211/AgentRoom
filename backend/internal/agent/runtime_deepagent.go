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
	Resolver    ModelConfigResolver
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

func (r *DeepAgentRuntime) Respond(ctx context.Context, request AgentRuntimeRequest, observers ...AgentEventObserver) (_ AgentRuntimeResponse, err error) {
	observeRuntimeEvent(ctx, observers, AgentRuntimeEvent{RunID: request.RunID, Kind: "accepted"})
	defer func() {
		kind := "completed"
		failure := ""
		if err != nil {
			kind = "failed"
			failure = "local runtime failed"
		}
		observeRuntimeEvent(ctx, observers, AgentRuntimeEvent{RunID: request.RunID, Kind: kind, Failure: failure})
	}()
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
	resolved := model.ResolvedModelConfig{}
	if r.config.Resolver != nil {
		var err error
		resolved, err = r.config.Resolver.Resolve(requestCtx, model.ModelRuntimeDeepAgent, request.Agent.ModelProfileID)
		if err != nil {
			return AgentRuntimeResponse{}, err
		}
	}
	metadata := modelAuditMetadata(resolved)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["runtime"] = model.AgentRuntimeDeepAgent
	metadata["run_id"] = runID
	cmd.Env = append(os.Environ(), r.config.Env...)
	if resolved.Source != "" {
		cmd.Env = append(cmd.Env, "MODEL_PROTOCOL=openai", "MODEL_BASE_URL="+resolved.BaseURL, "MODEL_NAME="+resolved.ModelName, "MODEL_API_KEY="+resolved.APIKey)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := r.acquire(requestCtx); err != nil {
		return AgentRuntimeResponse{Metadata: metadata}, err
	}
	defer r.release()

	if err := cmd.Run(); err != nil {
		safeOutput := shortCommandOutput(redactSecret(output.String(), resolved.APIKey))
		return AgentRuntimeResponse{Metadata: metadata}, fmt.Errorf("deepagent command failed: %w: %s", err, safeOutput)
	}

	reportPath := filepath.Join(r.config.WorkDir, "runs", runID, "report.md")
	reportBytes, err := os.ReadFile(reportPath)
	if err != nil {
		return AgentRuntimeResponse{Metadata: metadata}, fmt.Errorf("read deepagent report: %w", err)
	}
	report := strings.TrimSpace(redactSecret(string(reportBytes), resolved.APIKey))
	if report == "" {
		return AgentRuntimeResponse{Metadata: metadata}, fmt.Errorf("deepagent report is empty")
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
		Metadata: metadata,
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
