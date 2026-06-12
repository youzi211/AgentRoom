package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

const (
	markdownContentType = "text/markdown; charset=utf-8"
	maxMarkdownBytes    = 1 << 20
	chunkSizeRunes      = 1200
	maxKnowledgeChunks  = 6
)

var (
	ErrKnowledgeInvalidScope = errors.New("invalid knowledge scope")
	ErrKnowledgeInvalidFile  = errors.New("invalid knowledge file")
	ErrKnowledgeTooLarge     = errors.New("knowledge file is too large")
)

type KnowledgeService struct {
	store store.Store
	md    goldmark.Markdown
}

type UploadKnowledgeInput struct {
	Scope    string
	ScopeID  string
	FileName string
	Content  []byte
}

func NewKnowledgeService(s store.Store) *KnowledgeService {
	return &KnowledgeService{
		store: s,
		md:    goldmark.New(),
	}
}

func (s *KnowledgeService) UploadMarkdown(ctx context.Context, input UploadKnowledgeInput) (model.KnowledgeDocument, error) {
	scope := strings.TrimSpace(input.Scope)
	scopeID := strings.TrimSpace(input.ScopeID)
	fileName := strings.TrimSpace(filepath.Base(input.FileName))

	if !validKnowledgeScope(scope) || scopeID == "" {
		return model.KnowledgeDocument{}, ErrKnowledgeInvalidScope
	}
	if !isMarkdownFile(fileName) || len(input.Content) == 0 {
		return model.KnowledgeDocument{}, ErrKnowledgeInvalidFile
	}
	if len(input.Content) > maxMarkdownBytes {
		return model.KnowledgeDocument{}, ErrKnowledgeTooLarge
	}

	textContent, err := s.extractMarkdownText(input.Content)
	if err != nil {
		return model.KnowledgeDocument{}, err
	}
	if strings.TrimSpace(textContent) == "" {
		return model.KnowledgeDocument{}, ErrKnowledgeInvalidFile
	}

	now := time.Now().UTC()
	documentID := model.NewID("doc")
	document := model.KnowledgeDocument{
		ID:          documentID,
		Scope:       scope,
		ScopeID:     scopeID,
		FileName:    fileName,
		ContentType: markdownContentType,
		SizeBytes:   int64(len(input.Content)),
		Status:      model.KnowledgeStatusReady,
		CreatedAt:   now,
	}

	parts := splitKnowledgeText(textContent, chunkSizeRunes)
	chunks := make([]model.KnowledgeChunk, 0, len(parts))
	for i, part := range parts {
		chunks = append(chunks, model.KnowledgeChunk{
			ID:         model.NewID("chunk"),
			DocumentID: documentID,
			Scope:      scope,
			ScopeID:    scopeID,
			ChunkIndex: i,
			Content:    part,
			CreatedAt:  now,
		})
	}

	return s.store.CreateKnowledgeDocument(ctx, document, chunks)
}

func (s *KnowledgeService) ListDocuments(ctx context.Context, scope string, scopeID string) ([]model.KnowledgeDocument, error) {
	if !validKnowledgeScope(scope) || strings.TrimSpace(scopeID) == "" {
		return nil, ErrKnowledgeInvalidScope
	}
	return s.store.ListKnowledgeDocuments(ctx, store.ListKnowledgeDocumentsQuery{
		Scope:   scope,
		ScopeID: strings.TrimSpace(scopeID),
	})
}

func (s *KnowledgeService) DeleteDocument(ctx context.Context, documentID string) error {
	trimmed := strings.TrimSpace(documentID)
	if trimmed == "" {
		return ErrKnowledgeInvalidFile
	}
	return s.store.DeleteKnowledgeDocument(ctx, trimmed)
}

func (s *KnowledgeService) SearchForAgent(ctx context.Context, roomID string, agentID string, query string) ([]model.KnowledgeChunk, error) {
	roomChunks, err := s.store.SearchKnowledgeChunks(ctx, store.SearchKnowledgeChunksQuery{
		Scope:   model.KnowledgeScopeRoom,
		ScopeID: strings.TrimSpace(roomID),
		Query:   query,
		Limit:   maxKnowledgeChunks * 3,
	})
	if err != nil {
		return nil, err
	}
	roomChunks = RankKnowledgeChunks(roomChunks, query, maxKnowledgeChunks)

	agentChunks, err := s.store.SearchKnowledgeChunks(ctx, store.SearchKnowledgeChunksQuery{
		Scope:   model.KnowledgeScopeAgent,
		ScopeID: strings.TrimSpace(agentID),
		Query:   query,
		Limit:   maxKnowledgeChunks * 3,
	})
	if err != nil {
		return nil, err
	}
	agentChunks = RankKnowledgeChunks(agentChunks, query, maxKnowledgeChunks)

	return append(roomChunks, agentChunks...), nil
}

