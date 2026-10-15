package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"my-blog/backend/internal/store"
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
	cfg Config
	db  *sql.DB
}

func (s *Server) dbCtx(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), dbTimeout)
}

// internalError logs the real error and returns a generic message to the client.
func internalError(c *fiber.Ctx, what string, err error) error {
	log.Printf("[ERROR] %s %s: %s: %v", c.Method(), c.Path(), what, err)
	return fail(c, fiber.StatusInternalServerError, "Something went wrong. Please try again.")
}

// isValidID reports whether id has the shape store.NewID produces.
func isValidID(id string) bool {
	if len(id) != 24 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func parseID(c *fiber.Ctx) (string, bool) {
	id := c.Params("id")
	return id, isValidID(id)
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

	// NOCASE so accounts created before emails were normalized still collide.
	var existing string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE email = ? COLLATE NOCASE", email).Scan(&existing)
	if err == nil {
		return fail(c, fiber.StatusConflict, "An account with this email already exists")
	}
	if !errors.Is(err, sql.ErrNoRows) {
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

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO users (id, name, email, password, role) VALUES (?, ?, ?, ?, ?)",
		store.NewID(), name, email, string(hashedPassword), role,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
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
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, email, password, role FROM users WHERE email = ? COLLATE NOCASE",
		normalizeEmail(req.Email),
	).Scan(&user.ID, &user.Name, &user.Email, &user.Password, &user.Role)
	if errors.Is(err, sql.ErrNoRows) {
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

// queryBlogs loads blogs matching where (with args) and attaches their
// reactions, using one query for the blogs and one for all their reactions.
func (s *Server) queryBlogs(ctx context.Context, where string, args ...any) ([]Blog, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, title, body, author, created_at FROM blogs "+where+" ORDER BY created_at DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blogs := []Blog{}
	index := map[string]int{}
	for rows.Next() {
		var b Blog
		var created string
		if err := rows.Scan(&b.ID, &b.Title, &b.Body, &b.Author, &created); err != nil {
			return nil, err
		}
		b.CreatedAt = store.ParseTime(created)
		index[b.ID] = len(blogs)
		blogs = append(blogs, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(blogs) == 0 {
		return blogs, nil
	}

	ids := make([]any, 0, len(blogs))
	for _, b := range blogs {
		ids = append(ids, b.ID)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	rrows, err := s.db.QueryContext(ctx,
		"SELECT blog_id, user_id, kind FROM reactions WHERE blog_id IN ("+placeholders+")", ids...)
	if err != nil {
		return nil, err
	}
	defer rrows.Close()

	for rrows.Next() {
		var blogID, userID, kind string
		if err := rrows.Scan(&blogID, &userID, &kind); err != nil {
			return nil, err
		}
		b := &blogs[index[blogID]]
		if kind == "like" {
			b.Likes = append(b.Likes, userID)
		} else {
			b.Dislikes = append(b.Dislikes, userID)
		}
	}
	return blogs, rrows.Err()
}

// findBlog loads one blog, returning sql.ErrNoRows when it does not exist.
func (s *Server) findBlog(ctx context.Context, id string) (Blog, error) {
	blogs, err := s.queryBlogs(ctx, "WHERE id = ?", id)
	if err != nil {
		return Blog{}, err
	}
	if len(blogs) == 0 {
		return Blog{}, sql.ErrNoRows
	}
	return blogs[0], nil
}

func (s *Server) listBlogs(c *fiber.Ctx) error {
	ctx, cancel := s.dbCtx(c)
	defer cancel()

	blogs, err := s.queryBlogs(ctx, "")
	if err != nil {
		return internalError(c, "find blogs", err)
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

	blog, err := s.findBlog(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
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
		ID:        store.NewID(),
		Title:     *in.Title,
		Body:      *in.Body,
		Author:    user.Name,
		CreatedAt: time.Now().UTC(),
	}

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO blogs (id, title, body, author, created_at) VALUES (?, ?, ?, ?, ?)",
		blog.ID, blog.Title, blog.Body, blog.Author, blog.CreatedAt.Format(store.TimeFormat),
	)
	if err != nil {
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

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	// COALESCE keeps a column unchanged when its field was not sent.
	res, err := s.db.ExecContext(ctx,
		"UPDATE blogs SET title = COALESCE(?, title), body = COALESCE(?, body) WHERE id = ?",
		in.Title, in.Body, id,
	)
	if err != nil {
		return internalError(c, "update blog", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fail(c, fiber.StatusNotFound, "Blog not found")
	}

	blog, err := s.findBlog(ctx, id)
	if err != nil {
		return internalError(c, "find blog", err)
	}
	return c.JSON(toBlogResponse(blog, viewerID(c)))
}

// --- Interactions ---

func (s *Server) like(c *fiber.Ctx) error    { return s.toggleVote(c, "like") }
func (s *Server) dislike(c *fiber.Ctx) error { return s.toggleVote(c, "dislike") }

// toggleVote removes the caller's vote of this kind if present; otherwise it
// records it, replacing any opposite vote. A reader has at most one row per
// blog, so switching sides is a single upsert, and the transaction keeps
// concurrent requests from interleaving the check and the write.
func (s *Server) toggleVote(c *fiber.Ctx, kind string) error {
	blogID, ok := parseID(c)
	if !ok {
		return fail(c, fiber.StatusBadRequest, "Invalid blog id")
	}
	user, _ := currentUser(c)

	ctx, cancel := s.dbCtx(c)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return internalError(c, "begin vote", err)
	}
	defer tx.Rollback()

	var found string
	err = tx.QueryRowContext(ctx, "SELECT id FROM blogs WHERE id = ?", blogID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(c, fiber.StatusNotFound, "Blog not found")
	}
	if err != nil {
		return internalError(c, "find blog", err)
	}

	res, err := tx.ExecContext(ctx,
		"DELETE FROM reactions WHERE blog_id = ? AND user_id = ? AND kind = ?", blogID, user.ID, kind)
	if err != nil {
		return internalError(c, "remove "+kind, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO reactions (blog_id, user_id, kind) VALUES (?, ?, ?)
			 ON CONFLICT (blog_id, user_id) DO UPDATE SET kind = excluded.kind`,
			blogID, user.ID, kind)
		if err != nil {
			return internalError(c, "add "+kind, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internalError(c, "commit vote", err)
	}

	blog, err := s.findBlog(ctx, blogID)
	if err != nil {
		return internalError(c, "find blog", err)
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

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, blog_id, user_id, user_name, content, created_at FROM comments WHERE blog_id = ? ORDER BY created_at ASC",
		blogID,
	)
	if err != nil {
		return internalError(c, "find comments", err)
	}
	defer rows.Close()

	comments := []Comment{}
	for rows.Next() {
		var cm Comment
		var created string
		if err := rows.Scan(&cm.ID, &cm.BlogID, &cm.UserID, &cm.UserName, &cm.Content, &created); err != nil {
			return internalError(c, "decode comments", err)
		}
		cm.CreatedAt = store.ParseTime(created)
		comments = append(comments, cm)
	}
	if err := rows.Err(); err != nil {
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

	var found string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM blogs WHERE id = ?", blogID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(c, fiber.StatusNotFound, "Blog not found")
	}
	if err != nil {
		return internalError(c, "find blog", err)
	}

	comment := Comment{
		ID:        store.NewID(),
		BlogID:    blogID,
		UserID:    user.ID,
		UserName:  user.Name,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO comments (id, blog_id, user_id, user_name, content, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		comment.ID, comment.BlogID, comment.UserID, comment.UserName, comment.Content, comment.CreatedAt.Format(store.TimeFormat),
	)
	if err != nil {
		return internalError(c, "insert comment", err)
	}
	return c.Status(fiber.StatusCreated).JSON(comment)
}
