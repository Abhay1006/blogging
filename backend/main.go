package main

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"my-blog/backend/internal/store"
)

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Role     string `json:"role"`
}

type Blog struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	Likes     []string  `json:"likes"`
	Dislikes  []string  `json:"dislikes"`
	CreatedAt time.Time `json:"created_at"`
}

type Comment struct {
	ID        string    `json:"id"`
	BlogID    string    `json:"blog_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	db        *sql.DB
	jwtSecret []byte
)

func loadJWTSecret() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Println("[WARN] JWT_SECRET is not set; falling back to a well-known development secret.")
		log.Println("[WARN] Set JWT_SECRET in .env before exposing this server publicly.")
		secret = "secret"
	}
	jwtSecret = []byte(secret)
}

func authRequired(c *fiber.Ctx) error {
	tokenString := c.Get("Authorization")
	if tokenString == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Missing token"})
	}

	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}

	claims := token.Claims.(jwt.MapClaims)
	c.Locals("user_id", claims["user_id"])
	c.Locals("role", claims["role"])
	c.Locals("name", claims["name"])

	return c.Next()
}

func adminOnly(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "admin" {
		return c.Status(403).JSON(fiber.Map{"error": "Admin access required"})
	}
	return c.Next()
}

// blogExists reports whether a blog row is present, so reaction and comment
// handlers can still answer 404 the way the Mongo version did.
func blogExists(id string) (bool, error) {
	var found string
	err := db.QueryRow("SELECT id FROM blogs WHERE id = ?", id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// react applies the toggle rules for likes and dislikes. Because a reader has
// at most one row per blog, switching sides is a single upsert.
func react(c *fiber.Ctx, kind string) error {
	blogID := c.Params("id")
	userID, _ := c.Locals("user_id").(string)

	exists, err := blogExists(blogID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	if !exists {
		return c.Status(404).JSON(fiber.Map{"error": "Blog not found"})
	}

	var current string
	err = db.QueryRow(
		"SELECT kind FROM reactions WHERE blog_id = ? AND user_id = ?",
		blogID, userID,
	).Scan(&current)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// No reaction yet: record this one.
		_, err = db.Exec(
			"INSERT INTO reactions (blog_id, user_id, kind) VALUES (?, ?, ?)",
			blogID, userID, kind,
		)
	case err != nil:
		return c.Status(500).SendString(err.Error())
	case current == kind:
		// Same button pressed twice: toggle it off.
		_, err = db.Exec(
			"DELETE FROM reactions WHERE blog_id = ? AND user_id = ?",
			blogID, userID,
		)
	default:
		// Switching from like to dislike or back.
		_, err = db.Exec(
			"UPDATE reactions SET kind = ? WHERE blog_id = ? AND user_id = ?",
			kind, blogID, userID,
		)
	}

	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(200)
}

func registerRoutes(api fiber.Router) {
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "database": "sqlite"})
	})

	// --- Auth ---

	api.Post("/signup", func(c *fiber.Ctx) error {
		type SignupRequest struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var req SignupRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		var existing string
		err := db.QueryRow("SELECT id FROM users WHERE email = ?", req.Email).Scan(&existing)
		if err == nil {
			return c.Status(400).JSON(fiber.Map{"error": "User already exists"})
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return c.Status(500).JSON(fiber.Map{"error": "Could not create user"})
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not create user"})
		}

		role := "user"
		if req.Email == os.Getenv("ADMIN_EMAIL") {
			role = "admin"
		}

		user := User{
			ID:       store.NewID(),
			Name:     req.Name,
			Email:    req.Email,
			Password: string(hashedPassword),
			Role:     role,
		}

		_, err = db.Exec(
			"INSERT INTO users (id, name, email, password, role) VALUES (?, ?, ?, ?, ?)",
			user.ID, user.Name, user.Email, user.Password, user.Role,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not create user"})
		}

		return c.Status(201).JSON(fiber.Map{"message": "User created"})
	})

	api.Post("/login", func(c *fiber.Ctx) error {
		type LoginRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var req LoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		var user User
		err := db.QueryRow(
			"SELECT id, name, email, password, role FROM users WHERE email = ?",
			req.Email,
		).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &user.Role)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": user.ID,
			"role":    user.Role,
			"name":    user.Name,
			"exp":     time.Now().Add(time.Hour * 72).Unix(),
		})

		t, err := token.SignedString(jwtSecret)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not issue token"})
		}
		return c.JSON(fiber.Map{"token": t, "user": user})
	})

	// --- Blogs ---

	api.Get("/blogs", func(c *fiber.Ctx) error {
		rows, err := db.Query(
			"SELECT id, title, body, author, created_at FROM blogs ORDER BY created_at DESC",
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		defer rows.Close()

		blogs := []Blog{}
		ids := []string{}
		for rows.Next() {
			var b Blog
			var created string
			if err := rows.Scan(&b.ID, &b.Title, &b.Body, &b.Author, &created); err != nil {
				return c.Status(500).SendString(err.Error())
			}
			b.CreatedAt = store.ParseTime(created)
			blogs = append(blogs, b)
			ids = append(ids, b.ID)
		}
		if err := rows.Err(); err != nil {
			return c.Status(500).SendString(err.Error())
		}

		likes, dislikes, err := reactionsFor(ids)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		for i := range blogs {
			blogs[i].Likes = orEmpty(likes[blogs[i].ID])
			blogs[i].Dislikes = orEmpty(dislikes[blogs[i].ID])
		}

		return c.JSON(blogs)
	})

	api.Get("/blogs/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")

		var b Blog
		var created string
		err := db.QueryRow(
			"SELECT id, title, body, author, created_at FROM blogs WHERE id = ?", id,
		).Scan(&b.ID, &b.Title, &b.Body, &b.Author, &created)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Not found"})
		}
		b.CreatedAt = store.ParseTime(created)

		likes, dislikes, err := reactionsFor([]string{id})
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		b.Likes = orEmpty(likes[id])
		b.Dislikes = orEmpty(dislikes[id])

		return c.JSON(b)
	})

	api.Post("/blogs", authRequired, adminOnly, func(c *fiber.Ctx) error {
		blog := new(Blog)
		if err := c.BodyParser(blog); err != nil {
			return c.Status(400).SendString(err.Error())
		}

		blog.ID = store.NewID()
		blog.CreatedAt = time.Now().UTC()
		blog.Likes = []string{}
		blog.Dislikes = []string{}

		_, err := db.Exec(
			"INSERT INTO blogs (id, title, body, author, created_at) VALUES (?, ?, ?, ?, ?)",
			blog.ID, blog.Title, blog.Body, blog.Author, blog.CreatedAt.Format(store.TimeFormat),
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		return c.Status(201).JSON(blog)
	})

	api.Put("/blogs/:id", authRequired, adminOnly, func(c *fiber.Ctx) error {
		updateData := new(Blog)
		if err := c.BodyParser(updateData); err != nil {
			return c.Status(400).SendString(err.Error())
		}

		_, err := db.Exec(
			"UPDATE blogs SET title = ?, body = ? WHERE id = ?",
			updateData.Title, updateData.Body, c.Params("id"),
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendStatus(200)
	})

	// --- Interactions ---

	api.Post("/blogs/:id/like", authRequired, func(c *fiber.Ctx) error {
		return react(c, "like")
	})

	api.Post("/blogs/:id/dislike", authRequired, func(c *fiber.Ctx) error {
		return react(c, "dislike")
	})

	// --- Comments ---

	api.Get("/blogs/:id/comments", func(c *fiber.Ctx) error {
		rows, err := db.Query(
			"SELECT id, blog_id, user_id, user_name, content, created_at FROM comments WHERE blog_id = ? ORDER BY created_at ASC",
			c.Params("id"),
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		defer rows.Close()

		comments := []Comment{}
		for rows.Next() {
			var cm Comment
			var created string
			if err := rows.Scan(&cm.ID, &cm.BlogID, &cm.UserID, &cm.UserName, &cm.Content, &created); err != nil {
				return c.Status(500).SendString(err.Error())
			}
			cm.CreatedAt = store.ParseTime(created)
			comments = append(comments, cm)
		}
		if err := rows.Err(); err != nil {
			return c.Status(500).SendString(err.Error())
		}

		return c.JSON(comments)
	})

	api.Post("/blogs/:id/comments", authRequired, func(c *fiber.Ctx) error {
		blogID := c.Params("id")
		userID, _ := c.Locals("user_id").(string)
		userName, _ := c.Locals("name").(string)

		type CommentReq struct {
			Content string `json:"content"`
		}
		var req CommentReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		exists, err := blogExists(blogID)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		if !exists {
			return c.Status(404).JSON(fiber.Map{"error": "Blog not found"})
		}

		comment := Comment{
			ID:        store.NewID(),
			BlogID:    blogID,
			UserID:    userID,
			UserName:  userName,
			Content:   req.Content,
			CreatedAt: time.Now().UTC(),
		}

		_, err = db.Exec(
			"INSERT INTO comments (id, blog_id, user_id, user_name, content, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			comment.ID, comment.BlogID, comment.UserID, comment.UserName,
			comment.Content, comment.CreatedAt.Format(store.TimeFormat),
		)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		return c.Status(201).JSON(comment)
	})
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	loadJWTSecret()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "lumina.db"
	}

	log.Printf("[1/3] Opening local database at %s", dbPath)
	var err error
	db, err = store.Open(dbPath)
	if err != nil {
		log.Fatalf("could not open database: %v", err)
	}
	defer db.Close()
	log.Println("[2/3] Database ready.")

	app := fiber.New(fiber.Config{DisableStartupMessage: false})

	app.Use(func(c *fiber.Ctx) error {
		log.Printf("%s %s", c.Method(), c.Path())
		return c.Next()
	})

	// The frontend is served from this same origin in production, so CORS only
	// matters for the Vite dev server.
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173,http://127.0.0.1:5173"
	}
	origins := strings.Split(allowedOrigins, ",")
	for i, origin := range origins {
		origins[i] = strings.TrimSuffix(strings.TrimSpace(origin), "/")
	}
	cleanedOrigins := strings.Join(origins, ",")
	log.Printf("[CORS] Allowed Origins: %s", cleanedOrigins)

	app.Use(cors.New(cors.Config{
		AllowOrigins:     cleanedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// The API lives under /api so it cannot collide with the client-side
	// routes (/login, /signup) served by the single-page app below.
	registerRoutes(app.Group("/api"))

	// Serve the built frontend, when it has been deployed alongside the binary.
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./public"
	}
	if _, err := os.Stat(filepath.Join(staticDir, "index.html")); err == nil {
		log.Printf("[static] Serving frontend from %s", staticDir)
		// The shell must never be cached: a stale copy references hashed bundles
		// that a later deploy has deleted, and the page renders blank.
		sendShell := func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderCacheControl, "no-cache")
			return c.SendFile(filepath.Join(staticDir, "index.html"))
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
	} else {
		log.Printf("[static] No frontend build at %s; serving API only", staticDir)
		app.Get("/", func(c *fiber.Ctx) error {
			return c.SendString("Lumina API is running")
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	address := "0.0.0.0:" + port

	log.Printf("[3/3] Server is launching on %s", address)
	log.Fatal(app.Listen(address))
}
