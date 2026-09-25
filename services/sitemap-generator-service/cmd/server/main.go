package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"metarang/sitemap-generator-service/internal/config"
	"metarang/sitemap-generator-service/internal/repository"
	"metarang/sitemap-generator-service/internal/scheduler"
	"metarang/sitemap-generator-service/internal/sitemap"
)

type runState struct {
	mu          sync.RWMutex
	lastSuccess time.Time
	lastError   string
}

func (s *runState) success() {
	s.mu.Lock()
	s.lastSuccess = time.Now()
	s.lastError = ""
	s.mu.Unlock()
}

func (s *runState) failure(err error) {
	s.mu.Lock()
	s.lastError = err.Error()
	s.mu.Unlock()
}

func (s *runState) ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.lastSuccess.IsZero() && s.lastError == ""
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	state := &runState{}
	httpServer := startHealthServer(cfg.HTTPPort, state)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("health server shutdown: %v", err)
		}
	}()

	log.Printf("sitemap generator writing to %s (host export path %s), interval %s", cfg.OutputDir, cfg.ExportPath, scheduler.Interval)

	db, err := repository.Open(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer func() { _ = db.Close() }()
	log.Printf("connected to database %s", cfg.DBDatabase)

	source := repository.NewMySQL(db)
	generate := func(runCtx context.Context) error {
		_, err := sitemap.Generate(runCtx, source, sitemap.Options{
			OutputDir:     cfg.OutputDir,
			TemplatesPath: cfg.TemplatesPath,
			AdminPanelURL: cfg.AdminPanelURL,
		})
		if err != nil {
			state.failure(err)
			return err
		}
		state.success()
		return nil
	}

	if cfg.RunOnce {
		if err := generate(ctx); err != nil {
			log.Fatalf("sitemap generation failed: %v", err)
		}
		return
	}

	if err := scheduler.Run(ctx, scheduler.Interval, generate); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("scheduler: %v", err)
	}
	log.Printf("sitemap scheduler stopped")
}

func startHealthServer(port string, state *runState) *http.Server {
	mux := http.NewServeMux()
	live := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}
	mux.HandleFunc("/live", live)
	mux.HandleFunc("/health", live)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !state.ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready\n"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("health server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("health server: %v", err)
		}
	}()
	return server
}
