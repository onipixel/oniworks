import '../css/app.css'

import { on, start, navigate } from './router.ts'
import { loadUser, getUser, renderLogin, renderRegister } from './auth.ts'
import { renderFeed } from './feed.ts'
import { renderProfile } from './profile.ts'
import { renderPost } from './post.ts'
import { renderNotifications, initRealtimeNotifications, updateNotifBadge } from './notifications.ts'
import { renderExplore } from './explore.ts'
import { renderMessages, initRealtimeDMs } from './messages.ts'

const appEl = document.getElementById('app')!

// ─── Shell ────────────────────────────────────────────────────────

function renderShell(): { content: HTMLElement } {
  const user = getUser()!
  appEl.innerHTML = `
  <div class="flex min-h-screen bg-gray-950 text-white">

    <!-- Desktop Sidebar -->
    <aside id="sidebar" class="hidden lg:flex flex-col fixed top-0 left-0 h-full w-60 border-r border-gray-800 bg-gray-950 z-30 px-3 py-6">
      <a href="/feed" data-link class="flex items-center gap-2 px-3 mb-8">
        <span class="text-2xl font-black bg-gradient-to-r from-purple-400 to-pink-400 bg-clip-text text-transparent tracking-tight">OniGram</span>
      </a>
      <nav class="flex-1 flex flex-col gap-1">
        ${navItem('/feed', iconHome(), 'Home')}
        ${navItem('/explore', iconSearch(), 'Explore')}
        ${navItem('/messages', iconDM(), 'Messages', 'dm-badge-sidebar')}
        ${navItem('/notifications', iconBell(), 'Notifications', 'notif-badge-sidebar')}
        ${navItem(`/profile/${user.username}`, iconPerson(), 'Profile')}
      </nav>
      <div class="mt-auto space-y-2">
        <button id="create-btn-sidebar" class="flex items-center gap-3 w-full px-3 py-3 rounded-xl hover:bg-gray-800 transition text-sm font-semibold">
          ${iconPlus()} Create
        </button>
        <button id="logout-btn" class="flex items-center gap-3 w-full px-3 py-3 rounded-xl hover:bg-gray-800 transition text-sm text-gray-400">
          ${iconLogout()} Log out
        </button>
      </div>
    </aside>

    <!-- Main content -->
    <main id="content" class="flex-1 lg:ml-60 pb-20 lg:pb-0 min-h-screen"></main>

    <!-- Mobile Bottom Tab Bar -->
    <nav class="lg:hidden fixed bottom-0 left-0 right-0 bg-gray-950 border-t border-gray-800 z-30 flex items-center justify-around px-2 py-2">
      ${tabItem('/feed', iconHome(), 'feed')}
      ${tabItem('/explore', iconSearch(), 'explore')}
      <button id="create-btn-mobile" class="flex flex-col items-center gap-0.5 px-3 py-1">
        <div class="w-9 h-9 rounded-xl bg-purple-600 flex items-center justify-center">${iconPlus()}</div>
      </button>
      ${tabItem('/messages', iconDM(), 'messages', 'dm-badge-mobile')}
      ${tabItem(`/profile/${user.username}`, iconPerson(), 'profile')}
    </nav>
  </div>

  <!-- Create Post Modal (global) -->
  <div id="create-modal" class="hidden fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-md space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="font-semibold text-lg">New Post</h3>
        <button id="create-modal-close" class="text-gray-400 hover:text-white">✕</button>
      </div>
      <input id="create-file" type="file" accept="image/*" class="block w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:bg-purple-700 file:text-white file:border-0 hover:file:bg-purple-600 cursor-pointer" />
      <img id="create-preview" class="hidden rounded-xl w-full object-cover max-h-72" />
      <textarea id="create-caption" placeholder="Write a caption..." class="w-full bg-gray-800 border border-gray-700 rounded-xl px-4 py-3 text-sm resize-none h-24 focus:outline-none focus:border-purple-500 transition"></textarea>
      <div id="create-error" class="hidden text-red-400 text-sm"></div>
      <div class="flex gap-3">
        <button id="create-cancel" class="flex-1 border border-gray-700 rounded-xl py-2.5 text-sm hover:bg-gray-800 transition">Cancel</button>
        <button id="create-submit" class="flex-1 bg-purple-600 hover:bg-purple-700 rounded-xl py-2.5 text-sm font-semibold transition">Share</button>
      </div>
    </div>
  </div>`

  wireShell()

  return { content: appEl.querySelector<HTMLElement>('#content')! }
}

