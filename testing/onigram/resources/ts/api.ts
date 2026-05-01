import type { User, Post, Comment, Story, Notification, Conversation, Message } from './types.ts'

const BASE = '/api'

function token(): string {
  return localStorage.getItem('og_token') ?? ''
}

async function request<T>(method: string, path: string, body?: unknown, form?: FormData): Promise<T> {
  const headers: Record<string, string> = {}
  const tok = token()
  if (tok) headers['Authorization'] = `Bearer ${tok}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  const res = await fetch(BASE + path, {
    method,
    headers,
    body: form ?? (body !== undefined ? JSON.stringify(body) : undefined),
  })

  if (res.status === 204) return undefined as T

  const data = await res.json()
  if (!res.ok) throw new APIError(res.status, data?.error ?? data?.message ?? 'Request failed')
  return data as T
}

export class APIError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

// Auth
export const auth = {
  register: (username: string, email: string, password: string) =>
    request<{ token: string; user: User }>('POST', '/auth/register', { username, email, password }),
  login: (email: string, password: string) =>
    request<{ token: string; user: User }>('POST', '/auth/login', { email, password }),
  logout: () => request<void>('POST', '/auth/logout'),
  me: () => request<User>('GET', '/auth/me'),
}

// Feed & Posts
export const posts = {
  feed: (page = 1) => request<{ posts: Post[]; page: number }>('GET', `/feed?page=${page}`),
  explore: (page = 1) => request<{ posts: Post[]; page: number }>('GET', `/explore?page=${page}`),
  get: (id: number) => request<Post>('GET', `/posts/${id}`),
  create: (form: FormData) => request<Post>('POST', '/posts', undefined, form),
  delete: (id: number) => request<void>('DELETE', `/posts/${id}`),
  like: (id: number) => request<{ like_count: number }>('POST', `/posts/${id}/like`),
  unlike: (id: number) => request<{ like_count: number }>('DELETE', `/posts/${id}/like`),
  bookmark: (id: number) => request<{ bookmarked: boolean }>('POST', `/posts/${id}/bookmark`),
  unbookmark: (id: number) => request<{ bookmarked: boolean }>('DELETE', `/posts/${id}/bookmark`),
  comments: (id: number) => request<{ comments: Comment[] }>('GET', `/posts/${id}/comments`),
  addComment: (id: number, body: string) => request<Comment>('POST', `/posts/${id}/comments`, { body }),
}

// Users
export const users = {
  get: (username: string) => request<User>('GET', `/users/${username}`),
  search: (q: string) => request<{ users: User[] }>('GET', `/users/search?q=${encodeURIComponent(q)}`),
  suggestions: () => request<{ users: User[] }>('GET', '/users/suggestions'),
  updateProfile: (data: { username?: string; bio?: string; website?: string }) =>
    request<User>('PUT', '/users/me', data),
  updateAvatar: (form: FormData) => request<{ avatar_path: string }>('POST', '/users/me/avatar', undefined, form),
  follow: (username: string) => request<void>('POST', `/users/${username}/follow`),
  unfollow: (username: string) => request<void>('DELETE', `/users/${username}/follow`),
  posts: (username: string) => request<{ posts: Post[] }>('GET', `/users/${username}/posts`),
  followers: (username: string) => request<{ users: User[] }>('GET', `/users/${username}/followers`),
  following: (username: string) => request<{ users: User[] }>('GET', `/users/${username}/following`),
}

// Comments
export const comments = {
  delete: (id: number) => request<void>('DELETE', `/comments/${id}`),
  like: (id: number) => request<{ like_count: number }>('POST', `/comments/${id}/like`),
  unlike: (id: number) => request<{ like_count: number }>('DELETE', `/comments/${id}/like`),
}

// Stories
export const stories = {
  feed: () => request<{ stories: Story[] }>('GET', '/stories/feed'),
  create: (form: FormData) => request<Story>('POST', '/stories', undefined, form),
  markViewed: (id: number) => request<void>('POST', `/stories/${id}/view`),
  delete: (id: number) => request<void>('DELETE', `/stories/${id}`),
}

// Messages
export const messages = {
  inbox: () => request<{ conversations: Conversation[] }>('GET', '/messages'),
  thread: (username: string) =>
    request<{ conversation: Conversation; messages: Message[]; other_user: User }>('GET', `/messages/${username}`),
  send: (username: string, body: string) => request<Message>('POST', `/messages/${username}`, { body }),
}

// Bookmarks
export const bookmarks = {
  list: () => request<{ posts: Post[] }>('GET', '/bookmarks'),
}

// Notifications
export const notifications = {
  list: () => request<{ notifications: Notification[]; unread_count: number }>('GET', '/notifications'),
  markRead: (id: number) => request<void>('POST', `/notifications/${id}/read`),
  markAllRead: () => request<void>('POST', '/notifications/read-all'),
}
