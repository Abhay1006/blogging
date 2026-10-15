package main

import (
	"context"
	"errors"
	"log"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const (
	dbTimeout = 5 * time.Second

	maxNameLen     = 100
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt ignores anything longer, so reject it instead
	maxTitleLen    = 200
	maxBodyLen     = 200_000
	maxCommentLen  = 5_000
)

type Server struct {
	cfg      Config
	users    *mongo.Collection
	blogs    *mongo.Collection
	comments *mongo.Collection
}

func (s *Server) dbCtx(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), dbTimeout)
}

// internalError logs the real error and returns a generic message to the client.
func internalError(c *fiber.Ctx, what string, err error) error {
	log.Printf("[ERROR] %s %s: %s: %v", c.Method(), c.Path(), what, err)
	return fail(c, fiber.StatusInternalServerError, "Something went wrong. Please try again.")
}

func parseID(c *fiber.Ctx) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	return id, err == nil
}

// --- Auth ---

func (s *Server) signup(c *fiber.Ctx) error {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "Invalid request")
	}

	name := strings.TrimSpace(req.Name)
	email := normalizeEmail(req.Email)
	switch {
	case name == "" || utf8.RuneCountInString(name) > maxNameLen:
		return fail(c, fiber.StatusBadRequest, "Name is required (max 100 characters)")
	case !isValidEmail(email):
		return fail(c, fiber.StatusBadRequest, "A valid email is required")
	case len(req.Password) < minPasswordLen || len(req.Password) > maxPasswordLen:
		return fail(c, fiber.StatusBadRequest, "Password must be between 8 and 72 characters")
	}

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	err := s.users.FindOne(ctx, bson.M{"email": email}, options.FindOne().SetCollation(caseInsensitive)).Err()
	if err == nil {
		return fail(c, fiber.StatusConflict, "An account with this email already exists")
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return internalError(c, "find user", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return internalError(c, "hash password", err)
	}

	role := roleUser
	if s.cfg.AdminEmail != "" && email == s.cfg.AdminEmail {
		role = roleAdmin
	}

	user := User{
		ID:       primitive.NewObjectID(),
		Name:     name,
		Email:    email,
		Password: string(hashedPassword),
		Role:     role,
	}
	if _, err := s.users.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fail(c, fiber.StatusConflict, "An account with this email already exists")
		}
		return internalError(c, "insert user", err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "User created"})
}

func (s *Server) login(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "Invalid request")
	}

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	var user User
	err := s.users.FindOne(ctx, bson.M{"email": normalizeEmail(req.Email)}, options.FindOne().SetCollation(caseInsensitive)).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return fail(c, fiber.StatusUnauthorized, "Invalid email or password")
	}
	if err != nil {
		return internalError(c, "find user", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return fail(c, fiber.StatusUnauthorized, "Invalid email or password")
	}

	token, err := s.signToken(user)
	if err != nil {
		return internalError(c, "sign token", err)
	}
	return c.JSON(fiber.Map{"token": token, "user": user})
}

func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

// --- Blogs ---

func (s *Server) listBlogs(c *fiber.Ctx) error {
	ctx, cancel := s.dbCtx(c)
	defer cancel()

	cursor, err := s.blogs.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return internalError(c, "find blogs", err)
	}
	var blogs []Blog
	if err := cursor.All(ctx, &blogs); err != nil {
		return internalError(c, "decode blogs", err)
	}

	viewer := viewerID(c)
	resp := make([]BlogResponse, 0, len(blogs))
	for _, b := range blogs {
		resp = append(resp, toBlogResponse(b, viewer))
	}
	return c.JSON(resp)
}

func (s *Server) getBlog(c *fiber.Ctx) error {
	id, ok := parseID(c)
	if !ok {
		return fail(c, fiber.StatusBadRequest, "Invalid blog id")
	}

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	var blog Blog
	err := s.blogs.FindOne(ctx, bson.M{"_id": id}).Decode(&blog)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return fail(c, fiber.StatusNotFound, "Blog not found")
	}
	if err != nil {
		return internalError(c, "find blog", err)
	}
	return c.JSON(toBlogResponse(blog, viewerID(c)))
}

type blogInput struct {
	Title *string `json:"title"`
	Body  *string `json:"body"`
}

// validate trims the provided fields and checks them. When partial is false both
// fields are required.
func (in *blogInput) validate(partial bool) string {
	if in.Title == nil && in.Body == nil {
		return "Title or body is required"
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		in.Title = &t
		if t == "" || utf8.RuneCountInString(t) > maxTitleLen {
			return "Title is required (max 200 characters)"
		}
	} else if !partial {
		return "Title is required"
	}
	if in.Body != nil {
		if strings.TrimSpace(*in.Body) == "" || len(*in.Body) > maxBodyLen {
			return "Body is required"
		}
	} else if !partial {
		return "Body is required"
	}
	return ""
}

