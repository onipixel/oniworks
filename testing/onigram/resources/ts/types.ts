export interface User {
  id: number
  username: string
  email?: string
  bio: string
  website?: string
  avatar_path: string
  created_at: string
  follower_count?: number
  following_count?: number
  is_following?: boolean
}

export interface Post {
  id: number
  user_id: number
  image_path: string
  caption: string
  created_at: string
  user?: User
  like_count?: number
  is_liked?: boolean
  is_bookmarked?: boolean
}

export interface Comment {
  id: number
  user_id: number
  post_id: number
  body: string
  created_at: string
  user?: User
  like_count?: number
  is_liked?: boolean
}

export interface Story {
  id: number
  user_id: number
  image_path: string
  expires_at: string
  created_at: string
  user?: User
  viewed: boolean
}

export interface StoryGroup {
  user: User
  stories: Story[]
  hasUnseen: boolean
}

export interface Notification {
  id: number
  user_id: number
  actor_id: number
  type: 'like' | 'comment' | 'follow' | 'dm'
  post_id?: number
  read: boolean
  created_at: string
  actor?: User
}

export interface Conversation {
  id: number
  user1_id: number
  user2_id: number
  last_message_at?: string
  created_at: string
  other_user?: User
  last_message?: Message
  unread_count: number
}

export interface Message {
  id: number
  conversation_id: number
  sender_id: number
  body: string
  read: boolean
  created_at: string
  sender?: User
}

export interface AuthResponse {
  token: string
  user: User
}

export interface PaginatedPosts {
  posts: Post[]
  page: number
}

export interface NotificationsResponse {
  notifications: Notification[]
  unread_count: number
}
