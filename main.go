package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	enableReg := true
	if v := os.Getenv("ENABLE_REGISTRATION"); v != "" {
		enableReg = strings.EqualFold(v, "true") || v == "1"
	}

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

	// ── Auth Service ─────────────────────────────────────────────────
	auth := NewAuthService(db, jwtSecret)

	// ── HTTP Handler ─────────────────────────────────────────────────
	h := &handler{
		db:                  db,
		auth:                auth,
		translator:          or,
		corrector:           or,
		registrationEnabled: enableReg,
	}

	log.Printf("Registration enabled: %v", enableReg)

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

	// Public auth routes.
	r.Post("/auth/register", h.handleRegister)
	r.Post("/auth/login", h.handleLogin)
	r.Get("/api/config", h.handleConfig)

	// Login page.
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/login.html")
	})

	// Authenticated API routes.
	r.Group(func(r chi.Router) {
		r.Use(jwtMiddleware(auth))

		// Translation.
		r.Post("/api/translate", h.handleTranslate)
		r.Get("/api/history", h.handleHistory)
		r.Get("/api/export/csv", h.handleExportCSV)
		r.Delete("/api/translations/{id}", h.handleDelete)

		// Correction.
		r.Post("/api/correct", h.handleCorrect)
		r.Get("/api/corrections", h.handleCorrectionHistory)
		r.Get("/api/export/corrections/csv", h.handleExportCorrectionsCSV)
		r.Delete("/api/corrections/{id}", h.handleDeleteCorrection)
	})

	// Static files (frontend) — unauthenticated so login.html can load assets.
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

	log.Println("Server stopped gracefully")
}
