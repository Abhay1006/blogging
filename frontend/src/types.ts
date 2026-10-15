export interface User {
  id: string;
  name: string;
  email: string;
  role: 'admin' | 'user';
}

export interface Blog {
  id: string;
  title: string;
  body: string;
  author: string;
  like_count: number;
  dislike_count: number;
  liked_by_me: boolean;
  disliked_by_me: boolean;
  created_at: string;
}

export interface Comment {
  id: string;
  blog_id: string;
  user_id: string;
  user_name: string;
  content: string;
  created_at: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}
