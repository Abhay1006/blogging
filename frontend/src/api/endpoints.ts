import { api } from './client';
import type { Blog, Comment, LoginResponse } from '../types';

export interface BlogInput {
  title: string;
  body: string;
}

export interface Credentials {
  email: string;
  password: string;
}

export interface SignupInput extends Credentials {
  name: string;
}

export const fetchBlogs = async () => (await api.get<Blog[]>('/blogs')).data;

export const fetchBlog = async (id: string) => (await api.get<Blog>(`/blogs/${id}`)).data;

export const createBlog = async (input: BlogInput) => (await api.post<Blog>('/blogs', input)).data;

export const updateBlog = async (id: string, input: BlogInput) => (await api.put<Blog>(`/blogs/${id}`, input)).data;

export const vote = async (id: string, type: 'like' | 'dislike') => (await api.post<Blog>(`/blogs/${id}/${type}`)).data;

export const fetchComments = async (id: string) => (await api.get<Comment[]>(`/blogs/${id}/comments`)).data;

export const postComment = async (id: string, content: string) =>
  (await api.post<Comment>(`/blogs/${id}/comments`, { content })).data;

export const login = async (credentials: Credentials) => (await api.post<LoginResponse>('/login', credentials)).data;

export const signup = async (input: SignupInput) => {
  await api.post('/signup', input);
};
