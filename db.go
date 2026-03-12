package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// User represents an authenticated user account.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Translation represents a single translation record.
type Translation struct {
	ID        int64     `json:"id"`
	Chinese   string    `json:"chinese"`
	English   string    `json:"english"`
	CreatedAt time.Time `json:"created_at"`
}

// Correction represents a single English grammar-correction record.
type Correction struct {
	ID        int64     `json:"id"`
	Original  string    `json:"original"`
	Corrected string    `json:"corrected"`
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

	// Create tables and indexes.
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            TEXT PRIMARY KEY,
		email         TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at    DATETIME DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS translations (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		chinese    TEXT NOT NULL,
		english    TEXT NOT NULL,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_translations_user_created
		ON translations(user_id, created_at DESC);

	CREATE TABLE IF NOT EXISTS corrections (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		original   TEXT NOT NULL,
		corrected  TEXT NOT NULL,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_corrections_user_created
		ON corrections(user_id, created_at DESC);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return db, nil
}

// ── User queries ──────────────────────────────────────────────────────────────

func createUser(db *sql.DB, email, passwordHash string) (User, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		id, email, passwordHash, now,
	)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return User{ID: id, Email: email, CreatedAt: now}, nil
}

func getUserByEmail(db *sql.DB, email string) (User, string, error) {
	var u User
	var hash string
	err := db.QueryRow(
		`SELECT id, email, password_hash, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &hash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return User{}, "", nil
	}
	if err != nil {
		return User{}, "", fmt.Errorf("get user by email: %w", err)
	}
	return u, hash, nil
}

// ── Translation queries ───────────────────────────────────────────────────────

// InsertTranslation stores a new translation and returns the complete record.
func InsertTranslation(db *sql.DB, userID, chinese, english string) (Translation, error) {
	result, err := db.Exec(
		"INSERT INTO translations (user_id, chinese, english) VALUES (?, ?, ?)",
		userID, chinese, english,
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

// GetAllTranslations returns every translation for the given user ordered by created_at DESC.
// Used for CSV export where all records are needed.
func GetAllTranslations(db *sql.DB, userID string) ([]Translation, error) {
	rows, err := db.Query(
		"SELECT id, chinese, english, created_at FROM translations WHERE user_id = ? ORDER BY created_at DESC",
		userID,
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

// GetTranslations returns a page of translations for the given user ordered by created_at DESC.
// If search is non-empty, it filters by Chinese or English text.
func GetTranslations(db *sql.DB, userID string, limit, offset int, search string) ([]Translation, int, error) {
	var (
		total int
		rows  *sql.Rows
		err   error
	)

	if search != "" {
		pattern := "%" + search + "%"
		err = db.QueryRow(
			"SELECT COUNT(*) FROM translations WHERE user_id = ? AND (chinese LIKE ? OR english LIKE ?)",
			userID, pattern, pattern,
		).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count translations: %w", err)
		}
		rows, err = db.Query(
			"SELECT id, chinese, english, created_at FROM translations WHERE user_id = ? AND (chinese LIKE ? OR english LIKE ?) ORDER BY created_at DESC LIMIT ? OFFSET ?",
			userID, pattern, pattern, limit, offset,
		)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM translations WHERE user_id = ?", userID).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count translations: %w", err)
		}
		rows, err = db.Query(
			"SELECT id, chinese, english, created_at FROM translations WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
			userID, limit, offset,
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

// DeleteTranslation removes a translation by ID if it belongs to userID.
// Returns true if a row was deleted. The user_id check prevents cross-user deletion.
func DeleteTranslation(db *sql.DB, userID string, id int64) (bool, error) {
	result, err := db.Exec("DELETE FROM translations WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return false, fmt.Errorf("delete translation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return affected > 0, nil
}

// ── Corrections ──────────────────────────────────────────────────────────────

// InsertCorrection stores a new correction record and returns the complete record.
func InsertCorrection(db *sql.DB, userID, original, corrected string) (Correction, error) {
	result, err := db.Exec(
		"INSERT INTO corrections (user_id, original, corrected) VALUES (?, ?, ?)",
		userID, original, corrected,
	)
	if err != nil {
		return Correction{}, fmt.Errorf("insert correction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Correction{}, fmt.Errorf("get last insert id: %w", err)
	}

	var c Correction
	err = db.QueryRow(
		"SELECT id, original, corrected, created_at FROM corrections WHERE id = ?", id,
	).Scan(&c.ID, &c.Original, &c.Corrected, &c.CreatedAt)
	if err != nil {
		return Correction{}, fmt.Errorf("read back correction: %w", err)
	}

	return c, nil
}

// GetCorrections returns a page of corrections for the given user ordered by created_at DESC.
// If search is non-empty, it filters by original or corrected text.
func GetCorrections(db *sql.DB, userID string, limit, offset int, search string) ([]Correction, int, error) {
	var (
		total int
		rows  *sql.Rows
		err   error
	)

	if search != "" {
		pattern := "%" + search + "%"
		err = db.QueryRow(
			"SELECT COUNT(*) FROM corrections WHERE user_id = ? AND (original LIKE ? OR corrected LIKE ?)",
			userID, pattern, pattern,
		).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count corrections: %w", err)
		}
		rows, err = db.Query(
			"SELECT id, original, corrected, created_at FROM corrections WHERE user_id = ? AND (original LIKE ? OR corrected LIKE ?) ORDER BY created_at DESC LIMIT ? OFFSET ?",
			userID, pattern, pattern, limit, offset,
		)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM corrections WHERE user_id = ?", userID).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count corrections: %w", err)
		}
		rows, err = db.Query(
			"SELECT id, original, corrected, created_at FROM corrections WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
			userID, limit, offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("query corrections: %w", err)
	}
	defer rows.Close()

	var corrections []Correction
	for rows.Next() {
		var c Correction
		if err := rows.Scan(&c.ID, &c.Original, &c.Corrected, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan correction: %w", err)
		}
		corrections = append(corrections, c)
	}
	return corrections, total, rows.Err()
}

// GetAllCorrections returns every correction for the given user ordered by created_at DESC.
// Used for CSV export.
func GetAllCorrections(db *sql.DB, userID string) ([]Correction, error) {
	rows, err := db.Query(
		"SELECT id, original, corrected, created_at FROM corrections WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query corrections: %w", err)
	}
	defer rows.Close()

	var corrections []Correction
	for rows.Next() {
		var c Correction
		if err := rows.Scan(&c.ID, &c.Original, &c.Corrected, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan correction: %w", err)
		}
		corrections = append(corrections, c)
	}
	return corrections, rows.Err()
}

// DeleteCorrection removes a correction by ID if it belongs to userID.
// Returns true if a row was deleted. The user_id check prevents cross-user deletion.
func DeleteCorrection(db *sql.DB, userID string, id int64) (bool, error) {
	result, err := db.Exec("DELETE FROM corrections WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return false, fmt.Errorf("delete correction: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return affected > 0, nil
}
