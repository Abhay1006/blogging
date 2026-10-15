package main

import "time"

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Role     string `json:"role"` // "admin" or "user"
}

// Blog is a stored post together with the IDs of the readers who reacted to it.
type Blog struct {
	ID        string
	Title     string
	Body      string
	Author    string
	Likes     []string
	Dislikes  []string
	CreatedAt time.Time
}

type Comment struct {
	ID        string    `json:"id"`
	BlogID    string    `json:"blog_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
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
func toBlogResponse(b Blog, viewer *string) BlogResponse {
	resp := BlogResponse{
		ID:           b.ID,
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

func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
