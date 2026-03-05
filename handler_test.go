package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// mockTranslator is a test double for the Translator interface.
type mockTranslator struct {
	result string
	err    error
}

func (m *mockTranslator) Translate(_ context.Context, _ string) (string, error) {
	return m.result, m.err
}

// newTestHandler creates a handler with a temp DB and mock translator.
func newTestHandler(t *testing.T, translator Translator) *handler {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return &handler{db: db, translator: translator}
}

// ── POST /api/translate ─────────────────────────────────────

func TestHandleTranslate_Success(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{result: "Hello"})

	body := strings.NewReader(`{"chinese":"你好"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleTranslate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp Translation
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Chinese != "你好" {
		t.Errorf("expected Chinese='你好', got %q", resp.Chinese)
	}
	if resp.English != "Hello" {
		t.Errorf("expected English='Hello', got %q", resp.English)
	}
	if resp.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestHandleTranslate_EmptyInput(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{result: "Hello"})

	body := strings.NewReader(`{"chinese":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleTranslate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleTranslate_WhitespaceOnly(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{result: "Hello"})

	body := strings.NewReader(`{"chinese":"   "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleTranslate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for whitespace-only input, got %d", rec.Code)
	}
}

func TestHandleTranslate_TooLong(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{result: "Hello"})

	// 501 runes of Chinese characters.
	long := strings.Repeat("你", 501)
	body := strings.NewReader(`{"chinese":"` + long + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleTranslate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for too-long input, got %d", rec.Code)
	}
}

func TestHandleTranslate_InvalidJSON(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{result: "Hello"})

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleTranslate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestHandleTranslate_TranslatorError(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{
		err: io.ErrUnexpectedEOF, // simulate API failure
	})

	body := strings.NewReader(`{"chinese":"你好"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.handleTranslate(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	// Verify the error message does NOT leak internal details.
	var errResp map[string]string
	json.NewDecoder(rec.Body).Decode(&errResp)
	if strings.Contains(errResp["error"], "EOF") {
		t.Error("error response should not leak internal error details")
	}
}

// ── GET /api/history ────────────────────────────────────────

func TestHandleHistory_Empty(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{})

	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	rec := httptest.NewRecorder()

	h.handleHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Translations []Translation `json:"translations"`
		Total        int           `json:"total"`
		HasMore      bool          `json:"has_more"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Should return empty array, not null.
	if resp.Translations == nil {
		t.Error("expected empty array, got null")
	}
	if len(resp.Translations) != 0 {
		t.Errorf("expected 0 translations, got %d", len(resp.Translations))
	}
	if resp.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Total)
	}
	if resp.HasMore {
		t.Error("expected has_more=false for empty result")
	}
}

func TestHandleHistory_WithData(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{result: "Hello"})

	// Insert some data via translate endpoint.
	for _, chinese := range []string{"你好", "谢谢", "再见"} {
		body := strings.NewReader(`{"chinese":"` + chinese + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.handleTranslate(rec, req)
	}

	// Fetch history.
	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	rec := httptest.NewRecorder()
	h.handleHistory(rec, req)

	var resp struct {
		Translations []Translation `json:"translations"`
		Total        int           `json:"total"`
		HasMore      bool          `json:"has_more"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Translations) != 3 {
		t.Fatalf("expected 3 translations, got %d", len(resp.Translations))
	}
	if resp.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Total)
	}

	// Verify all inserted records are present.
	found := make(map[string]bool)
	for _, tr := range resp.Translations {
		found[tr.Chinese] = true
	}
	for _, ch := range []string{"你好", "谢谢", "再见"} {
		if !found[ch] {
			t.Errorf("missing translation for %q", ch)
		}
	}
}

// ── GET /api/export/csv ─────────────────────────────────────

func TestHandleExportCSV_Empty(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{})

	req := httptest.NewRequest(http.MethodGet, "/api/export/csv", nil)
	rec := httptest.NewRecorder()

	h.handleExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Errorf("expected Content-Type text/csv, got %q", ct)
	}

	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "attachment") {
		t.Errorf("expected attachment disposition, got %q", disp)
	}

	// Should have BOM + header row.
	body := rec.Body.Bytes()
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Error("expected UTF-8 BOM at start of CSV")
	}
}

func TestHandleExportCSV_WithData(t *testing.T) {
	h := newTestHandler(t, &mockTranslator{result: "Hello"})

	// Insert data.
	body := strings.NewReader(`{"chinese":"你好"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/translate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.handleTranslate(rec, req)

	// Export.
	req = httptest.NewRequest(http.MethodGet, "/api/export/csv", nil)
	rec = httptest.NewRecorder()
	h.handleExportCSV(rec, req)

	csvBody := rec.Body.String()
	// Should contain header + 1 data row.
	lines := strings.Split(strings.TrimSpace(csvBody), "\n")
	// Account for BOM in first line.
	if len(lines) != 2 {
		t.Errorf("expected 2 CSV lines (header + 1 row), got %d", len(lines))
	}
}
