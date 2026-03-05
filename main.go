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
	"github.com/joho/godotenv"
)

func main() {
	// ── Load .env (optional, ignored in Docker/production) ──────────
	_ = godotenv.Load()

	// ── Configuration ────────────────────────────────────────────────
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is required")
	}

	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "google/gemini-2.5-flash-preview"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/translations.db"
	}

	authUser := os.Getenv("AUTH_USERNAME")
	authPass := os.Getenv("AUTH_PASSWORD")

	// ── Database ─────────────────────────────────────────────────────
	db, err := InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("Database initialized at %s", dbPath)

	// ── OpenRouter Client ───────────────────────────────────────────
	or, err := NewOpenRouterClient(apiKey, model)
	if err != nil {
		log.Fatalf("Failed to initialize OpenRouter client: %v", err)
	}
	log.Printf("OpenRouter client initialized with model: %s", model)

	// ── HTTP Handler ─────────────────────────────────────────────────
	h := &handler{db: db, translator: or}

	// ── Router ───────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.Throttle(10)) // Limit to 10 concurrent requests.
	r.Use(corsMiddleware)

	// Health check (unauthenticated, for Docker/load balancer probes).
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("db: unhealthy"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		if authUser != "" && authPass != "" {
			r.Use(basicAuth(authUser, authPass))
			log.Println("Basic Auth enabled")
		} else {
			log.Println("WARNING: Basic Auth disabled (AUTH_USERNAME/AUTH_PASSWORD not set)")
		}

		// API routes.
		r.Route("/api", func(r chi.Router) {
			r.Post("/translate", h.handleTranslate)
			r.Get("/history", h.handleHistory)
			r.Get("/export/csv", h.handleExportCSV)
			r.Delete("/translations/{id}", h.handleDelete)
		})

		// Static files (frontend).
		fileServer := http.FileServer(http.Dir("static"))
		r.Handle("/*", fileServer)
	})

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

	log.Println("Server stopped gracefully")
}


