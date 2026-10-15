// Command migrate copies the blog's data out of MongoDB Atlas and into the
// local SQLite database. It is meant to be run once, from a machine that can
// reach Atlas; the resulting file is then copied to the server.
//
// Re-running it is safe: every write is an upsert keyed on the original
// document id, so existing rows are refreshed rather than duplicated.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"my-blog/backend/internal/store"
)

type mongoUser struct {
	ID       primitive.ObjectID `bson:"_id"`
	Name     string             `bson:"name"`
	Email    string             `bson:"email"`
	Password string             `bson:"password"`
	Role     string             `bson:"role"`
}

type mongoBlog struct {
	ID        primitive.ObjectID   `bson:"_id"`
	Title     string               `bson:"title"`
	Body      string               `bson:"body"`
	Author    string               `bson:"author"`
	Likes     []primitive.ObjectID `bson:"likes"`
	Dislikes  []primitive.ObjectID `bson:"dislikes"`
	CreatedAt time.Time            `bson:"created_at"`
}

type mongoComment struct {
	ID        primitive.ObjectID `bson:"_id"`
	BlogID    primitive.ObjectID `bson:"blog_id"`
	UserID    primitive.ObjectID `bson:"user_id"`
	UserName  string             `bson:"user_name"`
	Content   string             `bson:"content"`
	CreatedAt time.Time          `bson:"created_at"`
}

func main() {
	_ = godotenv.Load()

	var (
		uri    = flag.String("mongo-uri", os.Getenv("MONGO_URI"), "MongoDB connection string")
		dbName = flag.String("mongo-db", "lumina_db", "MongoDB database name")
		out    = flag.String("out", "lumina.db", "path of the SQLite file to write")
	)
	flag.Parse()

	if *uri == "" {
		log.Fatal("no MongoDB URI: pass -mongo-uri or set MONGO_URI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*uri))
	if err != nil {
		log.Fatalf("connecting to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("pinging MongoDB: %v", err)
	}
	log.Printf("Connected to MongoDB, reading database %q", *dbName)

	sqlite, err := store.Open(*out)
	if err != nil {
		log.Fatalf("opening %s: %v", *out, err)
	}
	defer sqlite.Close()

	src := client.Database(*dbName)

	// A single transaction: either the whole snapshot lands or none of it does.
	tx, err := sqlite.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("beginning transaction: %v", err)
	}
	defer tx.Rollback()

	users, err := copyUsers(ctx, src, tx)
	if err != nil {
		log.Fatalf("copying users: %v", err)
	}

	blogs, reactions, err := copyBlogs(ctx, src, tx)
	if err != nil {
		log.Fatalf("copying blogs: %v", err)
	}

	comments, err := copyComments(ctx, src, tx)
	if err != nil {
		log.Fatalf("copying comments: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("committing: %v", err)
	}

	log.Printf("Migration complete -> %s", *out)
	log.Printf("  users:     %d", users)
	log.Printf("  blogs:     %d", blogs)
	log.Printf("  reactions: %d", reactions)
	log.Printf("  comments:  %d", comments)
}

func copyUsers(ctx context.Context, src *mongo.Database, tx *sql.Tx) (int, error) {
	cursor, err := src.Collection("users").Find(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	stmt, err := tx.PrepareContext(ctx,
		"INSERT OR REPLACE INTO users (id, name, email, password, role) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for cursor.Next(ctx) {
		var u mongoUser
		if err := cursor.Decode(&u); err != nil {
			return count, err
		}
		role := u.Role
		if role == "" {
			role = "user"
		}
		if _, err := stmt.ExecContext(ctx, u.ID.Hex(), u.Name, u.Email, u.Password, role); err != nil {
			return count, err
		}
		count++
	}
	return count, cursor.Err()
}

func copyBlogs(ctx context.Context, src *mongo.Database, tx *sql.Tx) (blogs, reactions int, err error) {
	cursor, err := src.Collection("blogs").Find(ctx, bson.M{})
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)

	blogStmt, err := tx.PrepareContext(ctx,
		"INSERT OR REPLACE INTO blogs (id, title, body, author, created_at) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return 0, 0, err
	}
	defer blogStmt.Close()

	// OR IGNORE, not OR REPLACE: if a document somehow carried the same reader
	// in both arrays, the first one seen (like) wins rather than the last.
	reactStmt, err := tx.PrepareContext(ctx,
		"INSERT OR IGNORE INTO reactions (blog_id, user_id, kind) VALUES (?, ?, ?)")
	if err != nil {
		return 0, 0, err
	}
	defer reactStmt.Close()

	for cursor.Next(ctx) {
		var b mongoBlog
		if err := cursor.Decode(&b); err != nil {
			return blogs, reactions, err
		}

		created := b.CreatedAt
		if created.IsZero() {
			// Fall back to the timestamp embedded in the ObjectID.
			created = b.ID.Timestamp()
		}

		id := b.ID.Hex()
		if _, err := blogStmt.ExecContext(ctx, id, b.Title, b.Body, b.Author,
			created.UTC().Format(store.TimeFormat)); err != nil {
			return blogs, reactions, err
		}
		blogs++

		for _, uid := range b.Likes {
			res, err := reactStmt.ExecContext(ctx, id, uid.Hex(), "like")
			if err != nil {
				return blogs, reactions, err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				reactions++
			}
		}
		for _, uid := range b.Dislikes {
			res, err := reactStmt.ExecContext(ctx, id, uid.Hex(), "dislike")
			if err != nil {
				return blogs, reactions, err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				reactions++
			}
		}
	}

	return blogs, reactions, cursor.Err()
}

func copyComments(ctx context.Context, src *mongo.Database, tx *sql.Tx) (int, error) {
	cursor, err := src.Collection("comments").Find(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	stmt, err := tx.PrepareContext(ctx,
		"INSERT OR REPLACE INTO comments (id, blog_id, user_id, user_name, content, created_at) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	skipped := 0
	for cursor.Next(ctx) {
		var cm mongoComment
		if err := cursor.Decode(&cm); err != nil {
			return count, err
		}

		created := cm.CreatedAt
		if created.IsZero() {
			created = cm.ID.Timestamp()
		}

		_, err := stmt.ExecContext(ctx, cm.ID.Hex(), cm.BlogID.Hex(), cm.UserID.Hex(),
			cm.UserName, cm.Content, created.UTC().Format(store.TimeFormat))
		if err != nil {
			// A comment whose blog was deleted violates the foreign key. That is
			// an orphan worth reporting, not a reason to abort the migration.
			log.Printf("  skipping comment %s (blog %s): %v", cm.ID.Hex(), cm.BlogID.Hex(), err)
			skipped++
			continue
		}
		count++
	}
	if skipped > 0 {
		log.Printf("  skipped %d orphaned comment(s)", skipped)
	}
	return count, cursor.Err()
}
