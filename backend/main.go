package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"my-blog/backend/internal/store"
)

func newApp(s *Server) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
		BodyLimit:    1 << 20, // 1 MiB
	})

	app.Use(recover.New())
	app.Use(logger.New())
	// The frontend is served from this same origin in production, so CORS only
	// matters for the Vite dev server.
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Join(s.cfg.AllowedOrigins, ","),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// The API lives under /api so it cannot collide with the client-side
	// routes (/login, /signup) served by the single-page app below.
	api := app.Group("/api")

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "database": "sqlite"})
	})

	api.Post("/signup", s.signup)
	api.Post("/login", s.login)

	api.Get("/blogs", s.optionalAuth, s.listBlogs)
	api.Get("/blogs/:id", s.optionalAuth, s.getBlog)
	api.Post("/blogs", s.authRequired, adminOnly, s.createBlog)
	api.Put("/blogs/:id", s.authRequired, adminOnly, s.updateBlog)

	api.Post("/blogs/:id/like", s.authRequired, s.like)
	api.Post("/blogs/:id/dislike", s.authRequired, s.dislike)

	api.Get("/blogs/:id/comments", s.listComments)
	api.Post("/blogs/:id/comments", s.authRequired, s.createComment)

	// Unknown API paths get a JSON 404 rather than the SPA shell.
	api.Use(func(c *fiber.Ctx) error {
		return fail(c, fiber.StatusNotFound, "Not found")
	})

	serveFrontend(app, s.cfg.StaticDir)
	return app
}

// serveFrontend serves the built single-page app, when it has been deployed
// alongside the binary.
func serveFrontend(app *fiber.App, staticDir string) {
	if staticDir == "" {
		return
	}
	shell := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(shell); err != nil {
		log.Printf("[static] No frontend build at %s; serving API only", staticDir)
		return
	}
	log.Printf("[static] Serving frontend from %s", staticDir)

	// The shell must never be cached: a stale copy references hashed bundles
	// that a later deploy has deleted, and the page renders blank.
	sendShell := func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-cache")
		return c.SendFile(shell)
	}
	app.Get("/", sendShell)
	app.Get("/index.html", sendShell)

	// Hashed bundles never change, so they can be cached indefinitely.
	app.Static("/assets", filepath.Join(staticDir, "assets"), fiber.Static{Compress: true, MaxAge: 31536000})
	// A missing bundle is a real 404. Answering with the HTML shell instead
	// makes the browser reject it as a script and leaves a white screen.
	app.Get("/assets/*", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNotFound)
	})
	app.Static("/", staticDir, fiber.Static{Compress: true, MaxAge: 3600})

	// Any unmatched GET is a client-side route: hand back the SPA shell.
	app.Get("/*", sendShell)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	log.Printf("[CORS] Allowed origins: %s", strings.Join(cfg.AllowedOrigins, ", "))

	log.Printf("Opening local database at %s", cfg.DBPath)
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Could not open database: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := newApp(&Server{cfg: cfg, db: db})

	go func() {
		<-ctx.Done()
		log.Println("Shutting down...")
		if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	address := "0.0.0.0:" + cfg.Port
	log.Printf("Server listening on %s", address)
	if err := app.Listen(address); err != nil {
		log.Printf("Server error: %v", err)
	}

	if err := db.Close(); err != nil {
		log.Printf("Database close error: %v", err)
	}
}
