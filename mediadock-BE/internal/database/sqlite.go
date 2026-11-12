package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/mediadock/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectDevSQLite opens a local SQLite database for non-production development
// when PostgreSQL is unavailable, and ensures all schema tables exist.
func ConnectDevSQLite(dbPath string) (*gorm.DB, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	if err := db.AutoMigrate(
		&repository.User{},
		&repository.UserSettings{},
		&repository.MediaSource{},
		&repository.MediaItem{},
		&repository.DownloadJob{},
		&repository.DownloadHistory{},
		&repository.ConversionJob{},
	); err != nil {
		return nil, fmt.Errorf("automigrate sqlite: %w", err)
	}

	return db, nil
}

