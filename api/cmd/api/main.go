package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/config"
	"github.com/ipedrazas/pulse/api/internal/db"
	"github.com/ipedrazas/pulse/api/internal/grpcserver"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/ipedrazas/pulse/api/internal/rest"
	"github.com/ipedrazas/pulse/api/internal/version"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("pulse-api starting", "version", version.Version, "commit", version.Commit)

	cfg := config.Load()
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

	// REST server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(rest.CORSMiddleware())
	router.Use(rest.LoggingMiddleware())

	handler := rest.NewHandler(repo, agentSvc)
	handler.Register(router)

	go func() {
		slog.Info("REST server listening", "addr", cfg.RESTAddr)
		if err := router.Run(cfg.RESTAddr); err != nil {
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

	// Wait for shutdown
	<-ctx.Done()
	slog.Info("shutting down")
	grpcSrv.GracefulStop()
}
