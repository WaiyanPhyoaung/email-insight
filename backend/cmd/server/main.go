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
	"github.com/go-chi/cors"
	"github.com/waiyanphyoaung/email-insights/internal/analyzer"
	"github.com/waiyanphyoaung/email-insights/internal/api"
	"github.com/waiyanphyoaung/email-insights/internal/config"
	"github.com/waiyanphyoaung/email-insights/internal/service"
	"github.com/waiyanphyoaung/email-insights/internal/store"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	st, err := store.WaitForDB(ctx, cfg.DatabaseURL, 15)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	if err := st.RunMigrations(ctx, store.MigrationPath()); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	var az analyzer.Analyzer
	if cfg.LLMMockMode {
		log.Println("LLM: mock/heuristic mode (set OPENAI_API_KEY for OpenAI)")
		az = analyzer.NewMock()
	} else {
		log.Printf("LLM: OpenAI model %s", cfg.OpenAIModel)
		az = analyzer.NewOpenAI(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	}

	processor := service.NewProcessor(st, az)
	handler := api.NewHandler(processor, st)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin, "http://localhost:3000", "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", handler.Health)
	r.Route("/api", func(r chi.Router) {
		r.Post("/emails/upload", handler.UploadEmails)
		r.Get("/emails/{id}", handler.GetEmail)
		r.Get("/spending", handler.ListSpending)
		r.Get("/spending/summary", handler.SpendingSummary)
		r.Get("/saas", handler.ListSaaS)
		r.Get("/saas/summary", handler.SaaSSummary)
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
