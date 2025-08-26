package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/trianglehasfoursides/begadangz/form"
	"github.com/trianglehasfoursides/begadangz/internal"
	"github.com/trianglehasfoursides/begadangz/link"

	"github.com/trianglehasfoursides/begadangz/page"
	"github.com/trianglehasfoursides/begadangz/tools"
)

const (
	// Default configuration values
	DefaultPort         = "8000"
	DefaultReadTimeout  = 30 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	DefaultIdleTimeout  = 120 * time.Second
	ShutdownTimeout     = 15 * time.Second
)

// Config holds application configuration
type Config struct {
	Port         string
	Environment  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Application holds the application dependencies
type Application struct {
	config *Config
	logger *slog.Logger
	router *gin.Engine
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}

	env := os.Getenv("GIN_MODE")
	if env == "" {
		env = "debug"
	}

	readTimeout := DefaultReadTimeout
	if rt := os.Getenv("READ_TIMEOUT"); rt != "" {
		if duration, err := time.ParseDuration(rt); err == nil {
			readTimeout = duration
		}
	}

	writeTimeout := DefaultWriteTimeout
	if wt := os.Getenv("WRITE_TIMEOUT"); wt != "" {
		if duration, err := time.ParseDuration(wt); err == nil {
			writeTimeout = duration
		}
	}

	idleTimeout := DefaultIdleTimeout
	if it := os.Getenv("IDLE_TIMEOUT"); it != "" {
		if duration, err := time.ParseDuration(it); err == nil {
			idleTimeout = duration
		}
	}

	return &Config{
		Port:         port,
		Environment:  env,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
}

// setupLogger configures structured logging
func setupLogger(env string) *slog.Logger {
	var handler slog.Handler

	switch env {
	case "production", "prod":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// initializeDatabase sets up database connections and runs migrations
func initializeDatabase(logger *slog.Logger) error {
	logger.Info("Setting up database connection...")

	if err := internal.Setup(); err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}

	logger.Info("Setting up ClickHouse connection...")
	link.Setup()

	if err := link.Click.Ping(); err != nil {
		logger.Warn("ClickHouse connection failed", "error", err)
		return fmt.Errorf("clickhouse connection failed: %w", err)
	}

	logger.Info("Running database migrations...")

	// Initialize model instances for migration
	models := []any{
		&link.URL{},
		&link.Geo{},
		&internal.User{},
		&tools.Todo{},
		&tools.Note{},
		&page.Page{},
		&page.Link{},
	}

	if err := internal.DB.AutoMigrate(models...); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("Database setup completed successfully")
	return nil
}

// setupMiddlewares configures global middlewares
func setupMiddlewares(router *gin.Engine, logger *slog.Logger) {
	// Request ID middleware
	router.Use(requestid.New())

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Configure properly for production
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"*"}, // Configure properly for production
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Custom logging middleware
	router.Use(func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		raw := ctx.Request.URL.RawQuery
		requestID := requestid.Get(ctx)

		// Process request
		ctx.Next()

		// Log request details
		latency := time.Since(start)
		clientIP := ctx.ClientIP()
		method := ctx.Request.Method
		statusCode := ctx.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Info("HTTP Request",
			"request_id", requestID,
			"status", statusCode,
			"latency", latency,
			"client_ip", clientIP,
			"method", method,
			"path", path,
			"user_agent", ctx.Request.UserAgent(),
		)
	})

	// Recovery middleware with custom logging
	router.Use(gin.CustomRecovery(func(ctx *gin.Context, recovered any) {
		if err, ok := recovered.(string); ok {
			logger.Error("Panic recovered",
				"error", err,
				"request_id", requestid.Get(ctx),
				"path", ctx.Request.URL.Path,
			)
		}
		ctx.AbortWithStatus(http.StatusInternalServerError)
	}))
}

