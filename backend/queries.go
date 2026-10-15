package main

import "strings"

// reactionsFor collects likes and dislikes for the given blogs in a single
// query, so rendering the home page stays one round trip no matter how many
// posts it lists.
func reactionsFor(blogIDs []string) (likes, dislikes map[string][]string, err error) {
	likes = make(map[string][]string)
	dislikes = make(map[string][]string)

	if len(blogIDs) == 0 {
		return likes, dislikes, nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(blogIDs)), ",")
	query := "SELECT blog_id, user_id, kind FROM reactions WHERE blog_id IN (" + placeholders + ")"

	args := make([]any, len(blogIDs))
	for i, id := range blogIDs {
		args[i] = id
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var blogID, userID, kind string
		if err := rows.Scan(&blogID, &userID, &kind); err != nil {
			return nil, nil, err
		}
		if kind == "like" {
			likes[blogID] = append(likes[blogID], userID)
		} else {
			dislikes[blogID] = append(dislikes[blogID], userID)
		}
	}

	return likes, dislikes, rows.Err()
}

// orEmpty keeps the JSON contract stable: the frontend expects arrays, not null.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
