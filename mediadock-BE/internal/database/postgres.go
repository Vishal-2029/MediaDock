package database

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/mediadock/backend/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectPostgres opens a GORM PostgreSQL connection with production-oriented pool settings.
func ConnectPostgres(databaseURL string, isProduction bool) (*gorm.DB, error) {
	gormLogLevel := logger.Warn
	if !isProduction {
		gormLogLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

// Ping verifies database connectivity within the given context.
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// RunMigrations executes embedded *.up.sql migrations in lexical order inside transactions,
// recording applied versions in schema_migrations.
func RunMigrations(ctx context.Context, db *gorm.DB, log *slog.Logger) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}

	const createTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version VARCHAR(255) PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	if err := db.WithContext(ctx).Exec(createTableSQL).Error; err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		var count int64
		if err := db.WithContext(ctx).
			Raw("SELECT COUNT(1) FROM schema_migrations WHERE version = ?", name).
			Scan(&count).Error; err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if count > 0 {
			continue
		}

		rawSQL, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(rawSQL)).Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name).Error
		})
		if err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if log != nil {
			log.Info("applied database migration", "version", name)
		}
	}

	return nil
}

