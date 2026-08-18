package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devcapsule/backend/internal/api"
	"devcapsule/backend/internal/config"
	"devcapsule/backend/internal/docker"
	"devcapsule/backend/internal/store"
)

func main() {
	logger := log.New(os.Stdout, "[devcapsule] ", log.LstdFlags)
	cfg := config.Load()
	if cfg.JWTSecret == "dev-secret-change-me" {
		logger.Printf("WARNING: using the default JWT_SECRET; set a strong secret in production")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("connect database: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		logger.Fatalf("migrate: %v", err)
	}

	dc, err := docker.NewClient()
	if err != nil {
		logger.Fatalf("connect docker: %v", err)
	}
	defer dc.Close()

	srv, err := api.New(cfg, st, dc, logger)
	if err != nil {
		logger.Fatalf("build server: %v", err)
	}
	if err := srv.EnsureSystemTemplates(ctx); err != nil {
		logger.Fatalf("bootstrap system templates: %v", err)
	}
	if err := dc.EnsureNetwork(ctx, cfg.NetworkName); err != nil {
		logger.Fatalf("ensure network: %v", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 30 * time.Second,
		// Generous read window so multi-GB image imports over slower links
		// are not cut off. WriteTimeout is intentionally omitted: WebSocket
		// and SSE streams proxied to user containers can stay open indefinitely.
		ReadTimeout:    600 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go srv.StartBackground(ctx)
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shCtx)
	}()

	logger.Printf("listening on %s", cfg.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("serve: %v", err)
	}
}
