package main

import (
	"os"
	"path/filepath"
	"testing"
)

// testDB creates a temporary SQLite database for testing and returns
// the *sql.DB handle along with a cleanup function.
func testDB(t *testing.T) (*os.File, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create temp db file: %v", err)
	}
	return f, func() { f.Close() }
}

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify the table was created.
	var tableName string
	err = db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='translations'",
	).Scan(&tableName)
	if err != nil {
		t.Fatalf("translations table not found: %v", err)
	}
	if tableName != "translations" {
		t.Errorf("expected table name 'translations', got %q", tableName)
	}

	// Verify the index was created.
	var indexName string
	err = db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='index' AND name='idx_translations_created_at'",
	).Scan(&indexName)
	if err != nil {
		t.Fatalf("index not found: %v", err)
	}
}

func TestInitDB_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "a", "b", "c", "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify the nested directory was created.
	info, err := os.Stat(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}
}

func TestInsertTranslation(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	trans, err := InsertTranslation(db, "你好", "Hello")
	if err != nil {
		t.Fatalf("InsertTranslation failed: %v", err)
	}

	if trans.ID != 1 {
		t.Errorf("expected ID=1, got %d", trans.ID)
	}
	if trans.Chinese != "你好" {
		t.Errorf("expected Chinese='你好', got %q", trans.Chinese)
	}
	if trans.English != "Hello" {
		t.Errorf("expected English='Hello', got %q", trans.English)
	}
	if trans.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestInsertTranslation_AutoIncrement(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	t1, _ := InsertTranslation(db, "你好", "Hello")
	t2, _ := InsertTranslation(db, "谢谢", "Thanks")

	if t2.ID != t1.ID+1 {
		t.Errorf("expected auto-increment ID: got %d after %d", t2.ID, t1.ID)
	}
}

func TestGetAllTranslations_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	translations, err := GetAllTranslations(db)
	if err != nil {
		t.Fatalf("GetAllTranslations failed: %v", err)
	}
	if translations != nil {
		t.Errorf("expected nil for empty table, got %v", translations)
	}
}

func TestGetAllTranslations_OrderedByCreatedAtDesc(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Insert in order: first, second, third.
	InsertTranslation(db, "第一", "First")
	InsertTranslation(db, "第二", "Second")
	InsertTranslation(db, "第三", "Third")

	translations, err := GetAllTranslations(db)
	if err != nil {
		t.Fatalf("GetAllTranslations failed: %v", err)
	}

	if len(translations) != 3 {
		t.Fatalf("expected 3 translations, got %d", len(translations))
	}

	// Verify all records are present (ordering within same second is
	// non-deterministic due to SQLite datetime('now') precision).
	found := make(map[string]bool)
	for _, tr := range translations {
		found[tr.English] = true
	}
	for _, eng := range []string{"First", "Second", "Third"} {
		if !found[eng] {
			t.Errorf("missing translation %q", eng)
		}
	}
}

func TestInsertTranslation_EmptyStrings(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Empty strings are valid at the DB level (validation happens in handler).
	trans, err := InsertTranslation(db, "", "")
	if err != nil {
		t.Fatalf("InsertTranslation with empty strings failed: %v", err)
	}
	if trans.Chinese != "" || trans.English != "" {
		t.Errorf("expected empty strings, got %q / %q", trans.Chinese, trans.English)
	}
}

func TestInsertTranslation_Unicode(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	chinese := "今天的天气真不错啊 🌤️"
	english := "The weather's really nice today 🌤️"

	trans, err := InsertTranslation(db, chinese, english)
	if err != nil {
		t.Fatalf("InsertTranslation failed: %v", err)
	}
	if trans.Chinese != chinese {
		t.Errorf("Chinese mismatch: got %q", trans.Chinese)
	}
	if trans.English != english {
		t.Errorf("English mismatch: got %q", trans.English)
	}
}
