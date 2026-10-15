package main

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const (
	roleAdmin = "admin"
	roleUser  = "user"

	tokenTTL     = 72 * time.Hour
	localAuthKey = "auth"
)

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

// AuthUser is the authenticated caller, stored in the request locals.
type AuthUser struct {
	ID   string
	Role string
	Name string
}

func (s *Server) signToken(u User) (string, error) {
	claims := Claims{
		UserID: u.ID,
		Role:   u.Role,
		Name:   u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.cfg.JWTSecret)
}

// parseToken validates a bearer token and returns the caller it identifies.
func (s *Server) parseToken(header string) (*AuthUser, error) {
	tokenString, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || tokenString == "" {
		return nil, errors.New("missing bearer token")
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return s.cfg.JWTSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}

	if !isValidID(claims.UserID) {
		return nil, errors.New("invalid user id claim")
	}
	if claims.Role != roleAdmin && claims.Role != roleUser {
		return nil, errors.New("invalid role claim")
	}
	return &AuthUser{ID: claims.UserID, Role: claims.Role, Name: claims.Name}, nil
}

// authRequired rejects requests without a valid token.
func (s *Server) authRequired(c *fiber.Ctx) error {
	if c.Get(fiber.HeaderAuthorization) == "" {
		return fail(c, fiber.StatusUnauthorized, "Missing token")
	}
	user, err := s.parseToken(c.Get(fiber.HeaderAuthorization))
	if err != nil {
		return fail(c, fiber.StatusUnauthorized, "Invalid token")
	}
	c.Locals(localAuthKey, user)
	return c.Next()
}

// optionalAuth identifies the caller when a valid token is sent, and otherwise
// treats the request as anonymous. Used by public read endpoints.
func (s *Server) optionalAuth(c *fiber.Ctx) error {
	if user, err := s.parseToken(c.Get(fiber.HeaderAuthorization)); err == nil {
		c.Locals(localAuthKey, user)
	}
	return c.Next()
}

func adminOnly(c *fiber.Ctx) error {
	user, ok := currentUser(c)
	if !ok || user.Role != roleAdmin {
		return fail(c, fiber.StatusForbidden, "Admin access required")
	}
	return c.Next()
}

func currentUser(c *fiber.Ctx) (*AuthUser, bool) {
	user, ok := c.Locals(localAuthKey).(*AuthUser)
	return user, ok && user != nil
}

func viewerID(c *fiber.Ctx) *string {
	if user, ok := currentUser(c); ok {
		return &user.ID
	}
	return nil
}

// fail writes the API's standard error shape.
func fail(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

// errorHandler turns unhandled errors (including recovered panics and unknown
// routes) into the standard JSON error shape without leaking internals.
func errorHandler(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := "Internal server error"
	var fe *fiber.Error
	if errors.As(err, &fe) {
		status = fe.Code
		message = fe.Message
	}
	return fail(c, status, message)
}
