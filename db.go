package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Translation represents a single translation record.
type Translation struct {
	ID        int64     `json:"id"`
	Chinese   string    `json:"chinese"`
	English   string    `json:"english"`
	CreatedAt time.Time `json:"created_at"`
}

// InitDB opens (or creates) the SQLite database at dbPath,
// creates the translations table if it doesn't exist, and returns the *sql.DB handle.
func InitDB(dbPath string) (*sql.DB, error) {
	// Ensure the parent directory exists.
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite pragmas for better performance.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	// Create table and index.
	schema := `
	CREATE TABLE IF NOT EXISTS translations (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		chinese    TEXT NOT NULL,
		english    TEXT NOT NULL,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_translations_created_at
		ON translations(created_at DESC);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return db, nil
}

// InsertTranslation stores a new translation and returns the complete record.
func InsertTranslation(db *sql.DB, chinese, english string) (Translation, error) {
	result, err := db.Exec(
		"INSERT INTO translations (chinese, english) VALUES (?, ?)",
		chinese, english,
	)
	if err != nil {
		return Translation{}, fmt.Errorf("insert translation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Translation{}, fmt.Errorf("get last insert id: %w", err)
	}

	var t Translation
	err = db.QueryRow(
		"SELECT id, chinese, english, created_at FROM translations WHERE id = ?", id,
	).Scan(&t.ID, &t.Chinese, &t.English, &t.CreatedAt)
	if err != nil {
		return Translation{}, fmt.Errorf("read back translation: %w", err)
	}

	return t, nil
}

// GetAllTranslations returns every translation ordered by created_at DESC.
// Used for CSV export where all records are needed.
func GetAllTranslations(db *sql.DB) ([]Translation, error) {
	rows, err := db.Query(
		"SELECT id, chinese, english, created_at FROM translations ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("query translations: %w", err)
	}
	defer rows.Close()

	var translations []Translation
	for rows.Next() {
		var t Translation
		if err := rows.Scan(&t.ID, &t.Chinese, &t.English, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan translation: %w", err)
		}
		translations = append(translations, t)
	}
	return translations, rows.Err()
}

// GetTranslations returns a page of translations ordered by created_at DESC.
// If search is non-empty, it filters by Chinese or English text.
func GetTranslations(db *sql.DB, limit, offset int, search string) ([]Translation, int, error) {
	var (
		total int
		rows  *sql.Rows
		err   error
	)

	if search != "" {
		pattern := "%" + search + "%"
		err = db.QueryRow(
			"SELECT COUNT(*) FROM translations WHERE chinese LIKE ? OR english LIKE ?",
			pattern, pattern,
		).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count translations: %w", err)
		}
		rows, err = db.Query(
			"SELECT id, chinese, english, created_at FROM translations WHERE chinese LIKE ? OR english LIKE ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
			pattern, pattern, limit, offset,
		)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM translations").Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count translations: %w", err)
		}
		rows, err = db.Query(
			"SELECT id, chinese, english, created_at FROM translations ORDER BY created_at DESC LIMIT ? OFFSET ?",
			limit, offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("query translations: %w", err)
	}
	defer rows.Close()

	var translations []Translation
	for rows.Next() {
		var t Translation
		if err := rows.Scan(&t.ID, &t.Chinese, &t.English, &t.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan translation: %w", err)
		}
		translations = append(translations, t)
	}
	return translations, total, rows.Err()
}

// DeleteTranslation removes a translation by ID. Returns true if a row was deleted.
func DeleteTranslation(db *sql.DB, id int64) (bool, error) {
	result, err := db.Exec("DELETE FROM translations WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("delete translation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return affected > 0, nil
}
