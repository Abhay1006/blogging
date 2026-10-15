package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name     string             `bson:"name" json:"name"`
	Email    string             `bson:"email" json:"email"`
	Password string             `bson:"password" json:"-"`
	Role     string             `bson:"role" json:"role"` // "admin" or "user"
}

type Blog struct {
	ID        primitive.ObjectID   `bson:"_id,omitempty"`
	Title     string               `bson:"title"`
	Body      string               `bson:"body"`
	Author    string               `bson:"author"`
	Likes     []primitive.ObjectID `bson:"likes"`
	Dislikes  []primitive.ObjectID `bson:"dislikes"`
	CreatedAt time.Time            `bson:"created_at"`
}

type Comment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	BlogID    primitive.ObjectID `bson:"blog_id" json:"blog_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	UserName  string             `bson:"user_name" json:"user_name"`
	Content   string             `bson:"content" json:"content"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// BlogResponse is the public shape of a blog. It exposes vote counts and the
// viewer's own vote rather than the full lists of user IDs who voted.
type BlogResponse struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	Author       string    `json:"author"`
	LikeCount    int       `json:"like_count"`
	DislikeCount int       `json:"dislike_count"`
	LikedByMe    bool      `json:"liked_by_me"`
	DislikedByMe bool      `json:"disliked_by_me"`
	CreatedAt    time.Time `json:"created_at"`
}

// toBlogResponse converts a stored blog for the given viewer (nil when anonymous).
func toBlogResponse(b Blog, viewer *primitive.ObjectID) BlogResponse {
	resp := BlogResponse{
		ID:           b.ID.Hex(),
		Title:        b.Title,
		Body:         b.Body,
		Author:       b.Author,
		LikeCount:    len(b.Likes),
		DislikeCount: len(b.Dislikes),
		CreatedAt:    b.CreatedAt,
	}
	if viewer != nil {
		resp.LikedByMe = containsID(b.Likes, *viewer)
		resp.DislikedByMe = containsID(b.Dislikes, *viewer)
	}
	return resp
}

func containsID(ids []primitive.ObjectID, id primitive.ObjectID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
