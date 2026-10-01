package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var testSecret = []byte("test-secret-that-is-at-least-32-bytes-long")

// newTestServer builds the app without a database. Every case below must be
// rejected before a handler touches a collection.
func newTestServer() *Server {
	return &Server{cfg: Config{JWTSecret: testSecret, AllowedOrigins: []string{"http://localhost:5173"}}}
}

func tokenFor(t *testing.T, secret []byte, method jwt.SigningMethod, claims jwt.Claims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(method, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func validClaims(role string) Claims {
	return Claims{
		UserID: primitive.NewObjectID().Hex(),
		Role:   role,
		Name:   "Test",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func do(t *testing.T, s *Server, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := newApp(s).Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestHealthCheck(t *testing.T) {
	status, _ := do(t, newTestServer(), http.MethodGet, "/", "", "")
	if status != http.StatusOK {
		t.Fatalf("got %d, want 200", status)
	}
}

func TestAuthRequired(t *testing.T) {
	s := newTestServer()
	expired := validClaims(roleUser)
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	noExpiry := validClaims(roleUser)
	noExpiry.ExpiresAt = nil
	badUserID := validClaims(roleUser)
	badUserID.UserID = "not-an-id"

	cases := map[string]string{
		"missing token":      "",
		"garbage token":      "abc.def.ghi",
		"wrong secret":       tokenFor(t, []byte("secret"), jwt.SigningMethodHS256, validClaims(roleAdmin)),
		"wrong algorithm":    tokenFor(t, testSecret, jwt.SigningMethodHS512, validClaims(roleAdmin)),
		"expired":            tokenFor(t, testSecret, jwt.SigningMethodHS256, expired),
		"no expiry":          tokenFor(t, testSecret, jwt.SigningMethodHS256, noExpiry),
		"invalid user id":    tokenFor(t, testSecret, jwt.SigningMethodHS256, badUserID),
		"missing role claim": tokenFor(t, testSecret, jwt.SigningMethodHS256, jwt.MapClaims{"user_id": primitive.NewObjectID().Hex(), "exp": time.Now().Add(time.Hour).Unix()}),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			status, body := do(t, s, http.MethodPost, "/blogs/"+primitive.NewObjectID().Hex()+"/like", "", token)
			if status != http.StatusUnauthorized {
				t.Fatalf("got %d, want 401", status)
			}
			if body["error"] == nil {
				t.Fatalf("expected JSON error body, got %v", body)
			}
		})
	}
}

func TestAdminOnly(t *testing.T) {
	s := newTestServer()
	userToken := tokenFor(t, testSecret, jwt.SigningMethodHS256, validClaims(roleUser))
	id := primitive.NewObjectID().Hex()

	if status, _ := do(t, s, http.MethodPost, "/blogs", `{"title":"t","body":"b"}`, userToken); status != http.StatusForbidden {
		t.Fatalf("create: got %d, want 403", status)
	}
	if status, _ := do(t, s, http.MethodPut, "/blogs/"+id, `{"title":"t"}`, userToken); status != http.StatusForbidden {
		t.Fatalf("update: got %d, want 403", status)
	}
}

func TestInvalidInputIsRejected(t *testing.T) {
	s := newTestServer()
	user := tokenFor(t, testSecret, jwt.SigningMethodHS256, validClaims(roleUser))
	admin := tokenFor(t, testSecret, jwt.SigningMethodHS256, validClaims(roleAdmin))
	id := primitive.NewObjectID().Hex()

	cases := []struct {
		name, method, path, body, token string
	}{
		{"get blog bad id", http.MethodGet, "/blogs/garbage", "", ""},
		{"list comments bad id", http.MethodGet, "/blogs/garbage/comments", "", ""},
		{"like bad id", http.MethodPost, "/blogs/garbage/like", "", user},
		{"dislike bad id", http.MethodPost, "/blogs/garbage/dislike", "", user},
		{"comment bad id", http.MethodPost, "/blogs/garbage/comments", `{"content":"hi"}`, user},
		{"empty comment", http.MethodPost, "/blogs/" + id + "/comments", `{"content":"   "}`, user},
		{"update bad id", http.MethodPut, "/blogs/garbage", `{"title":"t"}`, admin},
		{"update nothing", http.MethodPut, "/blogs/" + id, `{}`, admin},
		{"update empty title", http.MethodPut, "/blogs/" + id, `{"title":"  "}`, admin},
		{"create missing body", http.MethodPost, "/blogs", `{"title":"t"}`, admin},
		{"signup empty email", http.MethodPost, "/signup", `{"name":"a","email":"","password":"password123"}`, ""},
		{"signup bad email", http.MethodPost, "/signup", `{"name":"a","email":"nope","password":"password123"}`, ""},
		{"signup missing name", http.MethodPost, "/signup", `{"email":"a@b.co","password":"password123"}`, ""},
		{"signup short password", http.MethodPost, "/signup", `{"name":"a","email":"a@b.co","password":"short"}`, ""},
		{"signup long password", http.MethodPost, "/signup", `{"name":"a","email":"a@b.co","password":"` + strings.Repeat("x", 73) + `"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := do(t, s, tc.method, tc.path, tc.body, tc.token)
			if status != http.StatusBadRequest {
				t.Fatalf("got %d (%v), want 400", status, body)
			}
		})
	}
}

func TestUnknownRouteReturnsJSON(t *testing.T) {
	status, body := do(t, newTestServer(), http.MethodGet, "/nope", "", "")
	if status != http.StatusNotFound || body["error"] == nil {
		t.Fatalf("got %d %v, want 404 with JSON error", status, body)
	}
}

func TestLoadConfigRequiresSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error when JWT_SECRET is empty")
	}
	t.Setenv("JWT_SECRET", "too-short")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error when JWT_SECRET is short")
	}
	t.Setenv("JWT_SECRET", string(testSecret))
	t.Setenv("ADMIN_EMAIL", "  Admin@Example.com ")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminEmail != "admin@example.com" {
		t.Fatalf("admin email not normalized: %q", cfg.AdminEmail)
	}
}

func TestParseOrigins(t *testing.T) {
	got := parseOrigins(" https://a.com/ , http://b.com,, ")
	want := []string{"https://a.com", "http://b.com"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestToBlogResponse(t *testing.T) {
	me, other := primitive.NewObjectID(), primitive.NewObjectID()
	b := Blog{ID: primitive.NewObjectID(), Likes: []primitive.ObjectID{me, other}, Dislikes: []primitive.ObjectID{}}

	anon := toBlogResponse(b, nil)
	if anon.LikeCount != 2 || anon.DislikeCount != 0 || anon.LikedByMe {
		t.Fatalf("unexpected anonymous response: %+v", anon)
	}
	mine := toBlogResponse(b, &me)
	if !mine.LikedByMe || mine.DislikedByMe {
		t.Fatalf("unexpected viewer response: %+v", mine)
	}
}