function navItem(href: string, icon: string, label: string, badgeId?: string): string {
  return `
  <a href="${href}" data-link class="nav-item flex items-center gap-3 px-3 py-3 rounded-xl hover:bg-gray-800 transition text-sm font-medium relative" data-route="${href}">
    <span class="relative">
      ${icon}
      ${badgeId ? `<span id="${badgeId}" class="hidden absolute -top-1 -right-1 w-4 h-4 bg-red-500 rounded-full text-[10px] flex items-center justify-center font-bold">!</span>` : ''}
    </span>
    ${label}
  </a>`
}

function tabItem(href: string, icon: string, _name: string, badgeId?: string): string {
  return `
  <a href="${href}" data-link class="tab-item flex flex-col items-center px-3 py-1 relative" data-route="${href}">
    <span class="tab-icon-wrap relative flex items-center justify-center w-10 h-10 rounded-xl transition-colors">
      ${icon}
      ${badgeId ? `<span id="${badgeId}" class="hidden absolute top-1 right-1 w-3 h-3 bg-red-500 rounded-full text-[8px] flex items-center justify-center font-bold">!</span>` : ''}
    </span>
  </a>`
}

function wireShell() {
  // Create modal
  const modal = appEl.querySelector<HTMLDivElement>('#create-modal')!
  appEl.querySelector('#create-btn-sidebar')?.addEventListener('click', () => modal.classList.remove('hidden'))
  appEl.querySelector('#create-btn-mobile')?.addEventListener('click', () => modal.classList.remove('hidden'))
  appEl.querySelector('#create-modal-close')?.addEventListener('click', closeModal)
  appEl.querySelector('#create-cancel')?.addEventListener('click', closeModal)

  appEl.querySelector('#create-file')!.addEventListener('change', (e) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    const preview = appEl.querySelector<HTMLImageElement>('#create-preview')!
    preview.src = URL.createObjectURL(file)
    preview.classList.remove('hidden')
  })

  appEl.querySelector('#create-submit')!.addEventListener('click', async () => {
    const fileEl = appEl.querySelector<HTMLInputElement>('#create-file')!
    const captionEl = appEl.querySelector<HTMLTextAreaElement>('#create-caption')!
    const errEl = appEl.querySelector<HTMLDivElement>('#create-error')!
    const file = fileEl.files?.[0]
    if (!file) { errEl.textContent = 'Please select an image'; errEl.classList.remove('hidden'); return }

    const { posts } = await import('./api.ts')
    const form = new FormData()
    form.append('image', file)
    form.append('caption', captionEl.value)

    try {
      const btn = appEl.querySelector<HTMLButtonElement>('#create-submit')!
      btn.disabled = true
      btn.textContent = 'Sharing...'
      await posts.create(form)
      closeModal()
      navigate('/feed')
    } catch (e: any) {
      errEl.textContent = e?.message ?? 'Upload failed'
      errEl.classList.remove('hidden')
      const btn = appEl.querySelector<HTMLButtonElement>('#create-submit')!
      btn.disabled = false
      btn.textContent = 'Share'
    }
  })

  // Logout
  appEl.querySelector('#logout-btn')?.addEventListener('click', () => {
    localStorage.removeItem('og_token')
    location.href = '/login'
  })

  // Active nav highlighting
  highlightNav(location.pathname)
}

