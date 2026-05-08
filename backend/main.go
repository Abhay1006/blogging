package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name     string             `bson:"name" json:"name"`
	Email    string             `bson:"email" json:"email"`
	Password string             `bson:"password" json:"-"`
	Role     string             `bson:"role" json:"role"` // "admin" or "user"
}

type Blog struct {
	ID        primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	Title     string               `bson:"title" json:"title"`
	Body      string               `bson:"body" json:"body"`
	Author    string               `bson:"author" json:"author"`
	Likes     []primitive.ObjectID `bson:"likes" json:"likes"`
	Dislikes  []primitive.ObjectID `bson:"dislikes" json:"dislikes"`
	CreatedAt time.Time            `bson:"created_at" json:"created_at"`
}

type Comment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	BlogID    primitive.ObjectID `bson:"blog_id" json:"blog_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	UserName  string             `bson:"user_name" json:"user_name"`
	Content   string             `bson:"content" json:"content"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

var (
	blogCollection    *mongo.Collection
	userCollection    *mongo.Collection
	commentCollection *mongo.Collection
	jwtSecret         = []byte("secret")
)

func connectDB() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Connected to MongoDB")
	// Use the database name specified in the connection string if available, otherwise default to "lumina_db"
	dbName := "lumina_db"
	db := client.Database(dbName)
	blogCollection = db.Collection("blogs")
	userCollection = db.Collection("users")
	commentCollection = db.Collection("comments")
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
	role := c.Locals("role").(string)
	if role != "admin" {
		return c.Status(403).JSON(fiber.Map{"error": "Admin access required"})
	}
	return c.Next()
}

func main() {
	connectDB()

	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		log.Printf("%s %s - Origin: %s", c.Method(), c.Path(), c.Get("Origin"))
		return c.Next()
	})

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173,http://127.0.0.1:5173"
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	// --- Auth Routes ---

	app.Post("/signup", func(c *fiber.Ctx) error {
		type SignupRequest struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var req SignupRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		var existingUser User
		err := userCollection.FindOne(context.Background(), bson.M{"email": req.Email}).Decode(&existingUser)
		if err == nil {
			return c.Status(400).JSON(fiber.Map{"error": "User already exists"})
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
		
		role := "user"
		// Simple logic: if email is admin@lumina.com, make them admin
		if req.Email == os.Getenv("ADMIN_EMAIL") {
			role = "admin"
		}

		user := User{
			ID:       primitive.NewObjectID(),
			Name:     req.Name,
			Email:    req.Email,
			Password: string(hashedPassword),
			Role:     role,
		}

		_, err = userCollection.InsertOne(context.Background(), user)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Could not create user"})
		}

		return c.Status(201).JSON(fiber.Map{"message": "User created"})
	})

	app.Post("/login", func(c *fiber.Ctx) error {
		type LoginRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var req LoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		var user User
		err := userCollection.FindOne(context.Background(), bson.M{"email": req.Email}).Decode(&user)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": user.ID.Hex(),
			"role":    user.Role,
			"name":    user.Name,
			"exp":     time.Now().Add(time.Hour * 72).Unix(),
		})

		t, _ := token.SignedString(jwtSecret)
		return c.JSON(fiber.Map{"token": t, "user": user})
	})

	// --- Blog Routes ---

	app.Get("/blogs", func(c *fiber.Ctx) error {
		cursor, err := blogCollection.Find(context.Background(), bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		var blogs []Blog
		cursor.All(context.Background(), &blogs)
		return c.JSON(blogs)
	})

	app.Get("/blogs/:id", func(c *fiber.Ctx) error {
		id, _ := primitive.ObjectIDFromHex(c.Params("id"))
		var blog Blog
		err := blogCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&blog)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Not found"})
		}
		return c.JSON(blog)
	})

	app.Post("/blogs", authRequired, adminOnly, func(c *fiber.Ctx) error {
		blog := new(Blog)
		if err := c.BodyParser(blog); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		blog.ID = primitive.NewObjectID()
		blog.CreatedAt = time.Now()
		blog.Likes = []primitive.ObjectID{}
		blog.Dislikes = []primitive.ObjectID{}
		_, err := blogCollection.InsertOne(context.Background(), blog)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.Status(201).JSON(blog)
	})

	// --- Interaction Routes ---

	app.Post("/blogs/:id/like", authRequired, func(c *fiber.Ctx) error {
		blogID, _ := primitive.ObjectIDFromHex(c.Params("id"))
		userID, _ := primitive.ObjectIDFromHex(c.Locals("user_id").(string))

		// Pull from dislikes, push to likes (atomic)
		_, err := blogCollection.UpdateOne(context.Background(), bson.M{"_id": blogID}, bson.M{
			"$pull": bson.M{"dislikes": userID},
			"$addToSet": bson.M{"likes": userID},
		})
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendStatus(200)
	})

	app.Post("/blogs/:id/dislike", authRequired, func(c *fiber.Ctx) error {
		blogID, _ := primitive.ObjectIDFromHex(c.Params("id"))
		userID, _ := primitive.ObjectIDFromHex(c.Locals("user_id").(string))

		_, err := blogCollection.UpdateOne(context.Background(), bson.M{"_id": blogID}, bson.M{
			"$pull": bson.M{"likes": userID},
			"$addToSet": bson.M{"dislikes": userID},
		})
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendStatus(200)
	})

	app.Get("/blogs/:id/comments", func(c *fiber.Ctx) error {
		blogID, _ := primitive.ObjectIDFromHex(c.Params("id"))
		cursor, err := commentCollection.Find(context.Background(), bson.M{"blog_id": blogID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		var comments []Comment
		cursor.All(context.Background(), &comments)
		return c.JSON(comments)
	})

	app.Post("/blogs/:id/comments", authRequired, func(c *fiber.Ctx) error {
		blogID, _ := primitive.ObjectIDFromHex(c.Params("id"))
		userID, _ := primitive.ObjectIDFromHex(c.Locals("user_id").(string))
		userName := c.Locals("name").(string)

		type CommentReq struct {
			Content string `json:"content"`
		}
		var req CommentReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		comment := Comment{
			ID:        primitive.NewObjectID(),
			BlogID:    blogID,
			UserID:    userID,
			UserName:  userName,
			Content:   req.Content,
			CreatedAt: time.Now(),
		}

		_, err := commentCollection.InsertOne(context.Background(), comment)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.Status(201).JSON(comment)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Fatal(app.Listen(":" + port))
}
