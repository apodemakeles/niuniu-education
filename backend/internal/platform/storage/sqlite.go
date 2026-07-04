package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"

	// 纯 Go SQLite 驱动，CGO_ENABLED=0 可交叉编译。
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open 打开/创建 SQLite 数据库，执行迁移并确保默认词库存在。
// DSN 指向 dataDir/niuniu.db。
func Open(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	dataDir, err := cfg.AbsDataDir()
	if err != nil {
		return nil, err
	}
	dsn := filepath.Join(dataDir, "niuniu.db")

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dsn, err)
	}

	// 本地单用户：单连接即可避免写锁冲突。
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA journal_mode=%s", cfg.Database.JournalMode)); err != nil {
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", cfg.Database.BusyTimeout)); err != nil {
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign_keys: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		return nil, err
	}
	return db, nil
}

// migrate 读取嵌入式 SQL 迁移脚本并按版本号顺序执行。
// 采用 schema_migrations 表记录已应用版本，保证幂等。
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name() // 如 001_init.sql
		version := name
		var exists string
		err := db.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version=?", version).Scan(&exists)
		if err == nil {
			continue // 已应用
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", version, err)
		}

		script, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(script)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		// 脚本内已 INSERT 对应版本记录，这里容错补一次。
		db.ExecContext(ctx, "INSERT OR IGNORE INTO schema_migrations(version) VALUES (?)", version)
	}
	return nil
}
