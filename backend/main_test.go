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

	"my-blog/backend/internal/store"
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
		UserID: store.NewID(),
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
	status, _ := do(t, newTestServer(), http.MethodGet, "/api/health", "", "")
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
		"missing role claim": tokenFor(t, testSecret, jwt.SigningMethodHS256, jwt.MapClaims{"user_id": store.NewID(), "exp": time.Now().Add(time.Hour).Unix()}),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			status, body := do(t, s, http.MethodPost, "/api/blogs/"+store.NewID()+"/like", "", token)
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
	id := store.NewID()

	if status, _ := do(t, s, http.MethodPost, "/api/blogs", `{"title":"t","body":"b"}`, userToken); status != http.StatusForbidden {
		t.Fatalf("create: got %d, want 403", status)
	}
	if status, _ := do(t, s, http.MethodPut, "/api/blogs/"+id, `{"title":"t"}`, userToken); status != http.StatusForbidden {
		t.Fatalf("update: got %d, want 403", status)
	}
}

func TestInvalidInputIsRejected(t *testing.T) {
	s := newTestServer()
	user := tokenFor(t, testSecret, jwt.SigningMethodHS256, validClaims(roleUser))
	admin := tokenFor(t, testSecret, jwt.SigningMethodHS256, validClaims(roleAdmin))
	id := store.NewID()

	cases := []struct {
		name, method, path, body, token string
	}{
		{"get blog bad id", http.MethodGet, "/api/blogs/garbage", "", ""},
		{"list comments bad id", http.MethodGet, "/api/blogs/garbage/comments", "", ""},
		{"like bad id", http.MethodPost, "/api/blogs/garbage/like", "", user},
		{"dislike bad id", http.MethodPost, "/api/blogs/garbage/dislike", "", user},
		{"comment bad id", http.MethodPost, "/api/blogs/garbage/comments", `{"content":"hi"}`, user},
		{"empty comment", http.MethodPost, "/api/blogs/" + id + "/comments", `{"content":"   "}`, user},
		{"update bad id", http.MethodPut, "/api/blogs/garbage", `{"title":"t"}`, admin},
		{"update nothing", http.MethodPut, "/api/blogs/" + id, `{}`, admin},
		{"update empty title", http.MethodPut, "/api/blogs/" + id, `{"title":"  "}`, admin},
		{"create missing body", http.MethodPost, "/api/blogs", `{"title":"t"}`, admin},
		{"signup empty email", http.MethodPost, "/api/signup", `{"name":"a","email":"","password":"password123"}`, ""},
		{"signup bad email", http.MethodPost, "/api/signup", `{"name":"a","email":"nope","password":"password123"}`, ""},
		{"signup missing name", http.MethodPost, "/api/signup", `{"email":"a@b.co","password":"password123"}`, ""},
		{"signup short password", http.MethodPost, "/api/signup", `{"name":"a","email":"a@b.co","password":"short"}`, ""},
		{"signup long password", http.MethodPost, "/api/signup", `{"name":"a","email":"a@b.co","password":"` + strings.Repeat("x", 73) + `"}`, ""},
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
	status, body := do(t, newTestServer(), http.MethodGet, "/api/nope", "", "")
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
	me, other := store.NewID(), store.NewID()
	b := Blog{ID: store.NewID(), Likes: []string{me, other}}

	anon := toBlogResponse(b, nil)
	if anon.LikeCount != 2 || anon.DislikeCount != 0 || anon.LikedByMe {
		t.Fatalf("unexpected anonymous response: %+v", anon)
	}
	mine := toBlogResponse(b, &me)
	if !mine.LikedByMe || mine.DislikedByMe {
		t.Fatalf("unexpected viewer response: %+v", mine)
	}
}

// TestEndToEnd drives the API against a real SQLite file.
func TestEndToEnd(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Server{cfg: Config{JWTSecret: testSecret, AdminEmail: "admin@example.com"}, db: db}

	signup := func(email string) {
		body := `{"name":"N","email":"` + email + `","password":"password123"}`
		if status, out := do(t, s, http.MethodPost, "/api/signup", body, ""); status != http.StatusCreated {
			t.Fatalf("signup %s: got %d %v", email, status, out)
		}
	}
	login := func(email string) string {
		status, out := do(t, s, http.MethodPost, "/api/login", `{"email":"`+email+`","password":"password123"}`, "")
		if status != http.StatusOK {
			t.Fatalf("login %s: got %d %v", email, status, out)
		}
		return out["token"].(string)
	}

	signup("Admin@Example.com")
	signup("reader@example.com")
	if status, _ := do(t, s, http.MethodPost, "/api/signup", `{"name":"N","email":"READER@example.com","password":"password123"}`, ""); status != http.StatusConflict {
		t.Fatalf("duplicate signup: got %d, want 409", status)
	}
	admin, reader := login("admin@example.com"), login("Reader@Example.com")

	status, blog := do(t, s, http.MethodPost, "/api/blogs", `{"title":" Hello ","body":"World"}`, admin)
	if status != http.StatusCreated || blog["title"] != "Hello" {
		t.Fatalf("create: got %d %v", status, blog)
	}
	id := blog["id"].(string)

	vote := func(kind string) map[string]any {
		status, out := do(t, s, http.MethodPost, "/api/blogs/"+id+"/"+kind, "", reader)
		if status != http.StatusOK {
			t.Fatalf("%s: got %d %v", kind, status, out)
		}
		return out
	}
	if out := vote("like"); out["like_count"] != 1.0 || out["liked_by_me"] != true {
		t.Fatalf("like: %v", out)
	}
	if out := vote("dislike"); out["like_count"] != 0.0 || out["dislike_count"] != 1.0 {
		t.Fatalf("switch to dislike: %v", out)
	}
	if out := vote("dislike"); out["dislike_count"] != 0.0 || out["disliked_by_me"] != false {
		t.Fatalf("toggle off: %v", out)
	}

	if status, _ := do(t, s, http.MethodPut, "/api/blogs/"+id, `{"body":"Edited"}`, admin); status != http.StatusOK {
		t.Fatalf("update: got %d", status)
	}
	if _, got := do(t, s, http.MethodGet, "/api/blogs/"+id, "", ""); got["body"] != "Edited" || got["title"] != "Hello" {
		t.Fatalf("partial update: %v", got)
	}
	if status, _ := do(t, s, http.MethodPost, "/api/blogs/"+id+"/comments", `{"content":"nice"}`, reader); status != http.StatusCreated {
		t.Fatalf("comment: got %d", status)
	}
	if status, _ := do(t, s, http.MethodGet, "/api/blogs/"+store.NewID(), "", ""); status != http.StatusNotFound {
		t.Fatalf("missing blog: got %d, want 404", status)
	}
}
