import '../css/app.css'

import { on, start, navigate } from './router.ts'
import { loadUser, getUser, renderLogin, renderRegister } from './auth.ts'
import { renderFeed } from './feed.ts'
import { renderProfile } from './profile.ts'
import { renderPost } from './post.ts'
import { renderNotifications, initRealtimeNotifications } from './notifications.ts'

const root = document.getElementById('app')!

// ─── Route definitions ────────────────────────────────────────────

on('/login', () => renderLogin(root))
on('/register', () => renderRegister(root))

on('/feed', () => {
  if (!requireAuth()) return
  renderFeed(root)
})

on('/profile/:username', (params) => {
  if (!requireAuth()) return
  renderProfile(root, params)
})

on('/post/:id', (params) => {
  if (!requireAuth()) return
  renderPost(root, params)
})

on('/notifications', () => {
  if (!requireAuth()) return
  renderNotifications(root)
})

// Default redirect
on('/', () => {
  const user = getUser()
  navigate(user ? '/feed' : '/login')
})

// ─── Bootstrap ────────────────────────────────────────────────────

async function bootstrap() {
  await loadUser()
  const user = getUser()

  // Connect realtime notifications if authenticated
  if (user) {
    const token = localStorage.getItem('og_token') ?? ''
    initRealtimeNotifications(user.id, token)
  }

  start()
}

function requireAuth(): boolean {
  if (!getUser()) {
    navigate('/login')
    return false
  }
  return true
}

bootstrap()