func (s *Server) createBlog(c *fiber.Ctx) error {
	var in blogInput
	if err := c.BodyParser(&in); err != nil {
		return fail(c, fiber.StatusBadRequest, "Invalid request")
	}
	if msg := in.validate(false); msg != "" {
		return fail(c, fiber.StatusBadRequest, msg)
	}

	user, _ := currentUser(c)
	blog := Blog{
		ID:        primitive.NewObjectID(),
		Title:     *in.Title,
		Body:      *in.Body,
		Author:    user.Name,
		Likes:     []primitive.ObjectID{},
		Dislikes:  []primitive.ObjectID{},
		CreatedAt: time.Now(),
	}

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	if _, err := s.blogs.InsertOne(ctx, blog); err != nil {
		return internalError(c, "insert blog", err)
	}
	return c.Status(fiber.StatusCreated).JSON(toBlogResponse(blog, &user.ID))
}

func (s *Server) updateBlog(c *fiber.Ctx) error {
	id, ok := parseID(c)
	if !ok {
		return fail(c, fiber.StatusBadRequest, "Invalid blog id")
	}
	var in blogInput
	if err := c.BodyParser(&in); err != nil {
		return fail(c, fiber.StatusBadRequest, "Invalid request")
	}
	if msg := in.validate(true); msg != "" {
		return fail(c, fiber.StatusBadRequest, msg)
	}

	set := bson.M{}
	if in.Title != nil {
		set["title"] = *in.Title
	}
	if in.Body != nil {
		set["body"] = *in.Body
	}

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	var blog Blog
	err := s.blogs.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&blog)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return fail(c, fiber.StatusNotFound, "Blog not found")
	}
	if err != nil {
		return internalError(c, "update blog", err)
	}
	return c.JSON(toBlogResponse(blog, viewerID(c)))
}

// --- Interactions ---

func (s *Server) like(c *fiber.Ctx) error    { return s.toggleVote(c, "likes", "dislikes") }
func (s *Server) dislike(c *fiber.Ctx) error { return s.toggleVote(c, "dislikes", "likes") }

// toggleVote removes the caller's vote from field if present; otherwise it adds
// it and removes any opposite vote. Each step is a single atomic update, so
// concurrent requests cannot leave a user in both lists.
func (s *Server) toggleVote(c *fiber.Ctx, field, opposite string) error {
	blogID, ok := parseID(c)
	if !ok {
		return fail(c, fiber.StatusBadRequest, "Invalid blog id")
	}
	user, _ := currentUser(c)

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	after := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var blog Blog

	// Already voted this way: remove the vote.
	err := s.blogs.FindOneAndUpdate(ctx,
		bson.M{"_id": blogID, field: user.ID},
		bson.M{"$pull": bson.M{field: user.ID}},
		after).Decode(&blog)

	if errors.Is(err, mongo.ErrNoDocuments) {
		// Not voted this way yet: add the vote and drop the opposite one.
		err = s.blogs.FindOneAndUpdate(ctx,
			bson.M{"_id": blogID},
			bson.M{"$pull": bson.M{opposite: user.ID}, "$addToSet": bson.M{field: user.ID}},
			after).Decode(&blog)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fail(c, fiber.StatusNotFound, "Blog not found")
		}
	}
	if err != nil {
		return internalError(c, "toggle "+field, err)
	}
	return c.JSON(toBlogResponse(blog, &user.ID))
}

// --- Comments ---

func (s *Server) listComments(c *fiber.Ctx) error {
	blogID, ok := parseID(c)
	if !ok {
		return fail(c, fiber.StatusBadRequest, "Invalid blog id")
	}

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	cursor, err := s.comments.Find(ctx, bson.M{"blog_id": blogID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return internalError(c, "find comments", err)
	}
	comments := []Comment{}
	if err := cursor.All(ctx, &comments); err != nil {
		return internalError(c, "decode comments", err)
	}
	return c.JSON(comments)
}

func (s *Server) createComment(c *fiber.Ctx) error {
	blogID, ok := parseID(c)
	if !ok {
		return fail(c, fiber.StatusBadRequest, "Invalid blog id")
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "Invalid request")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" || utf8.RuneCountInString(content) > maxCommentLen {
		return fail(c, fiber.StatusBadRequest, "Comment must be between 1 and 5000 characters")
	}
	user, _ := currentUser(c)

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	count, err := s.blogs.CountDocuments(ctx, bson.M{"_id": blogID}, options.Count().SetLimit(1))
	if err != nil {
		return internalError(c, "find blog", err)
	}
	if count == 0 {
		return fail(c, fiber.StatusNotFound, "Blog not found")
	}

	comment := Comment{
		ID:        primitive.NewObjectID(),
		BlogID:    blogID,
		UserID:    user.ID,
		UserName:  user.Name,
		Content:   content,
		CreatedAt: time.Now(),
	}
	if _, err := s.comments.InsertOne(ctx, comment); err != nil {
		return internalError(c, "insert comment", err)
	}
	return c.Status(fiber.StatusCreated).JSON(comment)
}
