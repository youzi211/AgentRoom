package mysql

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MySQLStore implements store.Store backed by a MySQL database via GORM.
type MySQLStore struct {
	db *gorm.DB
}

// Open creates a new GORM MySQL connection and verifies it is reachable.
func Open(dsn string) (*MySQLStore, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return &MySQLStore{db: db}, nil
}

func (s *MySQLStore) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (s *MySQLStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate runs AutoMigrate for all models and ensures schema is up to date.
func (s *MySQLStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).Set(
		"gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	).AutoMigrate(
		&AgentModel{},
		&RoomModel{},
		&RoomAgentModel{},
		&ParticipantModel{},
		&MessageModel{},
		&AgentRunModel{},
		&DialogueRunModel{},
		&KnowledgeDocumentModel{},
		&KnowledgeChunkModel{},
		&MeetingMinutesModel{},
		&SchemaMigrationModel{},
	)
}