// setupRoutes configures all application routes
func setupRoutes(router *gin.Engine, logger *slog.Logger) {
	// Initialize handlers
	u := &link.URL{}
	t := &tools.Todo{}
	n := &tools.Note{}
	p := &page.Page{}
	f := &form.Form{}
	fs := &form.FormSubmission{}

	// Health check endpoints
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	router.GET("/health/ready", func(ctx *gin.Context) {
		// Check database connection
		sqlDB, err := internal.DB.DB()
		if err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "database connection failed",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "database ping failed",
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	// Public routes
	router.GET("/", u.View)
	router.GET("/auth/:provider/callback", internal.Callback)
	router.GET("/auth/:provider", internal.Redirect)

	// API routes with authentication
	api := router.Group("/api")
	api.Use(internal.Auth)
	{
		// URL/Domain management
		domains := api.Group("/links")
		{
			domains.POST("/add", u.Add)
			domains.GET("/clicks/:name", u.Clicks)
			domains.GET("/qr/:name", u.Qr)
			domains.GET("/:name", u.Get)
			domains.PATCH("/:name", u.Edit)
			domains.DELETE("/:name", u.Delete)
		}

		// Todo management
		todos := api.Group("/todos")
		{
			todos.POST("/add", t.Add)
			todos.GET("/check/:name", t.Check)
			todos.GET("/filter/:name", t.Filter)
			todos.GET("/list/:filter", t.List)
			todos.PATCH("/:name", t.Edit)
			todos.DELETE("/clear", t.Clear)
		}

		// Note management
		notes := api.Group("/notes")
		{
			notes.GET("", n.List)
			notes.POST("/add", n.Add)
			notes.GET("/:name", n.Get)
			notes.PATCH("/:name", n.Edit)
			notes.DELETE("/:name", n.Remove)
		}

		// Page/Link management
		pages := api.Group("/pages")
		{
			pages.POST("", p.Create)
			pages.GET("", p.GetPage)
			pages.GET("/stats", p.GetLinkStats)
			pages.DELETE("", p.DeletePage)
		}

		links := api.Group("/links")
		{
			links.POST("", p.AddLink)
			links.PATCH("/:name", p.EditLink)
			links.DELETE("/:name", p.DeleteLink)
		}

		// User management
		users := api.Group("/user")
		{
			users.GET("/profile", func(ctx *gin.Context) {
				userID := internal.UserId(ctx)
				ctx.JSON(http.StatusOK, gin.H{
					"user_id": userID,
				})
			})
		}

		forms := api.Group("/form")
		{
			forms.POST("", f.Create)
			forms.GET("", f.List)
			forms.GET("/fs", fs.ListSubmissions)
			forms.GET("/fs/:id", fs.GetSubmission)
			forms.DELETE("/fs/:id", fs.DeleteSubmission)
			forms.PATCH("/:id", f.Update)
			forms.DELETE("/:id", f.Delete)
		}
	}

	logger.Info("Routes configured successfully")
}

// createServer creates and configures the HTTP server
func (app *Application) createServer() *http.Server {
	return &http.Server{
		Addr:         ":" + app.config.Port,
		Handler:      app.router,
		ReadTimeout:  app.config.ReadTimeout,
		WriteTimeout: app.config.WriteTimeout,
		IdleTimeout:  app.config.IdleTimeout,
		ErrorLog:     log.New(os.Stderr, "SERVER ERROR ", log.LstdFlags),
	}
}

// start starts the HTTP server with graceful shutdown
func (app *Application) start() error {
	server := app.createServer()

	// Channel to receive OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Channel to receive server errors
	serverErrors := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		app.logger.Info("Starting server",
			"port", app.config.Port,
			"environment", app.config.Environment,
		)

		serverErrors <- server.ListenAndServe()
	}()

	// Wait for either server error or shutdown signal
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server failed to start: %w", err)
		}

	case sig := <-quit:
		app.logger.Info("Received shutdown signal", "signal", sig)

		// Create context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()

		// Attempt graceful shutdown
		if err := server.Shutdown(ctx); err != nil {
			app.logger.Error("Server shutdown failed", "error", err)
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		app.logger.Info("Server shutdown completed successfully")
	}

	return nil
}

// validateEnvironment checks required environment variables
func validateEnvironment() error {
	required := []string{
		"DB_HOST",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
	}

	var missing []string
	for _, env := range required {
		if os.Getenv(env) == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	return nil
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// Not critical if .env file doesn't exist in production
		slog.Debug("No .env file found, using system environment variables")
	}

	// Load configuration
	config := loadConfig()

	// Set Gin mode
	gin.SetMode(config.Environment)

	// Setup logger
	logger := setupLogger(config.Environment)

	// Validate environment
	if err := validateEnvironment(); err != nil {
		logger.Error("Environment validation failed", "error", err)
		os.Exit(1)
	}

	// Initialize database
	if err := initializeDatabase(logger); err != nil {
		logger.Error("Database initialization failed", "error", err)
		os.Exit(1)
	}

	// Create application instance
	app := &Application{
		config: config,
		logger: logger,
		router: gin.New(),
	}

	// Setup middlewares and routes
	setupMiddlewares(app.router, logger)
	setupRoutes(app.router, logger)

	// Start server
	logger.Info("Application starting up",
		"version", "1.0.0", // You can make this dynamic
		"environment", config.Environment,
	)

	if err := app.start(); err != nil {
		logger.Error("Application failed to start", "error", err)
		os.Exit(1)
	}

	logger.Info("Application shutdown complete")
}
