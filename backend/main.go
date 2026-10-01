package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func newApp(s *Server) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
		BodyLimit:    1 << 20, // 1 MiB
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Join(s.cfg.AllowedOrigins, ","),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Health check (used by Render)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Lumina API is running")
	})

	app.Post("/signup", s.signup)
	app.Post("/login", s.login)

	app.Get("/blogs", s.optionalAuth, s.listBlogs)
	app.Get("/blogs/:id", s.optionalAuth, s.getBlog)
	app.Post("/blogs", s.authRequired, adminOnly, s.createBlog)
	app.Put("/blogs/:id", s.authRequired, adminOnly, s.updateBlog)

	app.Post("/blogs/:id/like", s.authRequired, s.like)
	app.Post("/blogs/:id/dislike", s.authRequired, s.dislike)

	app.Get("/blogs/:id/comments", s.listComments)
	app.Post("/blogs/:id/comments", s.authRequired, s.createComment)

	return app
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, db, err := connectDB(ctx, cfg)
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB")
	ensureIndexes(ctx, db)

	app := newApp(&Server{
		cfg:      cfg,
		users:    db.Collection("users"),
		blogs:    db.Collection("blogs"),
		comments: db.Collection("comments"),
	})

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

	disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Disconnect(disconnectCtx); err != nil {
		log.Printf("MongoDB disconnect error: %v", err)
	}
}
