package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// ── Configuration ────────────────────────────────────────────────
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is required")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3-flash-preview"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/translations.db"
	}

	// ── Database ─────────────────────────────────────────────────────
	db, err := InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("Database initialized at %s", dbPath)

	// ── Gemini Client ────────────────────────────────────────────────
	gemini, err := NewGeminiClient(apiKey, model)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}
	log.Printf("Gemini client initialized with model: %s", model)

	// ── HTTP Handler ─────────────────────────────────────────────────
	h := &handler{db: db, gemini: gemini}

	// ── Router ───────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// API routes.
	r.Route("/api", func(r chi.Router) {
		r.Post("/translate", h.handleTranslate)
		r.Get("/history", h.handleHistory)
		r.Get("/export/csv", h.handleExportCSV)
	})

	// Static files (frontend).
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/*", fileServer)

	// ── Server ───────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine.
	go func() {
		log.Printf("TransLens server starting on http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ── Graceful Shutdown ────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	db.Close()
	log.Println("Server stopped gracefully")
}