func (s *KnowledgeService) extractMarkdownText(source []byte) (string, error) {
	reader := text.NewReader(source)
	root := s.md.Parser().Parse(reader)

	var builder strings.Builder
	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			switch node.Kind() {
			case ast.KindParagraph, ast.KindHeading, ast.KindListItem, ast.KindCodeBlock, ast.KindFencedCodeBlock:
				builder.WriteString("\n")
			}
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Text:
			builder.Write(n.Value(source))
			if n.SoftLineBreak() || n.HardLineBreak() {
				builder.WriteString("\n")
			} else {
				builder.WriteString(" ")
			}
		case *ast.CodeSpan:
			builder.Write(n.Text(source))
			builder.WriteString(" ")
		case *ast.String:
			builder.Write(n.Value)
			builder.WriteString(" ")
		case *ast.FencedCodeBlock:
			builder.Write(n.Lines().Value(source))
			builder.WriteString("\n")
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			builder.Write(n.Lines().Value(source))
			builder.WriteString("\n")
			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse markdown: %w", err)
	}

	return normalizeKnowledgeText(builder.String()), nil
}

func validKnowledgeScope(scope string) bool {
	return scope == model.KnowledgeScopeRoom || scope == model.KnowledgeScopeAgent
}

func isMarkdownFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext == ".md" || ext == ".markdown"
}

func normalizeKnowledgeText(value string) string {
	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(collapseSpaces(line))
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, "\n")
}

func collapseSpaces(value string) string {
	var builder strings.Builder
	lastWasSpace := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				builder.WriteRune(' ')
				lastWasSpace = true
			}
			continue
		}
		builder.WriteRune(r)
		lastWasSpace = false
	}
	return builder.String()
}

func splitKnowledgeText(value string, maxRunes int) []string {
	paragraphs := strings.Split(value, "\n")
	chunks := make([]string, 0)
	var current bytes.Buffer
	currentRunes := 0

	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			chunks = append(chunks, text)
		}
		current.Reset()
		currentRunes = 0
	}

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		paragraphRunes := len([]rune(paragraph))
		if currentRunes > 0 && currentRunes+paragraphRunes+1 > maxRunes {
			flush()
		}
		if paragraphRunes > maxRunes {
			for _, part := range splitLongRunes(paragraph, maxRunes) {
				if currentRunes > 0 {
					flush()
				}
				chunks = append(chunks, part)
			}
			continue
		}

		if currentRunes > 0 {
			current.WriteString("\n")
			currentRunes++
		}
		current.WriteString(paragraph)
		currentRunes += paragraphRunes
	}
	flush()
	return chunks
}

func splitLongRunes(value string, maxRunes int) []string {
	runes := []rune(value)
	result := make([]string, 0, len(runes)/maxRunes+1)
	for len(runes) > 0 {
		end := maxRunes
		if len(runes) < end {
			end = len(runes)
		}
		result = append(result, strings.TrimSpace(string(runes[:end])))
		runes = runes[end:]
	}
	return result
}

func RankKnowledgeChunks(chunks []model.KnowledgeChunk, query string, limit int) []model.KnowledgeChunk {
	if limit <= 0 || len(chunks) <= limit {
		return chunks
	}
	keywords := queryKeywords(query)
	sort.SliceStable(chunks, func(i, j int) bool {
		return knowledgeScore(chunks[i].Content, keywords) > knowledgeScore(chunks[j].Content, keywords)
	})
	return chunks[:limit]
}

func queryKeywords(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("，。！？,.!?;；:：()（）[]【】\"'", r)
	})
	keywords := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) >= 2 {
			keywords = append(keywords, field)
		}
	}
	return keywords
}

func knowledgeScore(content string, keywords []string) int {
	lower := strings.ToLower(content)
	score := 0
	for _, keyword := range keywords {
		score += strings.Count(lower, keyword)
	}
	return score
}
