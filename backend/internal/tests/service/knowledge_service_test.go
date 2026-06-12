package service_test

import (
	"context"
	"strings"
	"testing"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

func TestKnowledgeServiceUploadsMarkdownAsChunks(t *testing.T) {
	store := &teststore.Store{}
	svc := service.NewKnowledgeService(store)

	document, err := svc.UploadMarkdown(context.Background(), service.UploadKnowledgeInput{
		Scope:    model.KnowledgeScopeRoom,
		ScopeID:  "room_1",
		FileName: "meeting.md",
		Content:  []byte("# Roadmap\n\nWe will launch AgentRoom knowledge in July.\n\n## Risks\n\nLatency must stay low."),
	})
	if err != nil {
		t.Fatalf("UploadMarkdown returned error: %v", err)
	}

	if document.ID == "" {
		t.Fatal("expected document id to be generated")
	}
	if document.Scope != model.KnowledgeScopeRoom || document.ScopeID != "room_1" {
		t.Fatalf("unexpected document scope: %#v", document)
	}
	if len(store.Chunks) == 0 {
		t.Fatal("expected markdown content to be parsed into chunks")
	}

	joined := strings.Join(chunkContents(store.Chunks), "\n")
	if !strings.Contains(joined, "Roadmap") || !strings.Contains(joined, "Latency must stay low.") {
		t.Fatalf("expected parsed markdown text in chunks, got %q", joined)
	}
}

func TestKnowledgeServiceRejectsNonMarkdownFile(t *testing.T) {
	svc := service.NewKnowledgeService(&teststore.Store{})

	_, err := svc.UploadMarkdown(context.Background(), service.UploadKnowledgeInput{
		Scope:    model.KnowledgeScopeAgent,
		ScopeID:  "agent_1",
		FileName: "notes.txt",
		Content:  []byte("plain text"),
	})
	if err == nil {
		t.Fatal("expected non-markdown file to be rejected")
	}
}

func chunkContents(chunks []model.KnowledgeChunk) []string {
	result := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		result = append(result, chunk.Content)
	}
	return result
}
