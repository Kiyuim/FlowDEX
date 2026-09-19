package migration

import (
	_ "embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed core_schema.sql
var coreSchema string

// EnsureCoreSchema creates all core tables (CREATE TABLE IF NOT EXISTS) if they
// don't already exist. It is idempotent and safe to run on every startup, which
// lets a service self-provision its schema against a fresh database (e.g. the
// Railway-internal MySQL) over the private network — no manual DB access needed.
func EnsureCoreSchema(db *gorm.DB) error {
	// Drop full-line SQL comments so statement splitting on ';' is clean.
	var buf strings.Builder
	for _, line := range strings.Split(coreSchema, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	for _, stmt := range strings.Split(buf.String(), ";") {
		s := strings.TrimSpace(stmt)
		if s == "" {
			continue
		}
		if err := db.Exec(s).Error; err != nil {
			// Tolerate "table already exists" (MySQL error 1050).
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return fmt.Errorf("EnsureCoreSchema failed on %.70q: %w", s, err)
		}
	}
	return nil
}
