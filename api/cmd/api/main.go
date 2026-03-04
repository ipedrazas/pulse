package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/config"
	"github.com/ipedrazas/pulse/api/internal/db"
	grpcserver "github.com/ipedrazas/pulse/api/internal/grpcserver"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/ipedrazas/pulse/api/internal/rest"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

func initDatabase(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	if err := db.RunMigrations(dbURL); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	slog.Info("database connected")
	return pool, nil
}

func setupGRPC(cfg *config.Config, repo *repository.PostgresRepo, notifier *alerts.Notifier) (*grpc.Server, *grpcserver.MonitoringService, error) {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpcserver.TokenAuthInterceptor(cfg.MonitorToken)),
	}

	if cfg.TLSCertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("load TLS certificate: %w", err)
		}
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
		slog.Info("gRPC TLS enabled", "cert", cfg.TLSCertFile)
	} else {
		slog.Info("gRPC TLS disabled (no TLS_CERT_FILE set)")
	}

	srv := grpc.NewServer(opts...)
	svc := grpcserver.NewMonitoringService(repo, repo, repo, notifier)
	monitorv1.RegisterMonitoringServiceServer(srv, svc)

	return srv, svc, nil
}

func startSweeper(ctx context.Context, svc *grpcserver.MonitoringService) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			swept, err := svc.SweepStaleContainers(ctx, 48*time.Hour)
			if err != nil {
				slog.Error("stale container sweep failed", "error", err)
			} else if swept > 0 {
				slog.Info("stale containers swept", "count", swept)
			}
		case <-ctx.Done():
			return
		}
	}
}

func startAgentChecker(ctx context.Context, svc *grpcserver.MonitoringService) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			svc.CheckAgentStatus(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := initDatabase(ctx, cfg.DBURL)
	if err != nil {
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := repository.NewPostgresRepo(pool)
	notifier := alerts.NewNotifier(cfg.WebhookURL, cfg.WebhookEvents)
	grpcSrv, monSvc, err := setupGRPC(cfg, repo, notifier)
	if err != nil {
		slog.Error("gRPC setup failed", "error", err)
		os.Exit(1)
	}

	go startSweeper(ctx, monSvc)
	go startAgentChecker(ctx, monSvc)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("failed to listen for grpc", "port", cfg.GRPCPort, "error", err)
		os.Exit(1)
	}

	// HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	handler := rest.NewHandler(repo, cfg.RESTToken)
	handler.RegisterRoutes(router)

	httpSrv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	// Start servers
	go func() {
		slog.Info("grpc server starting", "port", cfg.GRPCPort)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			slog.Error("grpc server error", "error", err)
		}
	}()

	go func() {
		slog.Info("http server starting", "port", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	cancel()
	grpcSrv.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*1e9)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	slog.Info("servers stopped")
}
