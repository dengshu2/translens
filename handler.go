package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

// Translator abstracts the translation capability so it can be mocked in tests.
type Translator interface {
	Translate(ctx context.Context, chinese string) (string, error)
}

// Corrector abstracts the grammar-correction capability so it can be mocked in tests.
type Corrector interface {
	CorrectEnglish(ctx context.Context, english string) (string, error)
}

// handler holds shared dependencies for HTTP handlers.
type handler struct {
	db                  *sql.DB
	auth                *AuthService
	translator          Translator
	corrector           Corrector
	registrationEnabled bool
}

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// handleTranslate handles POST /api/translate
func (h *handler) handleTranslate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chinese string `json:"chinese"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	// Input validation.
	req.Chinese = strings.TrimSpace(req.Chinese)
	if req.Chinese == "" {
		respondError(w, http.StatusBadRequest, "请输入中文内容")
		return
	}
	if utf8.RuneCountInString(req.Chinese) > 500 {
		respondError(w, http.StatusBadRequest, "输入内容不能超过 500 个字符")
		return
	}

	// Call OpenRouter to translate.
	english, err := h.translator.Translate(r.Context(), req.Chinese)
	if err != nil {
		log.Printf("Translation error: %v", err)
		respondError(w, http.StatusInternalServerError, "翻译失败，请稍后重试")
		return
	}

	// Save to database.
	t, err := InsertTranslation(h.db, req.Chinese, english)
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, http.StatusInternalServerError, "保存翻译记录失败")
		return
	}

	respondJSON(w, http.StatusOK, t)
}

// handleHistory handles GET /api/history?limit=20&offset=0&q=search
func (h *handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	search := strings.TrimSpace(r.URL.Query().Get("q"))

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	translations, total, err := GetTranslations(h.db, limit, offset, search)
	if err != nil {
		log.Printf("Query error: %v", err)
		respondError(w, http.StatusInternalServerError, "获取历史记录失败")
		return
	}

	// Return empty array instead of null.
	if translations == nil {
		translations = []Translation{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"translations": translations,
		"total":        total,
		"has_more":     offset+limit < total,
	})
}

// handleExportCSV handles GET /api/export/csv
func (h *handler) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	translations, err := GetAllTranslations(h.db)
	if err != nil {
		log.Printf("Export error: %v", err)
		respondError(w, http.StatusInternalServerError, "导出失败")
		return
	}

	filename := fmt.Sprintf("translations_%s.csv", time.Now().Format("20060102"))

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// Write UTF-8 BOM for Excel compatibility.
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row.
	writer.Write([]string{"id", "chinese", "english", "created_at"})

	// Data rows.
	for _, t := range translations {
		writer.Write([]string{
			fmt.Sprintf("%d", t.ID),
			t.Chinese,
			t.English,
			t.CreatedAt.Format(time.RFC3339),
		})
	}
}

// handleDelete handles DELETE /api/translations/{id}
func (h *handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	deleted, err := DeleteTranslation(h.db, id)
	if err != nil {
		log.Printf("Delete error: %v", err)
		respondError(w, http.StatusInternalServerError, "删除失败")
		return
	}
	if !deleted {
		respondError(w, http.StatusNotFound, "记录不存在")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleCorrect handles POST /api/correct
func (h *handler) handleCorrect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		English string `json:"english"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	req.English = strings.TrimSpace(req.English)
	if req.English == "" {
		respondError(w, http.StatusBadRequest, "请输入英文内容")
		return
	}
	if utf8.RuneCountInString(req.English) > 1000 {
		respondError(w, http.StatusBadRequest, "输入内容不能超过 1000 个字符")
		return
	}

	corrected, err := h.corrector.CorrectEnglish(r.Context(), req.English)
	if err != nil {
		log.Printf("Correction error: %v", err)
		respondError(w, http.StatusInternalServerError, "纠错失败，请稍后重试")
		return
	}

	c, err := InsertCorrection(h.db, req.English, corrected)
	if err != nil {
		log.Printf("Database error: %v", err)
		respondError(w, http.StatusInternalServerError, "保存纠错记录失败")
		return
	}

	respondJSON(w, http.StatusOK, c)
}

// handleCorrectionHistory handles GET /api/corrections?limit=20&offset=0&q=search
func (h *handler) handleCorrectionHistory(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	search := strings.TrimSpace(r.URL.Query().Get("q"))

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	corrections, total, err := GetCorrections(h.db, limit, offset, search)
	if err != nil {
		log.Printf("Query error: %v", err)
		respondError(w, http.StatusInternalServerError, "获取历史记录失败")
		return
	}

	if corrections == nil {
		corrections = []Correction{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"corrections": corrections,
		"total":       total,
		"has_more":    offset+limit < total,
	})
}

// handleExportCorrectionsCSV handles GET /api/export/corrections/csv
func (h *handler) handleExportCorrectionsCSV(w http.ResponseWriter, r *http.Request) {
	corrections, err := GetAllCorrections(h.db)
	if err != nil {
		log.Printf("Export error: %v", err)
		respondError(w, http.StatusInternalServerError, "导出失败")
		return
	}

	filename := fmt.Sprintf("corrections_%s.csv", time.Now().Format("20060102"))

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"id", "original", "corrected", "created_at"})

	for _, c := range corrections {
		writer.Write([]string{
			fmt.Sprintf("%d", c.ID),
			c.Original,
			c.Corrected,
			c.CreatedAt.Format(time.RFC3339),
		})
	}
}

// handleDeleteCorrection handles DELETE /api/corrections/{id}
func (h *handler) handleDeleteCorrection(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	deleted, err := DeleteCorrection(h.db, id)
	if err != nil {
		log.Printf("Delete error: %v", err)
		respondError(w, http.StatusInternalServerError, "删除失败")
		return
	}
	if !deleted {
		respondError(w, http.StatusNotFound, "记录不存在")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
