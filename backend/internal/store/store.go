// Package store owns the local SQLite database: how it is opened, what the
// schema looks like, and how identifiers and timestamps are represented.
// Both the API server and the one-off migration tool depend on it so the two
// can never drift apart.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// TimeFormat is how timestamps are stored. Text in RFC3339 sorts
// lexicographically in the same order it sorts chronologically, which lets
// ORDER BY created_at work directly on the stored column.
const TimeFormat = time.RFC3339Nano

const Schema = `
CREATE TABLE IF NOT EXISTS users (
	id       TEXT PRIMARY KEY,
	name     TEXT NOT NULL,
	email    TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	role     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS blogs (
	id         TEXT PRIMARY KEY,
	title      TEXT NOT NULL,
	body       TEXT NOT NULL,
	author     TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
	id         TEXT PRIMARY KEY,
	blog_id    TEXT NOT NULL REFERENCES blogs(id) ON DELETE CASCADE,
	user_id    TEXT NOT NULL,
	user_name  TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at TEXT NOT NULL
);

-- One row per (blog, user). The composite primary key is what makes a like
-- and a dislike from the same reader mutually exclusive -- the rule the old
-- $pull/$addToSet pair had to enforce by hand.
CREATE TABLE IF NOT EXISTS reactions (
	blog_id TEXT NOT NULL REFERENCES blogs(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL,
	kind    TEXT NOT NULL CHECK (kind IN ('like', 'dislike')),
	PRIMARY KEY (blog_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_blogs_created_at ON blogs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_comments_blog_id ON comments(blog_id, created_at);
CREATE INDEX IF NOT EXISTS idx_reactions_blog_id ON reactions(blog_id);
`

// Open opens, creating if necessary, the database at path and applies the
// schema. WAL mode keeps readers from queueing behind the occasional write,
// which matters on the slow flash storage of a phone.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		path,
	)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	if _, err := conn.Exec(Schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}

	return conn, nil
}

// NewID mints a 24-character hex identifier with the same shape as the
// MongoDB ObjectIDs this app used to store, so links to migrated posts keep
// working and the frontend's string ids need no changes.
func NewID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("could not generate id: %v", err)
	}
	return hex.EncodeToString(b)
}

// ParseTime reads a stored timestamp, tolerating the plain RFC3339 that other
// tools may have written rather than discarding the row.
func ParseTime(s string) time.Time {
	if t, err := time.Parse(TimeFormat, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	log.Printf("unparseable timestamp %q", s)
	return time.Time{}
}
