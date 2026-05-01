// Typed API client for OniGram — automatically attaches JWT from localStorage.

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
  if (!res.ok) {
    throw new APIError(res.status, data?.message ?? 'Request failed')
  }
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
    request<{ token: string; user: import('./types').User }>('POST', '/auth/register', { username, email, password }),
  login: (email: string, password: string) =>
    request<{ token: string; user: import('./types').User }>('POST', '/auth/login', { email, password }),
  logout: () => request<void>('POST', '/auth/logout'),
  me: () => request<import('./types').User>('GET', '/auth/me'),
}

// Feed & Posts
export const posts = {
  feed: (page = 1) => request<{ posts: import('./types').Post[]; page: number }>('GET', `/feed?page=${page}`),
  get: (id: number) => request<import('./types').Post>('GET', `/posts/${id}`),
  create: (form: FormData) => request<import('./types').Post>('POST', '/posts', undefined, form),
  delete: (id: number) => request<void>('DELETE', `/posts/${id}`),
  like: (id: number) => request<{ like_count: number }>('POST', `/posts/${id}/like`),
  unlike: (id: number) => request<{ like_count: number }>('DELETE', `/posts/${id}/like`),
  comments: (id: number) => request<{ comments: import('./types').Comment[] }>('GET', `/posts/${id}/comments`),
  addComment: (id: number, body: string) =>
    request<import('./types').Comment>('POST', `/posts/${id}/comments`, { body }),
}

// Users
export const users = {
  get: (username: string) => request<import('./types').User>('GET', `/users/${username}`),
  search: (q: string) => request<{ users: import('./types').User[] }>('GET', `/users/search?q=${encodeURIComponent(q)}`),
  updateProfile: (data: { username?: string; bio?: string }) => request<import('./types').User>('PUT', '/users/me', data),
  updateAvatar: (form: FormData) => request<{ avatar_path: string }>('POST', '/users/me/avatar', undefined, form),
  follow: (username: string) => request<void>('POST', `/users/${username}/follow`),
  unfollow: (username: string) => request<void>('DELETE', `/users/${username}/follow`),
  posts: (username: string) => request<{ posts: import('./types').Post[] }>('GET', `/users/${username}/posts`),
  followers: (username: string) => request<{ users: import('./types').User[] }>('GET', `/users/${username}/followers`),
  following: (username: string) => request<{ users: import('./types').User[] }>('GET', `/users/${username}/following`),
}

// Notifications
export const notifications = {
  list: () => request<{ notifications: import('./types').Notification[]; unread_count: number }>('GET', '/notifications'),
  markRead: (id: number) => request<void>('POST', `/notifications/${id}/read`),
  markAllRead: () => request<void>('POST', '/notifications/read-all'),
}
