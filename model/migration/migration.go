package migration

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
)

// MigrationFunc 定义迁移函数类型
type MigrationFunc func(*sql.DB) error

// migrations 存储所有注册的迁移
var migrations = make(map[string]MigrationFunc)

// registerMigration 注册一个迁移函数
func registerMigration(name string, fn MigrationFunc) {
	if _, exists := migrations[name]; exists {
		log.Printf("Warning: Migration %s already registered, overwriting", name)
	}
	migrations[name] = fn
}

// RunMigrations 按顺序执行所有迁移
func RunMigrations(db *sql.DB) error {
	// 确保迁移表存在
	if err := ensureMigrationTableExists(db); err != nil {
		return fmt.Errorf("failed to ensure migration table exists: %w", err)
	}

	// 获取已执行的迁移
	executedMigrations, err := getExecutedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get executed migrations: %w", err)
	}

	// 按名称排序迁移
	var migrationNames []string
	for name := range migrations {
		migrationNames = append(migrationNames, name)
	}
	sort.Strings(migrationNames)

	// 执行未执行的迁移
	for _, name := range migrationNames {
		if !executedMigrations[name] {
			log.Printf("Running migration: %s", name)
			if err := migrations[name](db); err != nil {
				return fmt.Errorf("failed to run migration %s: %w", name, err)
			}

			// 记录迁移已执行
			if err := recordMigration(db, name); err != nil {
				return fmt.Errorf("failed to record migration %s: %w", name, err)
			}
			log.Printf("Migration %s completed successfully", name)
		} else {
			log.Printf("Skipping already executed migration: %s", name)
		}
	}

	return nil
}

// ensureMigrationTableExists 确保迁移表存在
func ensureMigrationTableExists(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS migrations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`)
	return err
}

// getExecutedMigrations 获取已执行的迁移
func getExecutedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT name FROM migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result[name] = true
	}

	return result, rows.Err()
}

// recordMigration 记录迁移已执行
func recordMigration(db *sql.DB, name string) error {
	_, err := db.Exec("INSERT INTO migrations (name) VALUES (?)", name)
	return err
}