function closeModal() {
  const modal = appEl.querySelector<HTMLDivElement>('#create-modal')!
  modal.classList.add('hidden')
  ;(appEl.querySelector<HTMLInputElement>('#create-file')!).value = ''
  ;(appEl.querySelector<HTMLImageElement>('#create-preview')!).classList.add('hidden')
  ;(appEl.querySelector<HTMLTextAreaElement>('#create-caption')!).value = ''
  ;(appEl.querySelector<HTMLDivElement>('#create-error')!).classList.add('hidden')
  const btn = appEl.querySelector<HTMLButtonElement>('#create-submit')!
  btn.disabled = false
  btn.textContent = 'Share'
}

export function highlightNav(path: string) {
  appEl.querySelectorAll<HTMLElement>('.nav-item').forEach(el => {
    const route = el.dataset.route ?? ''
    const isActive = path === route || (route !== '/' && path.startsWith(route.replace(':username', '')))
    el.classList.toggle('text-white', isActive)
    el.classList.toggle('bg-gray-800', isActive)
    el.classList.toggle('text-gray-400', !isActive)
  })
  appEl.querySelectorAll<HTMLElement>('.tab-item').forEach(el => {
    const route = el.dataset.route ?? ''
    const isActive = path === route || (route !== '/' && path.startsWith(route.replace(':username', '')))
    el.classList.toggle('text-white', isActive)
    el.classList.toggle('text-gray-400', !isActive)
    const wrap = el.querySelector<HTMLElement>('.tab-icon-wrap')
    if (wrap) {
      wrap.style.background = isActive ? 'rgba(139,92,246,0.18)' : ''
    }
  })
}

export function showDMBadge() {
  appEl.querySelector('#dm-badge-sidebar')?.classList.remove('hidden')
  appEl.querySelector('#dm-badge-mobile')?.classList.remove('hidden')
}
export function hideDMBadge() {
  appEl.querySelector('#dm-badge-sidebar')?.classList.add('hidden')
  appEl.querySelector('#dm-badge-mobile')?.classList.add('hidden')
}

// ─── Icons ────────────────────────────────────────────────────────

function iconHome() {
  return `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>`
}
function iconSearch() {
  return `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>`
}
function iconBell() {
  return `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>`
}
function iconDM() {
  return `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>`
}
function iconPerson() {
  return `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>`
}
function iconPlus() {
  return `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 4v16m8-8H4"/></svg>`
}
function iconLogout() {
  return `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"/></svg>`
}

// ─── Route definitions ────────────────────────────────────────────

function withShell(renderFn: (content: HTMLElement) => void) {
  const { content } = renderShell()
  renderFn(content)
  highlightNav(location.pathname)
}

on('/login', () => renderLogin(appEl))
on('/register', () => renderRegister(appEl))

on('/feed', () => {
  if (!requireAuth()) return
  withShell(content => renderFeed(content))
})

on('/explore', () => {
  if (!requireAuth()) return
  withShell(content => renderExplore(content))
})

on('/messages', () => {
  if (!requireAuth()) return
  withShell(content => renderMessages(content, null))
})

on('/messages/:username', (p) => {
  if (!requireAuth()) return
  withShell(content => renderMessages(content, p.username))
})

on('/profile/:username', (params) => {
  if (!requireAuth()) return
  withShell(content => renderProfile(content, params))
})

on('/post/:id', (params) => {
  if (!requireAuth()) return
  withShell(content => renderPost(content, params))
})

on('/notifications', () => {
  if (!requireAuth()) return
  withShell(content => renderNotifications(content))
})

on('/', () => {
  const user = getUser()
  navigate(user ? '/feed' : '/login')
})

// ─── Bootstrap ────────────────────────────────────────────────────

async function bootstrap() {
  await loadUser()
  const user = getUser()

  if (user) {
    const tok = localStorage.getItem('og_token') ?? ''
    initRealtimeNotifications(user.id, tok, {
      onBadge: () => updateNotifBadge(appEl, true),
    })
    initRealtimeDMs(user.id, tok, {
      onMessage: () => showDMBadge(),
    })
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
