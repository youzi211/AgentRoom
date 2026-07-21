package agent_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/config"
	"agentroom/backend/internal/model"
)

func TestGoToPythonLLMRuntimeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-process Go/Python integration in short mode")
	}
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv is required for the Go/Python integration test")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	deepagentDir := filepath.Join(repositoryRoot, "deepagent")
	command := exec.Command("uv", "run", "python", "tests/integration_llm_server.py", "--port", strconv.Itoa(port))
	command.Dir = deepagentDir
	command.Env = append(os.Environ(),
		"PYTHONDONTWRITEBYTECODE=1",
		"UV_CACHE_DIR="+filepath.Join(deepagentDir, ".uv-cache"),
	)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})

	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "READY" {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatalf("Python integration runtime exited before readiness: %s", stderr.String())
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("Python integration runtime did not become ready: %s", stderr.String())
	}

	runtimeConfig := config.AgentRuntimeConfig{
		Transport:       config.AgentRuntimeTransportGRPC,
		GRPCAddress:     "127.0.0.1:" + strconv.Itoa(port),
		GRPCInsecure:    true,
		LLMTimeout:      5 * time.Second,
		DeepTimeout:     time.Minute,
		MaxRequestBytes: 1024 * 1024,
		MaxEventBytes:   1024 * 1024,
	}
	client, err := agent.NewRemoteRuntimeClient(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	runtime, err := agent.NewRemotePythonRuntime(
		model.AgentRuntimeLLM,
		client,
		5*time.Second,
		remoteModelResolver{resolved: model.ResolvedModelConfig{
			ProfileID: "profile_integration", Source: "database", BaseURL: "https://unused.invalid/v1",
			ModelName: "integration-model", APIKey: "integration-secret",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := remoteRequest("run_python_integration")
	request.Agent.ModelProfileID = "profile_integration"
	response, err := runtime.Respond(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "python integration response" {
		t.Fatalf("unexpected Python response: %#v", response)
	}
	if response.Metadata["model_profile_id"] != "profile_integration" || response.Metadata["model_name"] != "integration-model" {
		t.Fatalf("unexpected model audit: %#v", response.Metadata)
	}
	if len(response.KnowledgeSources) != 1 || response.KnowledgeSources[0].DocumentName != "Plan.md" {
		t.Fatalf("unexpected used knowledge sources: %#v", response.KnowledgeSources)
	}

	deepRuntime, err := agent.NewRemotePythonRuntime(
		model.AgentRuntimeDeepAgent,
		client,
		5*time.Second,
		remoteModelResolver{resolved: model.ResolvedModelConfig{
			ProfileID: "profile_research", Source: "database", BaseURL: "https://unused.invalid/v1",
			ModelName: "research-model", APIKey: "research-secret",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	deepRequest := remoteRequest("run_python_deepagent")
	deepRequest.Agent.Runtime = model.AgentRuntimeDeepAgent
	deepRequest.Agent.Name = "Research"
	deepRequest.Agent.Mention = "@Research"
	deepRequest.Agent.ModelProfileID = "profile_research"
	deepRequest.Trigger.Content = "@Research --config is research text"
	deepResponse, err := deepRuntime.Respond(context.Background(), deepRequest)
	if err != nil {
		t.Fatal(err)
	}
	if len(deepResponse.Artifacts) != 1 || !strings.Contains(deepResponse.Artifacts[0].Content, "--config is research text") {
		t.Fatalf("unexpected DeepAgent artifact: %#v", deepResponse.Artifacts)
	}
	if deepResponse.Metadata["model_profile_id"] != "profile_research" || deepResponse.Metadata["model_name"] != "research-model" {
		t.Fatalf("unexpected DeepAgent model audit: %#v", deepResponse.Metadata)
	}

	failureRequest := remoteRequest("run_python_failure")
	failureRequest.Trigger.Content = "[fail]"
	failureRequest.PromptContext.TriggerContent = "[fail]"
	if _, err := runtime.Respond(context.Background(), failureRequest); !errors.Is(err, agent.ErrRuntimeApplication) {
		t.Fatalf("expected remote application failure without local fallback, got %v", err)
	}

	delayedRuntime, err := agent.NewRemotePythonRuntime(
		model.AgentRuntimeLLM,
		client,
		100*time.Millisecond,
		remoteModelResolver{resolved: model.ResolvedModelConfig{
			ProfileID: "profile_integration", Source: "database", BaseURL: "https://unused.invalid/v1",
			ModelName: "integration-model", APIKey: "integration-secret",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	delayRequest := remoteRequest("run_python_delay")
	delayRequest.Trigger.Content = "[delay]"
	delayRequest.PromptContext.TriggerContent = "[delay]"
	if _, err := delayedRuntime.Respond(context.Background(), delayRequest); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected Go deadline to stop Python execution, got %v", err)
	}
}
