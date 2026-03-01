package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/ipedrazas/pulse/api/internal/config"
	"github.com/ipedrazas/pulse/api/internal/db"
	grpcserver "github.com/ipedrazas/pulse/api/internal/grpcserver"
	"github.com/ipedrazas/pulse/api/internal/rest"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Run database migrations
	if err := db.RunMigrations(cfg.DBURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Create database connection pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		slog.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to ping db", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected")

	// gRPC server
	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcserver.TokenAuthInterceptor(cfg.MonitorToken)),
	)
	monitorv1.RegisterMonitoringServiceServer(grpcSrv, grpcserver.NewMonitoringService(pool))

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("failed to listen for grpc", "port", cfg.GRPCPort, "error", err)
		os.Exit(1)
	}

	// HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	handler := rest.NewHandler(pool)
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
