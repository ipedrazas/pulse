package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/config"
	"github.com/ipedrazas/pulse/api/internal/db"
	"github.com/ipedrazas/pulse/api/internal/grpcserver"
	"github.com/ipedrazas/pulse/api/internal/metrics"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/ipedrazas/pulse/api/internal/rest"
	"github.com/ipedrazas/pulse/api/internal/version"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("pulse-api starting", "version", version.Version, "commit", version.Commit, "log_level", cfg.LogLevel.String())
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Database
	slog.Info("connecting to database")
	if err := db.RunMigrations(cfg.DBURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	pool, err := db.NewPool(ctx, cfg.DBURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := repository.NewPostgresRepository(pool)

	// Database health monitoring
	var dbHealthy atomic.Bool
	db.StartHealthCheck(ctx, pool, 30*time.Second, &dbHealthy)

	// Alerts
	notifier := alerts.NewNotifier(cfg.WebhookURL)

	// gRPC server
	agentSvc := grpcserver.NewAgentService(repo, notifier)
	cliSvc := grpcserver.NewCLIService(repo, agentSvc)

	grpcSrv := grpc.NewServer()
	pulsev1.RegisterAgentServiceServer(grpcSrv, agentSvc)
	pulsev1.RegisterCLIServiceServer(grpcSrv, cliSvc)
	reflection.Register(grpcSrv)

	grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("grpc listen failed", "addr", cfg.GRPCAddr, "error", err)
		os.Exit(1)
	}
	go func() {
		slog.Info("gRPC server listening", "addr", cfg.GRPCAddr)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			slog.Error("grpc serve failed", "error", err)
		}
	}()

	// Prometheus metrics
	metrics.Register()

	// REST server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(rest.CORSMiddleware())
	router.Use(rest.RequestIDMiddleware())
	router.Use(rest.MetricsMiddleware())
	router.Use(rest.LoggingMiddleware())

	handler := rest.NewHandler(repo, agentSvc)
	handler.Register(router)
	router.GET("/metrics", metrics.Handler())

	httpSrv := &http.Server{
		Addr:    cfg.RESTAddr,
		Handler: router,
	}
	go func() {
		slog.Info("REST server listening", "addr", cfg.RESTAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("rest serve failed", "error", err)
		}
	}()

	// Stale node detection
	go func() {
		ticker := time.NewTicker(cfg.StaleThreshold)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := repo.MarkStaleAgents(context.Background(), cfg.StaleThreshold)
				if err != nil {
					slog.Error("mark stale agents failed", "error", err)
				} else if n > 0 {
					slog.Warn("marked agents as lost", "count", n, "threshold", cfg.StaleThreshold)
				}
			}
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("shutting down")

	// Give in-flight requests up to 15 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("rest shutdown error", "error", err)
	}

	// GracefulStop waits for active RPCs; force-stop after the deadline
	stopped := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		slog.Warn("grpc graceful stop timed out, forcing stop")
		grpcSrv.Stop()
	}

	slog.Info("shutdown complete")
}
