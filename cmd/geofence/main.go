package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TomasB/geofence/internal/data"
	"github.com/TomasB/geofence/internal/handler/check"
	"github.com/TomasB/geofence/internal/handler/health"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize structured logging
	logLevel := getLogLevel(os.Getenv("LOG_LEVEL"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("service starting", "log_level", logLevel.String())

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Set Gin mode based on log level
	if logLevel == slog.LevelDebug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.New()

	// Add middleware
	router.Use(ginLogger(logger))
	router.Use(gin.Recovery())

	// Load MaxMind MMDB
	mmdbPath := os.Getenv("MMDB_PATH")
	if mmdbPath == "" {
		slog.Error("MMDB_PATH environment variable is required")
		os.Exit(1)
	}

	lookup, err := data.NewMmdbReader(mmdbPath)
	if err != nil {
		slog.Error("failed to open MMDB", "path", mmdbPath, "error", err)
		os.Exit(1)
	}
	defer lookup.Close()

	slog.Info("MMDB loaded", "path", mmdbPath)

	// Register health endpoints
	healthHandler := health.NewHandler()
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// Register API endpoints
	checkHandler := check.NewHandler(lookup)
	api := router.Group("/api/v1")
	{
		api.POST("/check", checkHandler.Check)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("service started", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("service shutting down")

	// Graceful shutdown with 30s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("service stopped")
}

// getLogLevel converts string log level to slog.Level
func getLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ginLogger creates a Gin middleware that logs using slog
func ginLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// Log request
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		attrs := []any{
			"method", method,
			"path", path,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
		}

		if len(c.Errors) > 0 {
			logger.Error("request completed with errors", append(attrs, "errors", c.Errors.String())...)
		} else if statusCode >= 500 {
			logger.Error("request completed", attrs...)
		} else if statusCode >= 400 {
			logger.Warn("request completed", attrs...)
		} else {
			logger.Info("request completed", attrs...)
		}
	}
}
