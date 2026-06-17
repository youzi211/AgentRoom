package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

func (s *MySQLStore) CreateKnowledgeDocument(ctx context.Context, document model.KnowledgeDocument, chunks []model.KnowledgeChunk) (model.KnowledgeDocument, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(knowledgeDocumentToModel(document)).Error; err != nil {
			return fmt.Errorf("insert knowledge document: %w", err)
		}
		for _, chunk := range chunks {
			if err := tx.Create(knowledgeChunkToModel(chunk)).Error; err != nil {
				return fmt.Errorf("insert knowledge chunk: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return model.KnowledgeDocument{}, err
	}
	return document, nil
}

func (s *MySQLStore) ListKnowledgeDocuments(ctx context.Context, query store.ListKnowledgeDocumentsQuery) ([]model.KnowledgeDocument, error) {
	var models []KnowledgeDocumentModel
	if err := s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ?", query.Scope, query.ScopeID).
		Order("created_at DESC, id DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list knowledge documents: %w", err)
	}

	documents := make([]model.KnowledgeDocument, len(models))
	for i, m := range models {
		documents[i] = m.toDomain()
	}
	return documents, nil
}

func (s *MySQLStore) DeleteKnowledgeDocument(ctx context.Context, documentID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", documentID).Delete(&KnowledgeChunkModel{}).Error; err != nil {
			return fmt.Errorf("delete knowledge chunks: %w", err)
		}

		result := tx.Where("id = ?", documentID).Delete(&KnowledgeDocumentModel{})
		if result.Error != nil {
			return fmt.Errorf("delete knowledge document: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: %s", store.ErrKnowledgeDocumentNotFound, documentID)
		}
		return nil
	})
}

func (s *MySQLStore) SearchKnowledgeChunks(ctx context.Context, query store.SearchKnowledgeChunksQuery) ([]model.KnowledgeChunk, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 6
	}
	if limit > 20 {
		limit = 20
	}

	type knowledgeChunkSearchRow struct {
		KnowledgeChunkModel
		DocumentName string `gorm:"column:document_name"`
	}

	var rows []knowledgeChunkSearchRow
	if err := s.db.WithContext(ctx).
		Table("knowledge_chunks AS c").
		Select("c.*, d.file_name AS document_name").
		Joins("JOIN knowledge_documents AS d ON d.id = c.document_id").
		Where("c.scope = ? AND c.scope_id = ?", query.Scope, query.ScopeID).
		Order("c.created_at DESC, c.chunk_index ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("search knowledge chunks: %w", err)
	}

	chunks := make([]model.KnowledgeChunk, len(rows))
	for i, row := range rows {
		chunk := row.KnowledgeChunkModel.toDomain()
		chunk.DocumentName = row.DocumentName
		chunks[i] = chunk
	}
	return chunks, nil
}
