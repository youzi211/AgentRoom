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

func TestKnowledgeServiceSearchReturnsChunkSources(t *testing.T) {
	store := &teststore.Store{}
	svc := service.NewKnowledgeService(store)
	ctx := context.Background()

	roomDocument, err := svc.UploadMarkdown(ctx, service.UploadKnowledgeInput{
		Scope:    model.KnowledgeScopeRoom,
		ScopeID:  "room_1",
		FileName: "roadmap.md",
		Content:  []byte("# Roadmap\n\nAgentRoom will add visible citations for knowledge-backed replies."),
	})
	if err != nil {
		t.Fatalf("UploadMarkdown room document returned error: %v", err)
	}
	agentDocument, err := svc.UploadMarkdown(ctx, service.UploadKnowledgeInput{
		Scope:    model.KnowledgeScopeAgent,
		ScopeID:  "agent_1",
		FileName: "qa-playbook.md",
		Content:  []byte("# QA Playbook\n\nAlways call out verification evidence and residual risk."),
	})
	if err != nil {
		t.Fatalf("UploadMarkdown agent document returned error: %v", err)
	}

	chunks, err := svc.SearchForAgent(ctx, "room_1", "agent_1", "verification citations")
	if err != nil {
		t.Fatalf("SearchForAgent returned error: %v", err)
	}

	if !hasChunkSource(chunks, roomDocument.ID, "roadmap.md") {
		t.Fatalf("expected room chunk source %s/roadmap.md in %#v", roomDocument.ID, chunks)
	}
	if !hasChunkSource(chunks, agentDocument.ID, "qa-playbook.md") {
		t.Fatalf("expected agent chunk source %s/qa-playbook.md in %#v", agentDocument.ID, chunks)
	}
}

func chunkContents(chunks []model.KnowledgeChunk) []string {
	result := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		result = append(result, chunk.Content)
	}
	return result
}

func hasChunkSource(chunks []model.KnowledgeChunk, documentID string, documentName string) bool {
	for _, chunk := range chunks {
		if chunk.DocumentID == documentID && chunk.DocumentName == documentName {
			return true
		}
	}
	return false
}
