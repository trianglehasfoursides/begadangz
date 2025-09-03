package main

import (
	"context"
	"fmt"
	"io"
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

	"github.com/charmbracelet/log"
	"golang.org/x/sync/errgroup"
)

type HTTPApplication struct {
	logger *log.Logger
	router *gin.Engine
}

func setupLog() *log.Logger {
	f, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// gabungin stderr dan file jadi satu writer
	mw := io.MultiWriter(os.Stderr, f)

	// logger tulis ke dua tempat sekaligus
	return log.New(mw)
}

// initializeDatabase sets up database connections and runs migrations
func setupDatabase(logger *log.Logger) error {
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
func (h *HTTPApplication) middlewares() {
	// Request ID middleware
	h.router.Use(requestid.New())

	// CORS middleware
	h.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Configure properly for production
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"*"}, // Configure properly for production
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Custom logging middleware
	h.router.Use(func(ctx *gin.Context) {
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

		h.logger.Info("HTTP Request",
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
	h.router.Use(gin.CustomRecovery(func(ctx *gin.Context, recovered any) {
		if err, ok := recovered.(string); ok {
			h.logger.Error("Panic recovered",
				"error", err,
				"request_id", requestid.Get(ctx),
				"path", ctx.Request.URL.Path,
			)
		}
		ctx.AbortWithStatus(http.StatusInternalServerError)
	}))
}

// setupRoutes configures all application routes
func (h *HTTPApplication) routes() {
	// Initialize handlers
	u := &link.URL{}
	t := &tools.Todo{}
	n := &tools.Note{}
	p := &page.Page{}
	f := &form.Form{}
	fs := &form.FormSubmission{}

	// Health check endpoints
	h.router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	h.router.GET("/health/ready", func(ctx *gin.Context) {
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
	h.router.GET("/", u.View)
	h.router.GET("/auth/:provider/callback", internal.Callback)
	h.router.GET("/auth/:provider", internal.Redirect)

	// API routes with authentication
	api := h.router.Group("/api")
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

	h.logger.Info("Routes configured successfully")
}

// createServer creates and configures the HTTP server
func (app *HTTPApplication) create() *http.Server {
	return &http.Server{
		Addr: ":" + "8000",
	}
}

// start starts the HTTP server with graceful shutdown
func (app *HTTPApplication) start() error {
	server := app.create()

	app.middlewares()
	app.routes()

	// Channel to receive OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Channel to receive server errors
	serverErrors := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		app.logger.Info("Starting server",
			"port", "8000",
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

		server.Shutdown(context.Background())

		app.logger.Info("Server shutdown completed successfully")
	}

	return nil
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// Not critical if .env file doesn't exist in production
		log.Debug("No .env file found, using system environment variables")
	}

	// Setup logger
	logger := setupLog()

	// Initialize database
	if err := setupDatabase(logger); err != nil {
		logger.Error("Database initialization failed", "error", err)
		os.Exit(1)
	}

	// storage
	if err := internal.StorageSetup(); err != nil {
		logger.Fatal(err.Error())
	}

	group, err := errgroup.WithContext(context.Background())
	if err != nil {
		logger.Fatal(err.Err().Error())
	}

	group.Go(func() error {
		app := &HTTPApplication{
			logger: logger,
			router: gin.Default(),
		}

		if err := app.start(); err != nil {
			return err
		}

		return nil
	})

	logger.Info("Application shutdown complete")
}
