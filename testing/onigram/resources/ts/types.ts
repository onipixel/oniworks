// Auto-typed from OniGram API responses

export interface User {
  id: number;
  username: string;
  email?: string;
  bio: string;
  avatar_path: string;
  created_at: string;
  follower_count?: number;
  following_count?: number;
  is_following?: boolean;
}

export interface Post {
  id: number;
  user_id: number;
  image_path: string;
  caption: string;
  created_at: string;
  user?: User;
  like_count?: number;
  is_liked?: boolean;
}

export interface Comment {
  id: number;
  user_id: number;
  post_id: number;
  body: string;
  created_at: string;
  user?: User;
}

export interface Notification {
  id: number;
  user_id: number;
  actor_id: number;
  type: 'like' | 'comment' | 'follow';
  post_id?: number;
  read: boolean;
  created_at: string;
  actor?: User;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface PaginatedPosts {
  posts: Post[];
  page: number;
}

export interface NotificationsResponse {
  notifications: Notification[];
  unread_count: number;
}
